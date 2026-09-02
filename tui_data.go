package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func loadTUIPodcasts(podcastsDir string) ([]tuiPodcast, error) {
	entries, err := os.ReadDir(podcastsDir)
	if err != nil {
		return nil, err
	}

	var podcasts []tuiPodcast
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".work" || strings.HasPrefix(entry.Name(), ".") || strings.HasSuffix(entry.Name(), "-1") {
			continue
		}
		podDir := filepath.Join(podcastsDir, entry.Name())
		if pod := loadSingleTUIPodcast(podDir, entry.Name()); pod != nil {
			podcasts = append(podcasts, *pod)
		}
	}

	sort.Slice(podcasts, func(i, j int) bool {
		return strings.ToLower(podcasts[i].name) < strings.ToLower(podcasts[j].name)
	})

	return podcasts, nil
}

func loadSingleTUIPodcast(podDir, name string) *tuiPodcast {
	_ = ensureABSIgnore(podDir)
	pod := tuiPodcast{
		name:   name,
		dir:    podDir,
		config: loadPodcastConfig(podDir),
	}

	cachedIdx, _ := loadPodcastCache(podDir)
	cachedByPath := make(map[string]CachedEpisodeSummary)
	cachedByName := make(map[string]CachedEpisodeSummary)
	if cachedIdx != nil {
		pod.author = cachedIdx.Author
		pod.description = cachedIdx.Description
		pod.feedURL = cachedIdx.FeedURL
		pod.coverPath = cachedIdx.CoverPath
		for _, ce := range cachedIdx.Episodes {
			if ce.Path != "" {
				cachedByPath[ce.Path] = ce
			}
			if ce.Filename != "" {
				cachedByName[ce.Filename] = ce
			}
		}
	}

	mp3Files, err := filepath.Glob(filepath.Join(podDir, "*.mp3"))
	if err != nil || len(mp3Files) == 0 {
		return nil
	}

	var episodes []tuiEpisode
	for _, mp3 := range mp3Files {
		episodes = append(episodes, loadSingleTUIEpisode(mp3, cachedByPath, cachedByName))
	}

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].displayDate().After(episodes[j].displayDate())
	})

	pod.episodes = episodes
	return &pod
}

func loadSingleTUIEpisode(mp3 string, cachedByPath, cachedByName map[string]CachedEpisodeSummary) tuiEpisode {
	absPath, _ := filepath.Abs(mp3)
	base := strings.TrimSuffix(mp3, ".mp3")
	hasCut := fileExists(base + ".cuts.json")
	hasTx := fileExists(base+".transcript.json") || fileExists(base+".transcript.txt")
	var fSize int64
	var modTime time.Time
	if fi, err := os.Stat(mp3); err == nil {
		fSize = fi.Size()
		modTime = fi.ModTime()
	}
	fn := filepath.Base(mp3)
	ep := tuiEpisode{
		filename:      fn,
		path:          absPath,
		hasAdsRemoved: hasCut,
		hasTranscript: hasTx,
		fileSize:      fSize,
		modTime:       modTime,
	}
	if ce, ok := cachedByPath[absPath]; ok {
		applyCachedSummaryToEpisode(&ep, ce)
	} else if ce, ok := cachedByName[fn]; ok {
		applyCachedSummaryToEpisode(&ep, ce)
	}
	return ep
}

func applyCachedSummaryToEpisode(ep *tuiEpisode, ce CachedEpisodeSummary) {
	ep.title = ce.Title
	ep.publishedAt = ce.PublishedAt
	ep.duration = ce.Duration
	ep.season = ce.Season
	ep.episode = ce.Episode
	if ce.HasTranscript {
		ep.hasTranscript = true
	}
}

func savePodcastToCache(pod *tuiPodcast) {
	if pod == nil {
		return
	}
	absPodDir, _ := filepath.Abs(pod.dir)
	var summaries []CachedEpisodeSummary
	for _, ep := range pod.episodes {
		sum := buildCachedEpisodeSummary(ep, pod.dir, pod.absData)
		summaries = append(summaries, sum)
	}

	absItemID := ""
	author := pod.author
	description := pod.description
	feedURL := pod.feedURL
	coverPath := pod.coverPath
	if pod.absData != nil {
		absItemID = pod.absData.ID
		if pod.absData.Media.Metadata.Author != "" {
			author = pod.absData.Media.Metadata.Author
		}
		if pod.absData.Media.Metadata.Description != "" {
			description = pod.absData.Media.Metadata.Description
		}
		if pod.absData.Media.Metadata.FeedURL != "" {
			feedURL = pod.absData.Media.Metadata.FeedURL
		}
	}
	index := CachedPodcastIndex{
		PodcastName: pod.name,
		PodcastDir:  absPodDir,
		ABSItemID:   absItemID,
		Author:      author,
		Description: description,
		FeedURL:     feedURL,
		CoverPath:   coverPath,
		UpdatedAt:   time.Now(),
		Episodes:    summaries,
	}
	_ = savePodcastCache(pod.dir, &index)
}

func buildCachedEpisodeSummary(ep tuiEpisode, podDir string, podABSData *PodcastItem) CachedEpisodeSummary {
	absPath, _ := filepath.Abs(ep.path)
	title := ep.displayTitle()
	pubAt := ep.publishedAt
	dur := ep.duration
	season := ep.season
	episode := ep.episode
	if ep.absData != nil {
		if ep.absData.Title != "" {
			title = ep.absData.Title
		}
		if pub := parseABSEpisodePublishedAt(ep.absData); pub > 0 {
			pubAt = pub
		}
		if ep.absData.Duration > 0 {
			dur = ep.absData.Duration
		}
		if ep.absData.Season != "" {
			season = ep.absData.Season
		}
		if ep.absData.Episode != "" {
			episode = ep.absData.Episode
		}

		det := CachedEpisodeDetails{
			Path:        absPath,
			Filename:    ep.filename,
			Title:       title,
			Description: ep.absData.Description,
			Subtitle:    ep.absData.Subtitle,
			EpisodeType: ep.absData.EpisodeType,
			RawABS:      ep.absData,
		}
		if podABSData != nil {
			det.Author = podABSData.Media.Metadata.Author
			det.FeedURL = podABSData.Media.Metadata.FeedURL
		}
		_ = saveEpisodeDetails(podDir, ep.filename, &det)
	}

	return CachedEpisodeSummary{
		Path:          absPath,
		Filename:      ep.filename,
		Title:         title,
		PublishedAt:   pubAt,
		Duration:      dur,
		FileSize:      ep.fileSize,
		Season:        season,
		Episode:       episode,
		HasAdsRemoved: ep.hasAdsRemoved,
		HasTranscript: ep.hasTranscript,
	}
}
