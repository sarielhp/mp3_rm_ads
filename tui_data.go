package main

import (
	"encoding/json"
	"fmt"
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
		if !entry.IsDir() || entry.Name() == ".work" || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		podDir := filepath.Join(podcastsDir, entry.Name())
		pod := tuiPodcast{
			name: entry.Name(),
			dir:  podDir,
		}

		mp3Files, err := filepath.Glob(filepath.Join(podDir, "*.mp3"))
		if err != nil {
			continue
		}

		var episodes []tuiEpisode
		for _, mp3 := range mp3Files {
			base := strings.TrimSuffix(mp3, ".mp3")
			hasCut := false
			if _, err := os.Stat(base + ".cuts.json"); err == nil {
				hasCut = true
			}
			var fSize int64
			var modTime time.Time
			if fi, err := os.Stat(mp3); err == nil {
				fSize = fi.Size()
				modTime = fi.ModTime()
			}
			episodes = append(episodes, tuiEpisode{
				filename:      filepath.Base(mp3),
				path:          mp3,
				hasAdsRemoved: hasCut,
				fileSize:      fSize,
				modTime:       modTime,
			})
		}

		sort.Slice(episodes, func(i, j int) bool {
			return episodes[i].modTime.After(episodes[j].modTime)
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

func loadTUIPodcastsABS(podcastsDir string, cfg Config) ([]tuiPodcast, error) {
	podcasts, err := loadTUIPodcasts(podcastsDir)
	if err != nil {
		return nil, err
	}

	if cfg.AudiobookshelfURL == "" || cfg.AudiobookshelfUser == "" || cfg.AudiobookshelfPass == "" {
		return podcasts, nil
	}

	token, err := absLogin(cfg)
	if err != nil {
		return podcasts, nil
	}

	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")

	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		return podcasts, nil
	}

	var allItems []absItem
	for _, lib := range libsResp.Libraries {
		if lib.MediaType != "podcast" {
			continue
		}
		var itemsResp absItemsResp
		endpoint := fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID)
		if err := absGet(baseURL, token, endpoint, &itemsResp); err == nil {
			allItems = append(allItems, itemsResp.Results...)
		}
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

		var fullItem absItem
		if err := absGet(baseURL, token, "/api/items/"+itemSummary.ID, &fullItem); err != nil {
			continue
		}
		pod.absData = &fullItem

		episodeMap := make(map[string]*absEpisode)
		for epIdx := range fullItem.Media.Episodes {
			ep := &fullItem.Media.Episodes[epIdx]
			if ep.AudioFile != nil {
				if ep.AudioFile.Metadata.Filename != "" {
					episodeMap[ep.AudioFile.Metadata.Filename] = ep
				}
				if ep.AudioFile.Metadata.RelPath != "" {
					episodeMap[filepath.Base(ep.AudioFile.Metadata.RelPath)] = ep
				}
			}
			if ep.Title != "" {
				episodeMap[ep.Title] = ep
				episodeMap[ep.Title+".mp3"] = ep
			}
		}

		for j := range pod.episodes {
			ep := &pod.episodes[j]
			if absEp, exists := episodeMap[ep.filename]; exists {
				ep.absData = absEp
			} else if absEp, exists := episodeMap[strings.TrimSuffix(ep.filename, ".mp3")]; exists {
				ep.absData = absEp
			}
		}
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
		var filtered []string
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e), ".mp3") {
				filtered = append(filtered, e)
			}
		}
		q[pod.dir] = filtered
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
