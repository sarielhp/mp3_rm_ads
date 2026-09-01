package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func executeEpisodeDownloads(client *ABSClient, item PodcastItem, episodesToDownload []FeedEpisode, reasons []string, podcastTitle string, itemID string, noWait, dryRun, oldest, forceNewOnly, verbose bool, keep *int, quiet bool) int {
	if len(episodesToDownload) > 0 {
		if !quiet {
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
		if !quiet {
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
			if !quiet {
				fmt.Printf("Queueing download request in Audiobookshelf for %d episode(s)...\n", len(episodesToDownload))
			}
			if err := client.DownloadEpisodes(itemID, episodesToDownload); err != nil {
				printError(fmt.Sprintf("Failed to queue episode download: %v", err))
			} else {
				if !quiet {
					fmt.Println("Download request successfully sent to Audiobookshelf!")
				}

				if !noWait {
					if !quiet {
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
							if !quiet {
								fmt.Printf("\nDownload completed! Podcast now has %d downloaded episodes.\n", newCount)
							}
							if podDir := findPodcastDirForItem(item, loadConfig().PodcastsDir); podDir != "" {
								for _, m := range findMP3Files(podDir) {
									if !fileExists(statusPathFor(m)) {
										_ = getOrCreateEpisodeStatus(m)
									}
								}
							}
							break
						} else if len(activeDls) > 0 {
							if !quiet {
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
							if !quiet {
								fmt.Println("\nTimeout waiting for download to finish. Download continues in background.")
							}
							break
						}
					}
				}
			}
		} else {
			if !quiet {
				fmt.Println("Dry run mode enabled. Skipping actual download request.")
			}
		}
	} else if !forceNewOnly && !quiet {
		fmt.Printf("No new episodes to download for %s.\n", podcastTitle)
	}

	if keep != nil {
		applyKeepPolicy(client, itemID, podcastTitle, *keep, dryRun, verbose, quiet)
	}

	return 0
}
