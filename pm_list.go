package main

import (
	"fmt"
	"strconv"
)

func printPodcastList(client *ABSClient, podcasts []PodcastItem, verbose bool, silent bool) {
	if silent {
		return
	}
	fmt.Println("\n=== Available Podcasts in Audiobookshelf ===")
	for idx, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = "Untitled Podcast"
		}
		feedURL := item.Media.Metadata.FeedURL
		downloaded := len(item.Media.Episodes)

		feedCountStr := "N/A"
		if feedURL != "" {
			eps, err := client.PodcastFeedEpisodes(feedURL)
			if err == nil {
				feedCountStr = strconv.Itoa(len(eps))
			}
		}

		fmt.Printf(" %d. %s\n", idx+1, title)
		if verbose {
			fmt.Printf("    ID: %s\n", item.ID)
		}
		fmt.Printf("    Downloaded: %d | Available in Catalog: %s\n", downloaded, feedCountStr)
		if verbose {
			if feedURL == "" {
				fmt.Println("    Feed URL: None")
			} else {
				fmt.Printf("    Feed URL: %s\n", feedURL)
			}
		}
		fmt.Println()
	}
}
