package main

import (
	"fmt"
	"sync"
	"time"
)

func handleServerGetInfo(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error connecting to Audiobookshelf: %v", err))
	}

	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts from Audiobookshelf: %v", err))
	}

	limitK := cli.Count
	if limitK <= 0 {
		limitK = 100
	}

	var targetPodcasts []PodcastItem
	if cli.Podcast != "" {
		match := matchPodcast(podcasts, cli.Podcast)
		if match == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching '%s' not found in Audiobookshelf.", cli.Podcast))
		}
		targetPodcasts = append(targetPodcasts, *match)
	} else {
		targetPodcasts = podcasts
	}

	if !cli.Quiet {
		fmt.Printf("\nFetching episode metadata (up to %d latest episodes) for %d podcast(s)...\n\n", limitK, len(targetPodcasts))
	}

	type getInfoResult struct {
		title       string
		fetched     int
		totalInFeed int
		latestTitle string
		latestDate  string
		err         error
	}

	results := make([]getInfoResult, len(targetPodcasts))
	sem := make(chan struct{}, 15)
	var wg sync.WaitGroup

	for idx, pod := range targetPodcasts {
		wg.Add(1)
		go func(i int, item PodcastItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			title := item.Media.Metadata.Title
			if title == "" {
				title = "Untitled Podcast"
			}
			feedURL := item.Media.Metadata.FeedURL
			if feedURL == "" {
				results[i] = getInfoResult{title: title, err: fmt.Errorf("no RSS feed URL configured")}
				return
			}

			var cachedEps []FeedEpisode
			totalEps := 0

			if !cli.Refresh {
				if entry := globalFeedCache.Get(feedURL); entry != nil && len(entry.Episodes) > 0 && !entry.IsExpired(FeedCacheDefaultTTL) {
					totalEps = len(entry.Episodes)
					takeCount := totalEps
					if takeCount > limitK {
						takeCount = limitK
					}
					cachedEps = entry.Episodes[:takeCount]
				}
			}

			if len(cachedEps) == 0 {
				feedEpisodes, err := client.PodcastFeedEpisodes(feedURL)
				if err != nil {
					results[i] = getInfoResult{title: title, err: err}
					return
				}

				totalEps = len(feedEpisodes)
				takeCount := totalEps
				if takeCount > limitK {
					takeCount = limitK
				}
				cachedEps = feedEpisodes[:takeCount]

				latestGUID := ""
				if len(cachedEps) > 0 {
					latestGUID = cachedEps[0].GUID
				}
				globalFeedCache.Put(feedURL, &FeedCacheEntry{
					FeedURL:     feedURL,
					LastChecked: time.Now(),
					LatestGUID:  latestGUID,
					Episodes:    cachedEps,
				})
			}

			latestEpTitle := ""
			latestEpDate := ""
			if len(cachedEps) > 0 {
				latestEpTitle = cachedEps[0].Title
				latestEpDate = cachedEps[0].PubDate
				if latestEpDate == "" && cachedEps[0].PublishedAt > 0 {
					latestEpDate = time.UnixMilli(cachedEps[0].PublishedAt).Format(time.RFC1123)
				}
			}

			podDir := findPodcastDirForItem(item, config.PodcastsDir)
			if podDir != "" {
				cache, _ := loadPodcastCache(podDir)
				if cache == nil {
					cache = &CachedPodcastIndex{
						PodcastName: title,
						PodcastDir:  podDir,
						FeedURL:     feedURL,
						ABSItemID:   item.ID,
					}
				}
				cache.UpdatedAt = time.Now()
				_ = savePodcastCache(podDir, cache)
			}

			results[i] = getInfoResult{
				title:       title,
				fetched:     len(cachedEps),
				totalInFeed: totalEps,
				latestTitle: latestEpTitle,
				latestDate:  latestEpDate,
			}
		}(idx, pod)
	}

	wg.Wait()
	_ = globalFeedCache.Save()

	totalFetched := 0
	totalSuccess := 0
	for _, res := range results {
		if res.err != nil {
			if !cli.Quiet {
				fmt.Printf("✗ %-50s : error (%v)\n", displayName(res.title), res.err)
			}
			continue
		}
		totalSuccess++
		totalFetched += res.fetched
		if !cli.Quiet {
			fmt.Printf("✓ %-50s : %d episodes (latest: %q)\n",
				displayName(res.title), res.fetched, displayName(res.latestTitle))
			if cli.Verbose && res.latestDate != "" {
				fmt.Printf("    Published: %s\n", res.latestDate)
			}
		}
	}

	if !cli.Quiet {
		fmt.Printf("\nDone! Fetched and cached metadata for %d episodes across %d podcast(s).\n\n",
			totalFetched, totalSuccess)
	}
}
