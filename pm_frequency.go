package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PodcastCadence string

const (
	CadenceHourly       PodcastCadence = "hourly"
	CadenceDaily        PodcastCadence = "daily"
	CadenceWeekly       PodcastCadence = "weekly"
	CadenceMonthly      PodcastCadence = "monthly"
	CadenceIntermittent PodcastCadence = "intermittent"
)

type PodcastFrequencyInfo struct {
	Type                string    `json:"type"`
	EpisodesAnalyzed    int       `json:"episodes_analyzed"`
	AvgDaysInterval     float64   `json:"avg_days_interval"`
	MedianHoursInterval float64   `json:"median_hours_interval"`
	EpisodesPerWeek     float64   `json:"episodes_per_week"`
	AnalyzedAt          time.Time `json:"analyzed_at"`
}

func analyzePodcastFrequency(episodes []FeedEpisode) PodcastFrequencyInfo {
	if len(episodes) < 3 {
		return PodcastFrequencyInfo{
			Type:             string(CadenceIntermittent),
			EpisodesAnalyzed: len(episodes),
			AnalyzedAt:       time.Now().UTC(),
		}
	}

	var timestamps []int64
	for _, ep := range episodes {
		if ms := getPubMS(ep); ms > 0 {
			timestamps = append(timestamps, ms)
		}
	}

	if len(timestamps) < 3 {
		return PodcastFrequencyInfo{
			Type:             string(CadenceIntermittent),
			EpisodesAnalyzed: len(episodes),
			AnalyzedAt:       time.Now().UTC(),
		}
	}

	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	intervals := make([]float64, len(timestamps)-1)
	for i := 0; i < len(timestamps)-1; i++ {
		diffMs := max(0, timestamps[i+1]-timestamps[i])
		intervals[i] = float64(diffMs) / (1000.0 * 3600.0)
	}

	spanDays := (float64(timestamps[len(timestamps)-1]-timestamps[0]) / (1000.0 * 3600.0)) / 24.0
	avgDaysInterval := spanDays / float64(len(intervals))

	var releasesPerDay, episodesPerWeek float64
	if avgDaysInterval > 0 {
		releasesPerDay = 1.0 / avgDaysInterval
		episodesPerWeek = releasesPerDay * 7.0
	}

	sortedIntervals := make([]float64, len(intervals))
	copy(sortedIntervals, intervals)
	sort.Float64s(sortedIntervals)

	var medianHoursInterval float64
	n := len(sortedIntervals)
	if n%2 == 1 {
		medianHoursInterval = sortedIntervals[n/2]
	} else {
		medianHoursInterval = (sortedIntervals[n/2-1] + sortedIntervals[n/2]) / 2.0
	}

	var cadence PodcastCadence
	if releasesPerDay > 10.0 || medianHoursInterval <= 4.0 {
		cadence = CadenceHourly
	} else if len(episodes) < 10 {
		cadence = CadenceIntermittent
	} else if episodesPerWeek >= 4.0 || medianHoursInterval <= 48.0 {
		cadence = CadenceDaily
	} else if episodesPerWeek >= 0.75 || medianHoursInterval <= 240.0 {
		cadence = CadenceWeekly
	} else if medianHoursInterval <= 1080.0 {
		cadence = CadenceMonthly
	} else {
		cadence = CadenceIntermittent
	}

	return PodcastFrequencyInfo{
		Type:                string(cadence),
		EpisodesAnalyzed:    len(episodes),
		AvgDaysInterval:     avgDaysInterval,
		MedianHoursInterval: medianHoursInterval,
		EpisodesPerWeek:     episodesPerWeek,
		AnalyzedAt:          time.Now().UTC(),
	}
}

type podcastFreqResult struct {
	title       string
	item        PodcastItem
	freq        PodcastFrequencyInfo
	podDir      string
	disabled    bool
	policySaved bool
	err         error
}

