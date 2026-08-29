package main

import (
	"fmt"
	"sort"
)

func applyKeepPolicy(client *ABSClient, itemID, podcastTitle string, keep int, dryRun bool, verbose bool, silent bool) bool {
	updatedItem, err := client.GetItem(itemID)
	if err != nil {
		printError(fmt.Sprintf("Failed to fetch updated item for keep policy: %v", err))
		return false
	}

	downloadedEpisodes := updatedItem.Media.Episodes
	sortedDownloaded := make([]PodcastEpisode, len(downloadedEpisodes))
	copy(sortedDownloaded, downloadedEpisodes)

	getEpPubMS := func(ep PodcastEpisode) int64 {
		if ep.PublishedAt > 0 {
			return ep.PublishedAt
		}
		return parsePubDate(ep.PubDate)
	}

	sort.Slice(sortedDownloaded, func(i, j int) bool {
		return getEpPubMS(sortedDownloaded[i]) < getEpPubMS(sortedDownloaded[j])
	})

	if len(sortedDownloaded) > keep {
		toDeleteCount := len(sortedDownloaded) - keep
		episodesToDelete := sortedDownloaded[:toDeleteCount]

		if !silent {
			fmt.Printf("\n=== Keep Policy (%d) for %s ===\n", keep, podcastTitle)
			fmt.Printf("Currently downloaded: %d episode(s).\n", len(sortedDownloaded))
			fmt.Printf("Keeping latest %d episode(s), deleting %d oldest episode(s)...\n", keep, toDeleteCount)
		}

		for idx, ep := range episodesToDelete {
			title := ep.Title
			if title == "" {
				title = "Untitled Episode"
			}
			epID := ep.ID
			pub := ep.PubDate
			if pub == "" {
				pub = fmt.Sprintf("%d", ep.PublishedAt)
			}
			if !silent {
				if verbose {
					fmt.Printf("  %d. Deleting: %s (Published: %s, ID: %s)\n", idx+1, title, pub, epID)
				} else {
					fmt.Printf("  %d. Deleting: %s (Published: %s)\n", idx+1, title, pub)
				}
			}
			if dryRun {
				if !silent {
					fmt.Println("     Dry run: Skipping deletion.")
				}
			} else {
				if err := client.DeletePodcastEpisode(itemID, epID); err == nil {
					if !silent {
						fmt.Println("     Deleted successfully.")
					}
				} else {
					printError(fmt.Sprintf("     Failed to delete episode %s: %v", title, err))
				}
			}
		}
	} else {
		if !silent {
			fmt.Printf("\t%s: %d\n", podcastTitle, len(sortedDownloaded))
		}
	}
	return true
}
