package main

import (
	"fmt"
	"time"
)

type getInfoResult struct {
	title       string
	fetched     int
	totalInFeed int
	latestTitle string
	latestDate  string
	err         error
}

func handleServerGetInfo(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error connecting to Audiobookshelf: %v", err))
	}

	targetPodcasts := resolveTargetPodcastsForGetInfo(client, cli)
	limitK := cli.Count
	if limitK <= 0 {
		limitK = 100
	}

	if !cli.Quiet {
		fmt.Printf("\nFetching episode metadata (up to %d latest episodes) for %d podcast(s)...\n\n", limitK, len(targetPodcasts))
	}

	results := make([]getInfoResult, len(targetPodcasts))
	sem := make(chan struct{}, 15)
	var wg syncWG

	for idx, pod := range targetPodcasts {
		wg.Add(1)
		go func(i int, item PodcastItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = fetchPodcastInfoTask(client, item, limitK, cli.Refresh, config.PodcastsDir)
		}(idx, pod)
	}

	wg.Wait()
	_ = globalFeedCache.Save()
	printGetInfoResults(results, cli.Quiet, cli.Verbose)
}

func resolveTargetPodcastsForGetInfo(client *ABSClient, cli CLIOptions) []PodcastItem {
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts from Audiobookshelf: %v", err))
	}
	if cli.Podcast != "" {
		match := matchPodcast(podcasts, cli.Podcast)
		if match == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching '%s' not found in Audiobookshelf.", cli.Podcast))
		}
		return []PodcastItem{*match}
	}
	return podcasts
}

func fetchPodcastInfoTask(client *ABSClient, item PodcastItem, limitK int, refresh bool, podcastsDir string) getInfoResult {
	title := item.Media.Metadata.Title
	if title == "" {
		title = "Untitled Podcast"
	}
	feedURL := item.Media.Metadata.FeedURL
	if feedURL == "" {
		return getInfoResult{title: title, err: fmt.Errorf("no RSS feed URL configured")}
	}

	cachedEps, totalEps, err := fetchOrCacheEpisodes(client, feedURL, limitK, refresh)
	if err != nil {
		return getInfoResult{title: title, err: err}
	}

	latestEpTitle, latestEpDate := extractLatestEpisodeInfo(cachedEps)
	updatePodcastDirCache(item, title, feedURL, podcastsDir)

	return getInfoResult{
		title:       title,
		fetched:     len(cachedEps),
		totalInFeed: totalEps,
		latestTitle: latestEpTitle,
		latestDate:  latestEpDate,
	}
}

func fetchOrCacheEpisodes(client *ABSClient, feedURL string, limitK int, refresh bool) ([]FeedEpisode, int, error) {
	if !refresh {
		if entry := globalFeedCache.Get(feedURL); entry != nil && len(entry.Episodes) > 0 && !entry.IsExpired(FeedCacheDefaultTTL) {
			totalEps := len(entry.Episodes)
			takeCount := min(totalEps, limitK)
			return entry.Episodes[:takeCount], totalEps, nil
		}
	}

	feedEpisodes, err := client.PodcastFeedEpisodes(feedURL)
	if err != nil {
		return nil, 0, err
	}

	totalEps := len(feedEpisodes)
	takeCount := min(totalEps, limitK)
	cachedEps := feedEpisodes[:takeCount]

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
	return cachedEps, totalEps, nil
}

func extractLatestEpisodeInfo(cachedEps []FeedEpisode) (string, string) {
	if len(cachedEps) == 0 {
		return "", ""
	}
	latestEpTitle := cachedEps[0].Title
	latestEpDate := cachedEps[0].PubDate
	if latestEpDate == "" && cachedEps[0].PublishedAt > 0 {
		latestEpDate = time.UnixMilli(cachedEps[0].PublishedAt).Format(time.RFC1123)
	}
	return latestEpTitle, latestEpDate
}

func updatePodcastDirCache(item PodcastItem, title, feedURL, podcastsDir string) {
	podDir := findPodcastDirForItem(item, podcastsDir)
	if podDir == "" {
		return
	}
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

func printGetInfoResults(results []getInfoResult, quiet, verbose bool) {
	totalFetched := 0
	totalSuccess := 0
	for _, res := range results {
		if res.err != nil {
			if !quiet {
				fmt.Printf("✗ %-50s : error (%v)\n", displayName(res.title), res.err)
			}
			continue
		}
		totalSuccess++
		totalFetched += res.fetched
		if !quiet {
			fmt.Printf("✓ %-50s : %d episodes (latest: %q)\n",
				displayName(res.title), res.fetched, displayName(res.latestTitle))
			if verbose && res.latestDate != "" {
				fmt.Printf("    Published: %s\n", res.latestDate)
			}
		}
	}

	if !quiet {
		fmt.Printf("\nDone! Fetched and cached metadata for %d episodes across %d podcast(s).\n\n",
			totalFetched, totalSuccess)
	}
}
