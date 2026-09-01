package main

import (
	"fmt"
	"sort"
	"strings"
)

func downloadPodcastEpisodes(client *ABSClient, item PodcastItem, count int, oldest, dryRun, noWait, fill, countGiven, checkNew, forceNewOnly, downloadAll bool, keep *int, verbose bool, quiet bool) int {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}
	feedURL := item.Media.Metadata.FeedURL
	itemID := item.ID

	if downloadAll {
		_ = resetPodcastDateCheck(client, loadConfig().AudiobookshelfDBPath, itemID, podcastTitle)
	}

	var episodesToDownload []FeedEpisode
	var reasons []string

	if feedURL != "" {
		feedEpisodes, err := client.PodcastFeedEpisodes(feedURL)
		if err != nil {
			printError(fmt.Sprintf("Failed to parse episode catalog for %s: %v", podcastTitle, err))
		} else {
			downloadedEpisodes := item.Media.Episodes
			downloadedURLs := make(map[string]bool)
			downloadedGUIDs := make(map[string]bool)
			downloadedTitles := make(map[string]bool)

			for _, ep := range downloadedEpisodes {
				if ep.EnclosureURL != "" {
					downloadedURLs[ep.EnclosureURL] = true
				}
				if ep.GUID != "" {
					downloadedGUIDs[ep.GUID] = true
				}
				t := strings.ToLower(strings.TrimSpace(ep.Title))
				if t != "" {
					downloadedTitles[t] = true
				}
			}

			activeDls, _ := client.ActiveDownloads(itemID)
			queuedTitles := make(map[string]bool)
			queuedURLs := make(map[string]bool)
			queuedGUIDs := make(map[string]bool)
			for _, ad := range activeDls {
				for _, t := range []string{ad.EpisodeDisplayTitle, ad.DisplayTitle, ad.Title, ad.Episode.Title} {
					t = strings.ToLower(strings.TrimSpace(t))
					if t != "" {
						queuedTitles[t] = true
					}
				}
				if ad.URL != "" {
					queuedURLs[ad.URL] = true
				}
				if ad.Episode.EnclosureURL != "" {
					queuedURLs[ad.Episode.EnclosureURL] = true
				}
				if ad.Episode.GUID != "" {
					queuedGUIDs[ad.Episode.GUID] = true
				}
			}

			isDownloaded := func(ep FeedEpisode) bool {
				encURL := ""
				if ep.Enclosure != nil {
					encURL = ep.Enclosure.URL
				}
				guid := ep.GUID
				title := strings.ToLower(strings.TrimSpace(ep.Title))

				return (encURL != "" && (downloadedURLs[encURL] || queuedURLs[encURL])) ||
					(guid != "" && (downloadedGUIDs[guid] || queuedGUIDs[guid])) ||
					(title != "" && (downloadedTitles[title] || queuedTitles[title]))
			}

			sortedCatalog := make([]FeedEpisode, len(feedEpisodes))
			copy(sortedCatalog, feedEpisodes)
			sort.Slice(sortedCatalog, func(i, j int) bool {
				return getPubMS(sortedCatalog[i]) < getPubMS(sortedCatalog[j])
			})

			downloadedIndices := []int{}
			for idx, ep := range sortedCatalog {
				if isDownloaded(ep) {
					downloadedIndices = append(downloadedIndices, idx)
				}
			}

			podDir := findPodcastDirForItem(item, "")
			podCfg := defaultPodcastConfig()
			if podDir != "" {
				podCfg = loadPodcastConfig(podDir)
			}

			if downloadAll {
				episodesToDownload, reasons = selectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, DownloadPolicyAll, 0, oldest)
				if countGiven && count > 0 && len(episodesToDownload) > count {
					episodesToDownload = episodesToDownload[:count]
				}
			} else if !fill && !countGiven {
				episodesToDownload, reasons = selectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, podCfg.DownloadPolicy, podCfg.DownloadK, oldest)
			} else if forceNewOnly {
				if len(downloadedIndices) > 0 {
					maxIdx := -1
					for _, idx := range downloadedIndices {
						if idx > maxIdx {
							maxIdx = idx
						}
					}
					var newEpisodes []FeedEpisode
					if maxIdx+1 < len(sortedCatalog) {
						for _, ep := range sortedCatalog[maxIdx+1:] {
							hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
							if hasEnc && !isDownloaded(ep) {
								newEpisodes = append(newEpisodes, ep)
							}
						}
					}
					if len(newEpisodes) > 0 {
						reasons = append(reasons, fmt.Sprintf("%d new episode(s)", len(newEpisodes)))
						episodesToDownload = newEpisodes
					}
				} else {
					var undownloaded []FeedEpisode
					for _, ep := range sortedCatalog {
						hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
						if hasEnc && !isDownloaded(ep) {
							undownloaded = append(undownloaded, ep)
						}
					}
					if len(undownloaded) > 0 {
						latestEp := undownloaded[len(undownloaded)-1]
						if oldest {
							latestEp = undownloaded[0]
						}
						reasons = append(reasons, "1 latest episode")
						episodesToDownload = []FeedEpisode{latestEp}
					}
				}
				if countGiven {
					if len(episodesToDownload) > count {
						episodesToDownload = episodesToDownload[:count]
					}
				}
			} else if fill {
				searchCatalog := make([]FeedEpisode, len(sortedCatalog))
				copy(searchCatalog, sortedCatalog)
				if !oldest {
					for i, j := 0, len(searchCatalog)-1; i < j; i, j = i+1, j-1 {
						searchCatalog[i], searchCatalog[j] = searchCatalog[j], searchCatalog[i]
					}
				}

				if checkNew && len(downloadedIndices) > 0 {
					maxIdx := -1
					for _, idx := range downloadedIndices {
						if idx > maxIdx {
							maxIdx = idx
						}
					}
					if maxIdx+1 < len(sortedCatalog) {
						var newEpisodes []FeedEpisode
						for _, ep := range sortedCatalog[maxIdx+1:] {
							if !isDownloaded(ep) {
								newEpisodes = append(newEpisodes, ep)
							}
						}
						if len(newEpisodes) > 0 {
							reasons = append(reasons, fmt.Sprintf("%d new episode(s)", len(newEpisodes)))
							episodesToDownload = append(episodesToDownload, newEpisodes...)
						}
					}
				}

				consecutiveMissing := 0
				gapTerminated := false

				inToDownload := func(target FeedEpisode) bool {
					for _, ep := range episodesToDownload {
						if (ep.GUID != "" && target.GUID != "" && ep.GUID == target.GUID) ||
							(ep.Enclosure != nil && target.Enclosure != nil && ep.Enclosure.URL == target.Enclosure.URL) {
							return true
						}
					}
					return false
				}

				for _, ep := range searchCatalog {
					if isDownloaded(ep) {
						consecutiveMissing = 0
					} else {
						consecutiveMissing++
						if consecutiveMissing > 10 {
							gapTerminated = true
							break
						}
						if !inToDownload(ep) {
							episodesToDownload = append(episodesToDownload, ep)
						}
					}
				}

				if len(episodesToDownload) > 0 && len(reasons) == 0 {
					reasons = append(reasons, fmt.Sprintf("%d gap/fill episode(s)", len(episodesToDownload)))
				}
				if gapTerminated && !quiet {
					fmt.Printf("Search for %s terminated: gap larger than 10 undownloaded episodes encountered.\n", podcastTitle)
				}
			} else {
				if checkNew && len(downloadedIndices) > 0 {
					maxIdx := -1
					for _, idx := range downloadedIndices {
						if idx > maxIdx {
							maxIdx = idx
						}
					}
					if maxIdx+1 < len(sortedCatalog) {
						var newEpisodes []FeedEpisode
						for _, ep := range sortedCatalog[maxIdx+1:] {
							if !isDownloaded(ep) {
								newEpisodes = append(newEpisodes, ep)
							}
						}
						if len(newEpisodes) > 0 {
							reasons = append(reasons, fmt.Sprintf("%d new episode(s)", len(newEpisodes)))
							episodesToDownload = append(episodesToDownload, newEpisodes...)
						}
					}
				}

				if len(episodesToDownload) == 0 {
					var undownloaded []FeedEpisode
					for _, ep := range sortedCatalog {
						if !isDownloaded(ep) {
							undownloaded = append(undownloaded, ep)
						}
					}
					if len(undownloaded) > 0 {
						if !oldest {
							for i, j := 0, len(undownloaded)-1; i < j; i, j = i+1, j-1 {
								undownloaded[i], undownloaded[j] = undownloaded[j], undownloaded[i]
							}
						}
						if len(undownloaded) > count {
							undownloaded = undownloaded[:count]
						}
						episodesToDownload = undownloaded
						reasons = append(reasons, fmt.Sprintf("%d undownloaded episode(s)", len(episodesToDownload)))
					}
				} else if countGiven {
					if len(episodesToDownload) > count {
						episodesToDownload = episodesToDownload[:count]
					}
				}
			}

			if countGiven && fill {
				if len(episodesToDownload) > count {
					episodesToDownload = episodesToDownload[:count]
				}
			}
		}
	} else {
		printError(fmt.Sprintf("Podcast %s has no RSS feed URL configured.", podcastTitle))
	}

	return executeEpisodeDownloads(client, item, episodesToDownload, reasons, podcastTitle, itemID, noWait, dryRun, oldest, forceNewOnly, verbose, keep, quiet)
}
