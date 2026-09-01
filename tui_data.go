package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/backend"
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

func absDownloadCover(baseURL, token, itemID, destPath string) error {
	client := backend.NewAudiobookshelf(backend.Config{Host: baseURL, Token: token})
	return client.DownloadCover(itemID, destPath)
}

func loadTUIPodcastsABS(podcastsDir string, cfg Config) ([]tuiPodcast, error) {
	podcasts, err := loadTUIPodcasts(podcastsDir)
	if err != nil {
		return nil, err
	}

	if cfg.AudiobookshelfURL == "" || cfg.AudiobookshelfUser == "" || cfg.AudiobookshelfPass == "" {
		return podcasts, nil
	}

	b, err := getBackend(cfg, true)
	if err != nil {
		return podcasts, nil
	}

	allItems, err := b.Podcasts()
	if err != nil {
		return podcasts, nil
	}

	itemByRel := make(map[string]absItem)
	for _, item := range allItems {
		itemByRel[item.RelPath] = item
		cleanRel := filepath.Base(item.RelPath)
		itemByRel[cleanRel] = item
		if item.Media.Metadata.Title != "" {
			itemByRel[item.Media.Metadata.Title] = item
		}
	}

	for i := range podcasts {
		pod := &podcasts[i]
		itemSummary, ok := itemByRel[pod.name]
		if !ok {
			itemSummary, ok = itemByRel[filepath.Base(pod.dir)]
		}
		if !ok {
			continue
		}

		fullItem, err := b.GetPodcast(itemSummary.ID)
		if err != nil || fullItem == nil {
			continue
		}
		pod.absData = fullItem
		if fullItem.Media.Metadata.Author != "" {
			pod.author = fullItem.Media.Metadata.Author
		}
		if fullItem.Media.Metadata.Description != "" {
			pod.description = fullItem.Media.Metadata.Description
		}
		if fullItem.Media.Metadata.FeedURL != "" {
			pod.feedURL = fullItem.Media.Metadata.FeedURL
		}

		cDir := cacheDirForPodcast(pod.dir)
		coverDest := filepath.Join(cDir, "cover.jpg")
		_ = b.DownloadCover(fullItem.ID, coverDest)

		quarantined := quarantineAbandonedDuplicates(pod.dir, fullItem.Media.Episodes)
		if len(quarantined) > 0 {
			printQuarantinedSummary(quarantined, pod.name)
		}

		episodeMap := make(map[string]*absEpisode)
		for epIdx := range fullItem.Media.Episodes {
			ep := &fullItem.Media.Episodes[epIdx]
			if ep.AudioFile != nil {
				if ep.AudioFile.Metadata.Filename != "" {
					episodeMap[ep.AudioFile.Metadata.Filename] = ep
					episodeMap[normalizeEpisodeTitle(ep.AudioFile.Metadata.Filename)] = ep
				}
				if ep.AudioFile.Metadata.RelPath != "" {
					cleanRel := filepath.Base(ep.AudioFile.Metadata.RelPath)
					episodeMap[cleanRel] = ep
					episodeMap[normalizeEpisodeTitle(cleanRel)] = ep
				}
			}
			if ep.Title != "" {
				episodeMap[ep.Title] = ep
				episodeMap[ep.Title+".mp3"] = ep
				episodeMap[normalizeEpisodeTitle(ep.Title)] = ep
			}
		}

		for j := range pod.episodes {
			ep := &pod.episodes[j]
			var matchedEp *absEpisode
			if absEp, exists := episodeMap[ep.filename]; exists {
				matchedEp = absEp
			} else if absEp, exists := episodeMap[strings.TrimSuffix(ep.filename, ".mp3")]; exists {
				matchedEp = absEp
			} else if absEp, exists := episodeMap[normalizeEpisodeTitle(ep.filename)]; exists {
				matchedEp = absEp
			}
			if matchedEp != nil {
				ep.absData = matchedEp
				if matchedEp.Title != "" {
					ep.title = matchedEp.Title
				}
				if pub := parseABSEpisodePublishedAt(matchedEp); pub > 0 {
					ep.publishedAt = pub
				}
				if matchedEp.Duration > 0 {
					ep.duration = matchedEp.Duration
				}
				if matchedEp.Season != "" {
					ep.season = matchedEp.Season
				}
				if matchedEp.Episode != "" {
					ep.episode = matchedEp.Episode
				}
			}
		}

		sort.Slice(pod.episodes, func(x, y int) bool {
			return pod.episodes[x].displayDate().After(pod.episodes[y].displayDate())
		})

		savePodcastToCache(pod)
	}

	return podcasts, nil
}

func loadAllQueues(pods []tuiPodcast) map[string][]string {
	q := make(map[string][]string)
	for _, pod := range pods {
		data, err := os.ReadFile(filepath.Join(pod.dir, "queue.json"))
		if err != nil {
			q[pod.dir] = nil
			continue
		}
		var entries []string
		if err := json.Unmarshal(data, &entries); err != nil {
			q[pod.dir] = nil
			continue
		}

		adFreeMap := make(map[string]bool)
		for _, ep := range pod.episodes {
			if ep.hasAdsRemoved {
				adFreeMap[ep.filename] = true
			}
		}

		var filtered []string
		needsResave := false
		for _, e := range entries {
			if !strings.HasSuffix(strings.ToLower(e), ".mp3") {
				needsResave = true
				continue
			}
			if adFreeMap[e] {
				needsResave = true
				continue
			}
			filtered = append(filtered, e)
		}
		q[pod.dir] = filtered
		if needsResave {
			saveQueue(pod.dir, filtered)
		}
	}
	return q
}

func saveQueue(dir string, entries []string) {
	if entries == nil {
		entries = []string{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(dir, "queue.json"), data, 0644)
}
