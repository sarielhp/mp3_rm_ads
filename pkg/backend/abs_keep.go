package backend

import (
	"fmt"
	"os"
	"sort"
)

func (c *AudiobookshelfBackend) ApplyKeepPolicy(podcastID, podcastTitle string, keep int, dryRun, verbose, quiet bool) (int, error) {
	updatedItem, err := c.GetPodcast(podcastID)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch updated item for keep policy: %w", err)
	}

	downloadedEpisodes := updatedItem.Media.Episodes
	sortedDownloaded := make([]Episode, len(downloadedEpisodes))
	copy(sortedDownloaded, downloadedEpisodes)

	getEpPubMS := func(ep Episode) int64 {
		if ep.PublishedAt > 0 {
			return ep.PublishedAt
		}
		return ParsePubDate(ep.PubDate)
	}

	sort.Slice(sortedDownloaded, func(i, j int) bool {
		return getEpPubMS(sortedDownloaded[i]) < getEpPubMS(sortedDownloaded[j])
	})

	deletedCount := 0
	if len(sortedDownloaded) > keep {
		toDeleteCount := len(sortedDownloaded) - keep
		episodesToDelete := sortedDownloaded[:toDeleteCount]

		if !quiet {
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
			if !quiet {
				if verbose {
					fmt.Printf("  %d. Deleting: %s (Published: %s, ID: %s)\n", idx+1, title, pub, epID)
				} else {
					fmt.Printf("  %d. Deleting: %s (Published: %s)\n", idx+1, title, pub)
				}
			}
			if dryRun {
				if !quiet {
					fmt.Println("     Dry run: Skipping deletion.")
				}
			} else {
				if err := c.DeletePodcastEpisode(podcastID, epID); err == nil {
					deletedCount++
					if !quiet {
						fmt.Println("     Deleted successfully.")
					}
				} else if !quiet {
					fmt.Fprintf(os.Stderr, "     Failed to delete episode %s: %v\n", title, err)
				}
			}
		}
	} else if !quiet {
		fmt.Printf("\t%s: %d\n", podcastTitle, len(sortedDownloaded))
	}
	return deletedCount, nil
}