func getEpisodesForFrequency(client *ABSClient, item PodcastItem, podcastsDir string, refresh bool) ([]FeedEpisode, error) {
	feedURL := item.Media.Metadata.FeedURL
	podDir := findPodcastDirForItem(item, podcastsDir)

	if !refresh {
		if feedURL != "" {
			if entry := globalFeedCache.Get(feedURL); entry != nil && len(entry.Episodes) > 0 && !entry.IsExpired(FeedCacheDefaultTTL) {
				return entry.Episodes, nil
			}
		}
		if podDir != "" {
			if eps := loadCachedFeedEpisodes(podDir); len(eps) > 0 {
				return eps, nil
			}
		}
	}

	if client != nil && feedURL != "" {
		if feedEpisodes, err := client.PodcastFeedEpisodes(feedURL); err == nil && len(feedEpisodes) > 0 {
			takeCount := min(100, len(feedEpisodes))
			cachedEps := feedEpisodes[:takeCount]
			globalFeedCache.Put(feedURL, &FeedCacheEntry{
				FeedURL:     feedURL,
				LastChecked: time.Now(),
				Episodes:    cachedEps,
			})
			return cachedEps, nil
		}
	}

	if len(item.Media.Episodes) > 0 {
		var eps []FeedEpisode
		for _, ep := range item.Media.Episodes {
			eps = append(eps, FeedEpisode{
				Title:       ep.Title,
				PubDate:     ep.PubDate,
				PublishedAt: ep.PublishedAt,
			})
		}
		return eps, nil
	}

	if podDir != "" {
		if eps := loadCachedFeedEpisodes(podDir); len(eps) > 0 {
			return eps, nil
		}
	}

	return nil, fmt.Errorf("no episodes available")
}

func loadCachedFeedEpisodes(podDir string) []FeedEpisode {
	if cache, _ := loadPodcastCache(podDir); cache != nil && len(cache.Episodes) > 0 {
		eps := make([]FeedEpisode, len(cache.Episodes))
		for i, ep := range cache.Episodes {
			eps[i] = FeedEpisode{Title: ep.Title, PublishedAt: ep.PublishedAt}
		}
		return eps
	}
	return nil
}

func handleServerFrequency(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	var targetItems []PodcastItem

	if err == nil && client != nil {
		if podcasts, pErr := client.PodcastItems(); pErr == nil {
			if cli.Podcast != "" {
				if matched := matchPodcast(podcasts, cli.Podcast); matched != nil {
					targetItems = append(targetItems, *matched)
				} else {
					printError(fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
					os.Exit(1)
				}
			} else {
				targetItems = podcasts
			}
		}
	}

	if len(targetItems) == 0 && config.PodcastsDir != "" {
		if entries, rErr := os.ReadDir(config.PodcastsDir); rErr == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if cli.Podcast != "" && !strings.EqualFold(e.Name(), cli.Podcast) && !strings.Contains(strings.ToLower(e.Name()), strings.ToLower(cli.Podcast)) {
					continue
				}
				podDir := filepath.Join(config.PodcastsDir, e.Name())
				var feedURL string
				if cache, _ := loadPodcastCache(podDir); cache != nil {
					feedURL = cache.FeedURL
				}
				targetItems = append(targetItems, PodcastItem{
					ID: e.Name(),
					Media: PodcastMedia{
						Metadata: PodcastMetadata{Title: e.Name(), FeedURL: feedURL},
					},
				})
			}
		}
	}

	if len(targetItems) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcasts found to analyze.")
		}
		return
	}

	shouldDisable := cli.ServerSubcmd == "disable_hourly" || cli.DisableHourly
	if !cli.Quiet {
		desc := "Analyzing podcast release frequency"
		if shouldDisable {
			desc = "Analyzing podcast frequency and updating hourly policies"
		}
		fmt.Printf("\n%s (%d podcast(s))...\n\n", desc, len(targetItems))
	}

	results := make([]podcastFreqResult, len(targetItems))
	for i, item := range targetItems {
		title := item.Media.Metadata.Title
		if title == "" {
			title = item.ID
		}
		eps, epErr := getEpisodesForFrequency(client, item, config.PodcastsDir, cli.Refresh)
		if epErr != nil {
			results[i] = podcastFreqResult{title: title, item: item, err: epErr}
			continue
		}

		freq := analyzePodcastFrequency(eps)
		podDir := findPodcastDirForItem(item, config.PodcastsDir)
		disabled, saved := false, false

		if podDir != "" {
			cfg := loadPodcastConfig(podDir)
			cfg.Frequency = &freq
			if shouldDisable && freq.Type == string(CadenceHourly) {
				cfg.DownloadPolicy = DownloadPolicyNone
				cfg.AdRemoval = AdRemovalNone
				disabled = true
			}
			if sErr := savePodcastConfig(podDir, cfg); sErr == nil {
				saved = true
			}
		}

		results[i] = podcastFreqResult{
			title:       title,
			item:        item,
			freq:        freq,
			podDir:      podDir,
			disabled:    disabled,
			policySaved: saved,
		}
	}

	_ = globalFeedCache.Save()

	if !cli.Quiet {
		printFrequencyTable(results, cli.Verbose, shouldDisable)
	}
}

