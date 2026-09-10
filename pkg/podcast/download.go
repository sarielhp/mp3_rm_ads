package podcast

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"abs/pkg/backend"
	"abs/pkg/config"
)

type DownloadOptions struct {
	Count        int
	Oldest       bool
	DryRun       bool
	NoWait       bool
	Fill         bool
	CountGiven   bool
	CheckNew     bool
	ForceNewOnly bool
	DownloadAll  bool
	Keep         *int
	Verbose      bool
	Quiet        bool
}

func GetPubMS(ep backend.FeedEpisode) int64 {
	if ep.PublishedAt > 0 {
		return ep.PublishedAt
	}
	if ep.PubDate != "" {
		ms, _ := ParseFeedDate(ep.PubDate)
		return ms
	}
	return 0
}

func scanPodcastDiskTitles(item backend.Podcast) map[string]bool {
	diskTitles := make(map[string]bool)
	podDir := FindPodcastDirForItem(item, "")
	if podDir == "" {
		return diskTitles
	}
	entries, err := os.ReadDir(podDir)
	if err != nil {
		return diskTitles
	}
	for _, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name()))
		if entry.IsDir() {
			diskTitles[name] = true
		} else if strings.HasSuffix(name, ".mp3") {
			diskTitles[name] = true
			diskTitles[strings.TrimSuffix(name, ".mp3")] = true
		}
	}
	return diskTitles
}

func BuildDownloadedChecker(client backend.Backend, item backend.Podcast, itemID string) func(backend.FeedEpisode) bool {
	downloadedURLs := make(map[string]bool)
	downloadedGUIDs := make(map[string]bool)
	downloadedTitles := make(map[string]bool)

	isPodfetch := client != nil && client.Name() == "podfetch"
	for _, ep := range item.Media.Episodes {
		if ep.AudioFile == nil && isPodfetch {
			continue
		}
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

	queuedTitles := make(map[string]bool)
	queuedURLs := make(map[string]bool)
	queuedGUIDs := make(map[string]bool)

	if client != nil {
		activeDls, _ := client.ActiveDownloads(itemID)
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
	}

	diskTitles := scanPodcastDiskTitles(item)

	return func(ep backend.FeedEpisode) bool {
		encURL := ""
		if ep.Enclosure != nil {
			encURL = ep.Enclosure.URL
		}
		guid := ep.GUID
		title := strings.ToLower(strings.TrimSpace(ep.Title))

		if (encURL != "" && (downloadedURLs[encURL] || queuedURLs[encURL])) ||
			(guid != "" && (downloadedGUIDs[guid] || queuedGUIDs[guid])) ||
			(title != "" && (downloadedTitles[title] || queuedTitles[title])) {
			return true
		}
		if title != "" && diskTitles[title] {
			return true
		}
		return false
	}
}

func ResolveEpisodesToDownload(item backend.Podcast, sortedCatalog []backend.FeedEpisode, downloadedIndices []int, isDownloaded func(backend.FeedEpisode) bool, opts DownloadOptions) ([]backend.FeedEpisode, []string) {
	podDir := FindPodcastDirForItem(item, "")
	podCfg := config.DefaultPodcastConfig(nil)
	if podDir != "" {
		podCfg = config.LoadPodcastConfig(podDir, podCfg)
	}

	if opts.DownloadAll {
		eps, reasons := SelectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, config.DownloadPolicyAll, 0, opts.Oldest)
		if opts.CountGiven && opts.Count > 0 && len(eps) > opts.Count {
			eps = eps[:opts.Count]
		}
		return eps, reasons
	}
	if !opts.Fill && !opts.CountGiven {
		return SelectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, podCfg.DownloadPolicy, podCfg.DownloadK, opts.Oldest)
	}
	if opts.ForceNewOnly {
		return selectForceNewEpisodes(sortedCatalog, downloadedIndices, isDownloaded, opts.Count, opts.CountGiven, opts.Oldest)
	}
	if opts.Fill {
		eps, reasons := selectFillEpisodes(sortedCatalog, downloadedIndices, isDownloaded, opts.CheckNew, opts.Oldest, opts.Quiet, item.Media.Metadata.Title)
		if opts.CountGiven && len(eps) > opts.Count {
			eps = eps[:opts.Count]
		}
		return eps, reasons
	}
	return selectDefaultUndownloadedEpisodes(sortedCatalog, downloadedIndices, isDownloaded, opts.Count, opts.CountGiven, opts.CheckNew, opts.Oldest)
}

