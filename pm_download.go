package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func downloadPodcastEpisodes(client *ABSClient, item PodcastItem, count int, oldest, dryRun, noWait, fill, countGiven, checkNew, forceNewOnly bool, keep *int, verbose bool, silent bool) int {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}
	feedURL := item.Media.Metadata.FeedURL
	itemID := item.ID

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

			if forceNewOnly {
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
							if !isDownloaded(ep) {
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
						if !isDownloaded(ep) {
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
				if gapTerminated && !silent {
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

	if len(episodesToDownload) > 0 {
		if !silent {
			fmt.Print("\r\x1b[K")
			fmt.Printf("=== Podcast: %s ===\n", podcastTitle)
		}
		sort.Slice(episodesToDownload, func(i, j int) bool {
			return getPubMS(episodesToDownload[i]) < getPubMS(episodesToDownload[j])
		})
		if !oldest {
			for i, j := 0, len(episodesToDownload)-1; i < j; i, j = i+1, j-1 {
				episodesToDownload[i], episodesToDownload[j] = episodesToDownload[j], episodesToDownload[i]
			}
		}

		directionStr := "latest -> oldest"
		if oldest {
			directionStr = "oldest -> newest"
		}
		if !silent {
			fmt.Printf("Found %s (%s).\n", strings.Join(reasons, " and "), directionStr)

			fmt.Printf("\n=== Selected %d Episode(s) for Download ===\n", len(episodesToDownload))
			for idx, ep := range episodesToDownload {
				pub := ep.PubDate
				if pub == "" {
					pub = fmt.Sprintf("%d", ep.PublishedAt)
				}
				encURL := ""
				if ep.Enclosure != nil {
					encURL = ep.Enclosure.URL
				}
				fmt.Printf("  %d. %s\n", idx+1, ep.Title)
				fmt.Printf("     Published: %s\n", pub)
				if verbose {
					fmt.Printf("     URL: %s\n", encURL)
				}
				fmt.Println()
			}
		}

		if !dryRun {
			if !silent {
				fmt.Printf("Queueing download request in Audiobookshelf for %d episode(s)...\n", len(episodesToDownload))
			}
			if err := client.DownloadEpisodes(itemID, episodesToDownload); err != nil {
				printError(fmt.Sprintf("Failed to queue episode download: %v", err))
			} else {
				if !silent {
					fmt.Println("Download request successfully sent to Audiobookshelf!")
				}

				if !noWait {
					if !silent {
						fmt.Println("\nWaiting for Audiobookshelf to complete episode download(s)...")
					}
					time.Sleep(2 * time.Second)
					startTime := time.Now()

					for {
						activeDls, err := client.ActiveDownloads(itemID)
						if err == nil && len(activeDls) == 0 {
							updatedItem, _ := client.GetItem(itemID)
							newCount := 0
							if updatedItem != nil {
								newCount = len(updatedItem.Media.Episodes)
							}
							if !silent {
								fmt.Printf("\nDownload completed! Podcast now has %d downloaded episodes.\n", newCount)
							}
							break
						} else if len(activeDls) > 0 {
							if !silent {
								dl := activeDls[0]
								title := dl.EpisodeDisplayTitle
								if title == "" {
									title = "Episode"
								}
								fmt.Printf("    Downloading: %s\r", title)
								os.Stdout.Sync()
							}
						}

						time.Sleep(3 * time.Second)
						if time.Since(startTime) > 300*time.Second {
							if !silent {
								fmt.Println("\nTimeout waiting for download to finish. Download continues in background.")
							}
							break
						}
					}
				}
			}
		} else {
			if !silent {
				fmt.Println("Dry run mode enabled. Skipping actual download request.")
			}
		}
	} else if !forceNewOnly && !silent {
		fmt.Printf("No new episodes to download for %s.\n", podcastTitle)
	}

	if keep != nil {
		applyKeepPolicy(client, itemID, podcastTitle, *keep, dryRun, verbose, silent)
	}

	return len(episodesToDownload)
}