func printFrequencyTable(results []podcastFreqResult, verbose bool, disableMode bool) {
	fmt.Printf("  %-3s │ %-32s │ %-12s │ %-9s │ %-11s │ %-11s │ %s\n",
		"#", "Podcast", "Cadence", "Analyzed", "Episodes/Wk", "Median Int", "Status")
	divider := strings.Repeat("─", 98)
	fmt.Println("  " + divider)

	disabledCount := 0
	for i, r := range results {
		if r.err != nil {
			fmt.Printf("  %2d. │ %-32s │ %-12s │ %-9s │ %-11s │ %-11s │ %s\n",
				i+1, truncate(displayName(r.title), 32), "error", "-", "-", "-", r.err.Error())
			continue
		}

		status := "saved"
		if r.disabled {
			status = "disabled (none)"
			disabledCount++
		} else if r.freq.Type == string(CadenceHourly) && !disableMode {
			status = "hourly (active)"
		}

		epWk := fmt.Sprintf("%.1f/wk", r.freq.EpisodesPerWeek)
		if r.freq.EpisodesPerWeek == 0 && r.freq.Type == string(CadenceIntermittent) {
			epWk = "-"
		}

		medInt := fmt.Sprintf("%.1fh", r.freq.MedianHoursInterval)
		if r.freq.MedianHoursInterval >= 48.0 {
			medInt = fmt.Sprintf("%.1fd", r.freq.MedianHoursInterval/24.0)
		} else if r.freq.MedianHoursInterval == 0 && r.freq.Type == string(CadenceIntermittent) {
			medInt = "-"
		}

		epsStr := fmt.Sprintf("%d eps", r.freq.EpisodesAnalyzed)
		fmt.Printf("  %2d. │ %-32s │ %-12s │ %-9s │ %-11s │ %-11s │ %s\n",
			i+1, truncate(displayName(r.title), 32), r.freq.Type, epsStr, epWk, medInt, status)

		if verbose {
			fmt.Printf("       ↳ Span: %.1fd | Avg Interval: %.1fd | Median Interval: %.1fh | Analyzed: %s\n",
				r.freq.AvgDaysInterval*float64(max(1, r.freq.EpisodesAnalyzed-1)),
				r.freq.AvgDaysInterval,
				r.freq.MedianHoursInterval,
				r.freq.AnalyzedAt.Format("2006-01-02 15:04:05 UTC"))
		}
	}
	fmt.Println("  " + divider)

	if disableMode {
		fmt.Printf("\nCompleted hourly check: %d podcast(s) updated with download_policy=none and ad_removal=none.\n\n", disabledCount)
	} else {
		fmt.Println()
	}
}
