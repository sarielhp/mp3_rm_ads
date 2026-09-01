package main

import (
	"fmt"
	"strconv"
	"strings"
)

func printPodcastList(client *ABSClient, podcasts []PodcastItem, verbose bool, quiet bool) {
	if quiet {
		return
	}
	name := "Podcast Server"
	if client != nil {
		bname := client.Name()
		if bname == "podfetch" {
			name = "PodFetch"
		} else if bname == "audiobookshelf" {
			name = "Audiobookshelf"
		} else {
			name = strings.Title(bname)
		}
	}
	fmt.Printf("\n=== Available Podcasts in %s ===\n", name)
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
