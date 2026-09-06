package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	client, _ := getABSClient(config, cli.Quiet)
	targetItems := collectFrequencyTargetItems(config, cli, client)

	if len(targetItems) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcasts found to analyze.")
		}
		return
	}

	shouldDisable := cli.ServerSubcmd == "disable_hourly" || cli.ServerSubcmd == "disable-hourly" || cli.DisableHourly
	if !cli.Quiet {
		desc := "Analyzing podcast release frequency"
		if shouldDisable {
			desc = "Analyzing podcast frequency and updating hourly policies"
		}
		fmt.Printf("\n%s (%d podcast(s))...\n\n", desc, len(targetItems))
	}

	results := make([]podcastFreqResult, len(targetItems))
	for i, item := range targetItems {
		results[i] = processSingleFrequencyItem(client, item, config, cli, shouldDisable)
	}

	_ = globalFeedCache.Save()

	if !cli.Quiet {
		printFrequencyTable(results, cli.Verbose, shouldDisable)
	}
}

func collectFrequencyTargetItems(config Config, cli CLIOptions, client *ABSClient) []PodcastItem {
	var targetItems []PodcastItem
	if client != nil {
		if podcasts, pErr := client.PodcastItems(); pErr == nil {
			if cli.Podcast != "" {
				if matched := matchPodcast(podcasts, cli.Podcast); matched != nil {
					targetItems = append(targetItems, *matched)
				} else {
					fatalError("%s\n", fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
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
	return targetItems
}

func processSingleFrequencyItem(client *ABSClient, item PodcastItem, config Config, cli CLIOptions, shouldDisable bool) podcastFreqResult {
	title := item.Media.Metadata.Title
	if title == "" {
		title = item.ID
	}
	eps, epErr := getEpisodesForFrequency(client, item, config.PodcastsDir, cli.Refresh)
	if epErr != nil {
		return podcastFreqResult{title: title, item: item, err: epErr}
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

	return podcastFreqResult{
		title:       title,
		item:        item,
		freq:        freq,
		podDir:      podDir,
		disabled:    disabled,
		policySaved: saved,
	}
}

func printFrequencyTable(results []podcastFreqResult, verbose bool, disableMode bool) {
	cadenceGroups := []struct {
		cadence PodcastCadence
		title   string
	}{
		{CadenceHourly, "HOURLY"},
		{CadenceDaily, "DAILY"},
		{CadenceWeekly, "WEEKLY"},
		{CadenceMonthly, "MONTHLY"},
		{CadenceIntermittent, "INTERMITTENT"},
	}

	grouped := make(map[PodcastCadence][]podcastFreqResult)
	var errors []podcastFreqResult
	disabledCount := 0

	for _, r := range results {
		if r.err != nil {
			errors = append(errors, r)
			continue
		}
		if r.disabled {
			disabledCount++
		}
		c := PodcastCadence(r.freq.Type)
		grouped[c] = append(grouped[c], r)
	}

	for c := range grouped {
		sort.Slice(grouped[c], func(i, j int) bool {
			return strings.ToLower(grouped[c][i].title) < strings.ToLower(grouped[c][j].title)
		})
	}

	fmt.Printf("  %-3s │ %-38s │ %-9s │ %-11s │ %-11s │ %s\n",
		"#", "Podcast", "Analyzed", "Episodes/Wk", "Median Int", "Status")
	doubleDivider := strings.Repeat("═", 98)
	fmt.Println("  " + doubleDivider)

	globalIdx := 1
	for _, g := range cadenceGroups {
		list := grouped[g.cadence]
		if len(list) == 0 {
			continue
		}
		printCadenceGroup(g.title, list, &globalIdx, verbose, disableMode)
	}

	if len(errors) > 0 {
		printFrequencyErrors(errors, &globalIdx)
	}

	fmt.Println("  " + doubleDivider)

	if disableMode {
		fmt.Printf("\nCompleted hourly check: %d podcast(s) updated with download_policy=none and ad_removal=none.\n\n", disabledCount)
	} else {
		fmt.Println()
	}
}

func printCadenceGroup(groupTitle string, list []podcastFreqResult, globalIdx *int, verbose, disableMode bool) {
	headerTitle := fmt.Sprintf("─── %s (%d podcast%s) ", groupTitle, len(list), func() string {
		if len(list) == 1 {
			return ""
		}
		return "s"
	}())
	if len(headerTitle) < 98 {
		headerTitle += strings.Repeat("─", 98-len(headerTitle))
	}
	fmt.Println("  " + headerTitle)

	for _, r := range list {
		status := "saved"
		if r.disabled {
			status = "disabled (none)"
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
		fmt.Printf("  %2d. │ %-38s │ %-9s │ %-11s │ %-11s │ %s\n",
			*globalIdx, truncate(displayName(r.title), 38), epsStr, epWk, medInt, status)
		*globalIdx++

		if verbose {
			fmt.Printf("       ↳ Span: %.1fd | Avg Interval: %.1fd | Median: %.1fh | Analyzed: %s\n",
				r.freq.AvgDaysInterval*float64(max(1, r.freq.EpisodesAnalyzed-1)),
				r.freq.AvgDaysInterval,
				r.freq.MedianHoursInterval,
				r.freq.AnalyzedAt.Format("2006-01-02 15:04:05 UTC"))
		}
	}
	fmt.Println("  " + strings.Repeat("─", 98))
}

func printFrequencyErrors(errors []podcastFreqResult, globalIdx *int) {
	headerTitle := fmt.Sprintf("─── ERRORS (%d) ", len(errors))
	if len(headerTitle) < 98 {
		headerTitle += strings.Repeat("─", 98-len(headerTitle))
	}
	fmt.Println("  " + headerTitle)
	for _, r := range errors {
		fmt.Printf("  %2d. │ %-38s │ %-9s │ %-11s │ %-11s │ %s\n",
			*globalIdx, truncate(displayName(r.title), 38), "-", "-", "-", r.err.Error())
		*globalIdx++
	}
	fmt.Println("  " + strings.Repeat("─", 98))
}
