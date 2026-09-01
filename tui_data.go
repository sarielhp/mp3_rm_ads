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
		_ = ensureABSIgnore(podDir)
		pod := tuiPodcast{
			name:   entry.Name(),
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
		if err != nil {
			continue
		}

		var episodes []tuiEpisode
		for _, mp3 := range mp3Files {
			absPath, _ := filepath.Abs(mp3)
			base := strings.TrimSuffix(mp3, ".mp3")
			hasCut := false
			if _, err := os.Stat(base + ".cuts.json"); err == nil {
				hasCut = true
			}
			hasTx := false
			if _, err := os.Stat(base + ".transcript.json"); err == nil {
				hasTx = true
			} else if _, err := os.Stat(base + ".transcript.txt"); err == nil {
				hasTx = true
			}
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
				ep.title = ce.Title
				ep.publishedAt = ce.PublishedAt
				ep.duration = ce.Duration
				ep.season = ce.Season
				ep.episode = ce.Episode
				if ce.HasTranscript {
					ep.hasTranscript = true
				}
			} else if ce, ok := cachedByName[fn]; ok {
				ep.title = ce.Title
				ep.publishedAt = ce.PublishedAt
				ep.duration = ce.Duration
				ep.season = ce.Season
				ep.episode = ce.Episode
				if ce.HasTranscript {
					ep.hasTranscript = true
				}
			}
			episodes = append(episodes, ep)
		}

		sort.Slice(episodes, func(i, j int) bool {
			return episodes[i].displayDate().After(episodes[j].displayDate())
		})

		pod.episodes = episodes
		if len(episodes) > 0 {
			podcasts = append(podcasts, pod)
		}
	}

	sort.Slice(podcasts, func(i, j int) bool {
		return strings.ToLower(podcasts[i].name) < strings.ToLower(podcasts[j].name)
	})

	return podcasts, nil
}

func savePodcastToCache(pod *tuiPodcast) {
	if pod == nil {
		return
	}
	absPodDir, _ := filepath.Abs(pod.dir)
	var summaries []CachedEpisodeSummary
	for _, ep := range pod.episodes {
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
			if pod.absData != nil {
				det.Author = pod.absData.Media.Metadata.Author
				det.FeedURL = pod.absData.Media.Metadata.FeedURL
			}
			_ = saveEpisodeDetails(pod.dir, ep.filename, &det)
		}

		summaries = append(summaries, CachedEpisodeSummary{
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
		})
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
