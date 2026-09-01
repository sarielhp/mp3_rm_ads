package main

import (
	"github.com/sariel/abs/pkg/backend"
	"path/filepath"
	"sort"
	"strings"
)

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