func selectForceNewEpisodes(sortedCatalog []backend.FeedEpisode, downloadedIndices []int, isDownloaded func(backend.FeedEpisode) bool, count int, countGiven, oldest bool) ([]backend.FeedEpisode, []string) {
	var episodesToDownload []backend.FeedEpisode
	var reasons []string
	if len(downloadedIndices) > 0 {
		maxIdx := -1
		for _, idx := range downloadedIndices {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		var newEpisodes []backend.FeedEpisode
		if maxIdx+1 < len(sortedCatalog) {
			for _, ep := range sortedCatalog[maxIdx+1:] {
				hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
				if hasEnc && !isDownloaded(ep) {
					newEpisodes = append(newEpisodes, ep)
				}
			}
		}
		if len(newEpisodes) > 0 {
			reasons = append(reasons, fmt.Sprintf("%d new episode(s)", len(newEpisodes)))
			episodesToDownload = newEpisodes
		}
	} else {
		var undownloaded []backend.FeedEpisode
		for _, ep := range sortedCatalog {
			hasEnc := (ep.Enclosure != nil && ep.Enclosure.URL != "") || ep.EnclosureURL != ""
			if hasEnc && !isDownloaded(ep) {
				undownloaded = append(undownloaded, ep)
			}
		}
		if len(undownloaded) > 0 {
			latestEp := undownloaded[len(undownloaded)-1]
			if oldest {
				latestEp = undownloaded[0]
			}
			reasons = append(reasons, "1 latest episode")
			episodesToDownload = []backend.FeedEpisode{latestEp}
		}
	}
	if countGiven && len(episodesToDownload) > count {
		episodesToDownload = episodesToDownload[:count]
	}
	return episodesToDownload, reasons
}

func selectFillEpisodes(sortedCatalog []backend.FeedEpisode, downloadedIndices []int, isDownloaded func(backend.FeedEpisode) bool, checkNew, oldest, quiet bool, podcastTitle string) ([]backend.FeedEpisode, []string) {
	var episodesToDownload []backend.FeedEpisode
	var reasons []string

	searchCatalog := make([]backend.FeedEpisode, len(sortedCatalog))
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
			var newEpisodes []backend.FeedEpisode
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

	inToDownload := func(target backend.FeedEpisode) bool {
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
	if gapTerminated && !quiet {
		fmt.Printf("Search for %s terminated: gap larger than 10 undownloaded episodes encountered.\n", podcastTitle)
	}
	return episodesToDownload, reasons
}

func selectDefaultUndownloadedEpisodes(sortedCatalog []backend.FeedEpisode, downloadedIndices []int, isDownloaded func(backend.FeedEpisode) bool, count int, countGiven, checkNew, oldest bool) ([]backend.FeedEpisode, []string) {
	var episodesToDownload []backend.FeedEpisode
	var reasons []string

	if checkNew && len(downloadedIndices) > 0 {
		maxIdx := -1
		for _, idx := range downloadedIndices {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		if maxIdx+1 < len(sortedCatalog) {
			var newEpisodes []backend.FeedEpisode
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
		var undownloaded []backend.FeedEpisode
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
	} else if countGiven && len(episodesToDownload) > count {
		episodesToDownload = episodesToDownload[:count]
	}
	return episodesToDownload, reasons
}

func DownloadPodcastEpisodes(client backend.Backend, item backend.Podcast, opts DownloadOptions) int {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}
	feedURL := item.Media.Metadata.FeedURL
	itemID := item.ID

	if feedURL == "" {
		if !opts.Quiet {
			fmt.Printf("Podcast %s has no RSS feed URL configured.\n", podcastTitle)
		}
		return 0
	}

	feedEpisodes, err := client.PodcastFeedEpisodes(feedURL)
	if err != nil {
		if !opts.Quiet {
			fmt.Printf("Failed to parse episode catalog for %s: %v\n", podcastTitle, err)
		}
		return 0
	}

	isDownloaded := BuildDownloadedChecker(client, item, itemID)
	sortedCatalog := make([]backend.FeedEpisode, len(feedEpisodes))
	copy(sortedCatalog, feedEpisodes)
	sort.Slice(sortedCatalog, func(i, j int) bool {
		return GetPubMS(sortedCatalog[i]) < GetPubMS(sortedCatalog[j])
	})

	var downloadedIndices []int
	for idx, ep := range sortedCatalog {
		if isDownloaded(ep) {
			downloadedIndices = append(downloadedIndices, idx)
		}
	}

	episodesToDownload, reasons := ResolveEpisodesToDownload(item, sortedCatalog, downloadedIndices, isDownloaded, opts)
	return ExecuteEpisodeDownloads(client, item, episodesToDownload, reasons, opts)
}

func ExecuteEpisodeDownloads(client backend.Backend, item backend.Podcast, episodesToDownload []backend.FeedEpisode, reasons []string, opts DownloadOptions) int {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}

	if len(episodesToDownload) > 0 {
		sortAndPrintSelectedEpisodes(episodesToDownload, podcastTitle, reasons, opts.Oldest, opts.Verbose, opts.Quiet)
		queueAndTrackDownloads(client, item, episodesToDownload, opts.NoWait, opts.DryRun, opts.Quiet)
	} else if !opts.ForceNewOnly && !opts.Quiet {
		fmt.Printf("No new episodes to download for %s.\n", podcastTitle)
	}

	return len(episodesToDownload)
}

func sortAndPrintSelectedEpisodes(episodesToDownload []backend.FeedEpisode, podcastTitle string, reasons []string, oldest, verbose, quiet bool) {
	if !quiet {
		fmt.Printf("\n=== Podcast: %s ===\n", podcastTitle)
	}
	sort.Slice(episodesToDownload, func(i, j int) bool {
		return GetPubMS(episodesToDownload[i]) < GetPubMS(episodesToDownload[j])
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
		}
	}
}

func queueAndTrackDownloads(client backend.Backend, item backend.Podcast, episodesToDownload []backend.FeedEpisode, noWait, dryRun, quiet bool) {
	if dryRun {
		if !quiet {
			fmt.Println("Dry run mode enabled. Skipping actual download request.")
		}
		return
	}

	if !quiet {
		fmt.Printf("Queueing download request for %d episode(s)...\n", len(episodesToDownload))
	}
	if err := client.DownloadEpisodes(item.ID, episodesToDownload); err != nil {
		if !quiet {
			fmt.Printf("Failed to queue episode download: %v\n", err)
		}
		return
	}

	if !quiet {
		fmt.Println("Download request successfully sent!")
	}
	if !noWait {
		waitForEpisodeDownloads(client, item.ID, quiet)
	}
}

func waitForEpisodeDownloads(client backend.Backend, itemID string, quiet bool) {
	if !quiet {
		fmt.Println("\nWaiting for backend to complete episode download(s)...")
	}
	time.Sleep(2 * time.Second)

	for {
		activeDls, err := client.ActiveDownloads(itemID)
		if err == nil && len(activeDls) == 0 {
			if !quiet {
				fmt.Println("Episode download(s) completed!")
			}
			return
		}
		time.Sleep(2 * time.Second)
	}
}
