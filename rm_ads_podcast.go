package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/sariel/abs/pkg/backend"
)

func resolvePodcastTarget(podcastsDir string, cli CLIOptions) (*ResolvedPodcast, bool) {
	candidate := ""
	if cli.Podcast != "" {
		candidate = cli.Podcast
	} else if len(cli.Args) == 1 {
		candidate = cli.Args[0]
	} else {
		return nil, false
	}

	cleanCand := strings.TrimSpace(candidate)
	if cleanCand == "" {
		return nil, false
	}

	lower := strings.ToLower(cleanCand)
	if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".m4a") ||
		strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".aac") {
		return nil, false
	}

	if podcastsDir == "" {
		podcastsDir = "."
	}

	if isExcludedRootPodcastsDir(podcastsDir, cleanCand) {
		return nil, false
	}

	if res, err := resolveAnyID(podcastsDir, cleanCand); err == nil && res.IsPodcast() {
		if !isExcludedRootPodcastsDir(podcastsDir, res.Podcast.Dir) {
			return res.Podcast, true
		}
	}

	if dir, title, found := resolvePodcastDirByIDOrName(podcastsDir, cleanCand); found {
		if !isExcludedRootPodcastsDir(podcastsDir, dir) {
			shortID := getOrSetPodcastShortID(dir, title)
			cfg := loadPodcastConfig(dir)
			return &ResolvedPodcast{
				Dir:        dir,
				Title:      title,
				ShortID:    shortID,
				FolderName: filepath.Base(dir),
				UUID:       cfg.ID,
				Config:     cfg,
			}, true
		}
	}

	return nil, false
}

func isExcludedRootPodcastsDir(podcastsDir, cand string) bool {
	absCand, err1 := filepath.Abs(cand)
	absRoot, err2 := filepath.Abs(podcastsDir)
	if err1 != nil || err2 != nil {
		return false
	}
	return absCand == absRoot
}

func handlePodcastRmAdsWorkflow(pod *ResolvedPodcast, cli CLIOptions, config Config, action string) error {
	targetAudioPath, err := resolveTargetEpisodeForRmAds(pod, cli, config)
	if err != nil {
		return err
	}
	if targetAudioPath == "" {
		return nil
	}

	epFilename := queueItemFilename(pod.Dir, targetAudioPath)
	displayEp := filepath.Base(targetAudioPath)
	if strings.EqualFold(stripExt(displayEp), "podcast") {
		if t := episodeTitleFromPath(targetAudioPath); t != "" {
			displayEp = t
		}
	}
	if cli.DryRun {
		if !cli.Quiet {
			fmt.Printf("[Dry run] Would queue and process %s for ad removal.\n", displayName(displayEp))
		}
		return nil
	}

	added := addEpisodeToQueueFile(pod.Dir, epFilename)
	epID := getOrSetEpisodeShortID(pod.Dir, pod.ShortID, targetAudioPath)
	title := episodeTitleFromPath(targetAudioPath)

	if !cli.Quiet {
		if added {
			fmt.Printf("Added to AdR queue: [%s] %s\n", boldCyan(epID), displayName(title))
		} else {
			fmt.Printf("Already in AdR queue: [%s] %s\n", boldCyan(epID), displayName(title))
		}
	}

	podcastsDir := config.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}
	totalQueued := countAllQueuedEpisodes(podcastsDir)
	if totalQueued > 1 {
		if !cli.Quiet {
			fmt.Printf("Queued for ad removal (%d items in queue).\n", totalQueued)
		}
		return nil
	}

	return processSingleQueuedTarget(pod.Dir, targetAudioPath, action, cli, config)
}

func countAllQueuedEpisodes(podcastsDir string) int {
	if podcastsDir == "" {
		podcastsDir = "."
	}
	entries := scanPodcastDirs(podcastsDir)
	total := 0
	for _, p := range entries {
		items := collectPodcastQueueItems(p)
		total += len(items)
	}
	return total
}

func resolveTargetEpisodeForRmAds(pod *ResolvedPodcast, cli CLIOptions, config Config) (string, error) {
	b := getActiveBackendForPodcast(config, cli.Quiet)
	if b != nil {
		if targetPath, handled := findTargetEpisodeFromBackend(b, pod, config, cli.Quiet); handled {
			return targetPath, nil
		}
	}

	targetPath, ok := findLatestUncleanedLocalEpisode(pod.Dir, pod.Title, cli.Quiet)
	if !ok {
		return "", nil
	}
	return targetPath, nil
}

func getActiveBackendForPodcast(config Config, quiet bool) backend.Backend {
	if isPodfetchActive(config) {
		if config.PodfetchURL != "" || config.PodfetchDBPath != "" {
			b, err := getBackend(config, quiet)
			if err == nil {
				return b
			}
		}
		return nil
	}
	if isAudiobookshelfActive(config) {
		if config.AudiobookshelfURL != "" || config.AudiobookshelfDBPath != "" {
			b, err := getABSClient(config, quiet)
			if err == nil {
				return b
			}
			b2, err2 := getBackend(config, quiet)
			if err2 == nil {
				return b2
			}
		}
	}
	return nil
}

func findTargetEpisodeFromBackend(b backend.Backend, pod *ResolvedPodcast, config Config, quiet bool) (string, bool) {
	feedURL, targetItem := resolveBackendPodcastAndFeed(b, pod, config)
	if feedURL == "" {
		return "", false
	}

	feedEpisodes, err := b.PodcastFeedEpisodes(feedURL)
	if err != nil || len(feedEpisodes) == 0 {
		feedEpisodes, _, _, _, _ = fetchFeedDirect(feedURL, "", "")
	}
	if len(feedEpisodes) == 0 {
		return "", false
	}

	sort.Slice(feedEpisodes, func(i, j int) bool {
		return getPubMS(feedEpisodes[i]) > getPubMS(feedEpisodes[j])
	})

	for i, fe := range feedEpisodes {
		localPath, isDownloaded := findLocalPathForFeedEpisode(pod.Dir, fe, targetItem)
		if isDownloaded {
			if isEpisodeClean(localPath) {
				continue
			}
			return localPath, true
		}

		if i == 0 {
			dlPath, dlErr := downloadSingleFeedEpisode(b, targetItem, pod.Dir, fe, quiet)
			if dlErr == nil && dlPath != "" {
				return dlPath, true
			}
		}
	}

	if uncleanedPath, ok := findLatestUncleanedLocalEpisode(pod.Dir, pod.Title, true); ok && uncleanedPath != "" {
		return uncleanedPath, true
	}

	if !quiet {
		fmt.Printf("All episodes for podcast %s already have ads removed.\n", displayName(pod.Title))
	}
	return "", true
}

func resolveBackendPodcastAndFeed(b backend.Backend, pod *ResolvedPodcast, config Config) (string, *backend.Podcast) {
	feedURL := ""
	itemID := pod.UUID
	if cached, _ := loadPodcastCache(pod.Dir); cached != nil {
		if cached.FeedURL != "" {
			feedURL = cached.FeedURL
		}
		if cached.ABSItemID != "" {
			itemID = cached.ABSItemID
		}
	}

	if itemID != "" {
		if targetItem, err := b.GetPodcast(itemID); err == nil && targetItem != nil {
			if targetItem.Media.Metadata.FeedURL != "" {
				feedURL = targetItem.Media.Metadata.FeedURL
			}
			if pod.Title == "" && targetItem.Media.Metadata.Title != "" {
				pod.Title = targetItem.Media.Metadata.Title
			}
			return feedURL, targetItem
		}
	}

	podcasts, err := b.Podcasts()
	if err != nil {
		return feedURL, nil
	}

	var targetItem *backend.Podcast
	for i := range podcasts {
		p := &podcasts[i]
		if isMatchingBackendPodcast(p, itemID, pod, config.PodcastsDir) {
			targetItem = p
			break
		}
	}

	if targetItem != nil {
		if targetItem.Media.Metadata.FeedURL != "" {
			feedURL = targetItem.Media.Metadata.FeedURL
		}
		if pod.Title == "" && targetItem.Media.Metadata.Title != "" {
			pod.Title = targetItem.Media.Metadata.Title
		}
	}

	return feedURL, targetItem
}

func isMatchingBackendPodcast(p *backend.Podcast, itemID string, pod *ResolvedPodcast, podcastsDir string) bool {
	if itemID != "" && p.ID == itemID {
		return true
	}
	if pod.UUID != "" && p.ID == pod.UUID {
		return true
	}
	if findPodcastDirForItem(*p, podcastsDir) == pod.Dir {
		return true
	}
	if strings.EqualFold(p.Media.Metadata.Title, pod.Title) {
		return true
	}
	return strings.EqualFold(filepath.Base(p.Path), filepath.Base(pod.Dir))
}

func normalizeForFuzzyMatch(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

func stripShowPrefix(title, showName string) string {
	if showName == "" || title == "" {
		return title
	}
	cleanShow := strings.ToLower(strings.TrimSpace(showName))
	cleanTitle := strings.TrimSpace(title)
	if strings.HasPrefix(strings.ToLower(cleanTitle), cleanShow) {
		trimmed := strings.TrimSpace(cleanTitle[len(cleanShow):])
		trimmed = strings.TrimLeft(trimmed, "-:–—_ ")
		if trimmed != "" {
			return trimmed
		}
	}
	return cleanTitle
}

func isFuzzyEpisodeMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	na := normalizeForFuzzyMatch(a)
	nb := normalizeForFuzzyMatch(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	shorter, longer := na, nb
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	if len([]rune(shorter)) >= 6 && strings.Contains(longer, shorter) {
		return true
	}
	return false
}

func resolveMatchingEpisodeAudioFile(podDir string, ep backend.Episode) (string, bool) {
	if ep.AudioFile == nil || ep.AudioFile.Metadata == nil {
		return "", false
	}
	podBase := filepath.Base(podDir)
	podRoot := filepath.Dir(podDir)

	checkCandidatePath := func(raw string) string {
		if raw == "" {
			return ""
		}
		if fileExists(raw) {
			return raw
		}
		if p := filepath.Join(podDir, raw); fileExists(p) {
			return p
		}
		if p := filepath.Join(podDir, filepath.Base(raw)); fileExists(p) {
			return p
		}
		epDirName := filepath.Base(filepath.Dir(raw))
		if epDirName != "." && epDirName != "/" && epDirName != "" && epDirName != podBase {
			if p := filepath.Join(podDir, epDirName, filepath.Base(raw)); fileExists(p) {
				return p
			}
		}

		if p := filepath.Join(podRoot, raw); fileExists(p) {
			return p
		}
		trimmed := strings.TrimPrefix(raw, "/podcasts/")
		trimmed = strings.TrimPrefix(trimmed, "podcasts/")
		trimmed = strings.TrimPrefix(trimmed, "/")
		if p := filepath.Join(podRoot, trimmed); fileExists(p) {
			return p
		}
		if p := filepath.Join(podDir, trimmed); fileExists(p) {
			return p
		}
		if strings.HasPrefix(trimmed, podBase+"/") {
			rel := strings.TrimPrefix(trimmed, podBase+"/")
			if p := filepath.Join(podDir, rel); fileExists(p) {
				return p
			}
		}
		return ""
	}

	if p := checkCandidatePath(ep.AudioFile.Metadata.Path); p != "" {
		return p, true
	}
	if p := checkCandidatePath(ep.AudioFile.Metadata.RelPath); p != "" {
		return p, true
	}
	if ep.AudioFile.Metadata.Filename != "" {
		if p := filepath.Join(podDir, ep.AudioFile.Metadata.Filename); fileExists(p) {
			return p, true
		}
	}
	return "", false
}

func findLocalPathForFeedEpisode(podDir string, fe backend.FeedEpisode, item *backend.Podcast) (string, bool) {
	if path, ok := findMatchingEpisodeInItem(podDir, fe, item); ok {
		return path, true
	}

	podBase := filepath.Base(podDir)
	safeTitle := sanitizePodcastTitle(fe.Title)
	feStripped := stripShowPrefix(fe.Title, podBase)
	for _, mp3 := range findMP3Files(podDir) {
		base := stripExt(filepath.Base(mp3))
		title := episodeTitleFromPath(mp3)
		baseStripped := stripShowPrefix(base, podBase)
		titleStripped := stripShowPrefix(title, podBase)
		if strings.EqualFold(base, safeTitle) || strings.EqualFold(base, fe.Title) ||
			strings.EqualFold(title, safeTitle) || strings.EqualFold(title, fe.Title) ||
			strings.EqualFold(sanitizePodcastTitle(title), safeTitle) ||
			strings.EqualFold(baseStripped, feStripped) ||
			strings.EqualFold(titleStripped, feStripped) ||
			isFuzzyEpisodeMatch(title, fe.Title) ||
			isFuzzyEpisodeMatch(titleStripped, feStripped) {
			return mp3, true
		}
		detailKey := filepath.Base(mp3)
		if strings.EqualFold(base, "podcast") && title != "" {
			detailKey = title + ".mp3"
		}
		if dt, _ := loadEpisodeDetails(podDir, detailKey); dt != nil {
			if (fe.GUID != "" && dt.Subtitle == fe.GUID) ||
				(fe.Title != "" && strings.EqualFold(dt.Title, fe.Title)) {
				return mp3, true
			}
		}
	}

	return "", false
}

func findMatchingEpisodeInItem(podDir string, fe backend.FeedEpisode, item *backend.Podcast) (string, bool) {
	if item == nil {
		return "", false
	}
	feURL := fe.EnclosureURL
	if feURL == "" && fe.Enclosure != nil {
		feURL = fe.Enclosure.URL
	}
	for _, ep := range item.Media.Episodes {
		matched := (feURL != "" && ep.EnclosureURL != "" && feURL == ep.EnclosureURL) ||
			(fe.GUID != "" && ep.GUID != "" && fe.GUID == ep.GUID) ||
			(fe.Title != "" && ep.Title != "" && strings.EqualFold(strings.TrimSpace(fe.Title), strings.TrimSpace(ep.Title))) ||
			(fe.Title != "" && ep.Title != "" && isFuzzyEpisodeMatch(ep.Title, fe.Title))
		if matched && ep.AudioFile != nil && ep.AudioFile.Metadata != nil {
			if p, ok := resolveMatchingEpisodeAudioFile(podDir, ep); ok {
				return p, true
			}
		}
	}
	return "", false
}

func downloadSingleFeedEpisode(b backend.Backend, item *backend.Podcast, podDir string, fe backend.FeedEpisode, quiet bool) (string, error) {
	if !quiet {
		fmt.Printf("Downloading latest episode: %s\n", fe.Title)
	}

	itemID := resolveDownloadPodcastID(item, podDir)
	if itemID == "" {
		return "", fmt.Errorf("could not determine podcast ID for download")
	}

	existingFiles := make(map[string]bool)
	for _, f := range findMP3Files(podDir) {
		existingFiles[f] = true
	}

	if err := b.DownloadEpisodes(itemID, []backend.FeedEpisode{fe}); err != nil {
		return "", fmt.Errorf("failed to trigger episode download: %w", err)
	}

	waitForBackendDownloads(b, itemID)

	for _, f := range findMP3Files(podDir) {
		if !existingFiles[f] {
			return f, nil
		}
	}

	safeTitle := sanitizePodcastTitle(fe.Title)
	for _, f := range findMP3Files(podDir) {
		base := stripExt(filepath.Base(f))
		title := episodeTitleFromPath(f)
		if strings.EqualFold(base, safeTitle) || strings.EqualFold(base, fe.Title) ||
			strings.EqualFold(title, safeTitle) || strings.EqualFold(title, fe.Title) ||
			strings.EqualFold(sanitizePodcastTitle(title), safeTitle) ||
			isFuzzyEpisodeMatch(title, fe.Title) {
			return f, nil
		}
	}

	return "", fmt.Errorf("downloaded file not found in %s", podDir)
}

func resolveDownloadPodcastID(item *backend.Podcast, podDir string) string {
	if item != nil && item.ID != "" {
		return item.ID
	}
	if cached, _ := loadPodcastCache(podDir); cached != nil {
		return cached.ABSItemID
	}
	return ""
}

func waitForBackendDownloads(b backend.Backend, itemID string) {
	startTime := time.Now()
	for {
		activeDls, err := b.ActiveDownloads(itemID)
		if err == nil && len(activeDls) == 0 {
			break
		}
		time.Sleep(2 * time.Second)
		if time.Since(startTime) > 300*time.Second {
			break
		}
	}
}

func findLatestUncleanedLocalEpisode(podDir, podTitle string, quiet bool) (string, bool) {
	mp3s := findMP3Files(podDir)
	if len(mp3s) == 0 {
		if !quiet {
			fmt.Printf("All episodes for podcast %s already have ads removed.\n", displayName(podTitle))
		}
		return "", false
	}

	sort.Slice(mp3s, func(i, j int) bool {
		ti := getEpisodePublicationTime(mp3s[i])
		tj := getEpisodePublicationTime(mp3s[j])
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return mp3s[i] > mp3s[j]
	})

	for _, mp3 := range mp3s {
		if !isEpisodeClean(mp3) {
			return mp3, true
		}
	}

	if !quiet {
		fmt.Printf("All episodes for podcast %s already have ads removed.\n", displayName(podTitle))
	}
	return "", false
}

func processSingleQueuedTarget(podDir, targetAudioPath, action string, cli CLIOptions, config Config) error {
	epFilename := queueItemFilename(podDir, targetAudioPath)
	targetHost := resolveRemoteProcessingTargetHost(cli, config)

	if targetHost != "" {
		if err := runRemotePush(&config, []string{targetAudioPath}, targetHost, nil, cli.Priority, cli.Quiet, cli.Verbose); err != nil {
			return fmt.Errorf("error pushing episode to remote: %w", err)
		}
		if err := pollAndPullRemoteEpisode(config, cli, targetHost, targetAudioPath); err != nil {
			return err
		}
	} else {
		executeLocalBatchProcessing([]string{targetAudioPath}, cli, config, action)
	}

	removeEpisodeFromQueueFile(podDir, epFilename)
	removeEpisodeFromQueueFile(podDir, filepath.Base(targetAudioPath))
	return nil
}

func queueItemFilename(podDir, audioPath string) string {
	if rel, err := filepath.Rel(podDir, audioPath); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(audioPath)
}

func pollAndPullRemoteEpisode(config Config, cli CLIOptions, targetHost, targetAudioPath string) error {
	if !cli.Quiet {
		fmt.Printf("Waiting for remote processing on %s to complete...\n", targetHost)
	}
	startTime := time.Now()
	for {
		_ = runRemotePull(&config, targetHost, nil, true, cli.Verbose)
		if isEpisodeClean(targetAudioPath) {
			break
		}
		time.Sleep(3 * time.Second)
		if time.Since(startTime) > 3600*time.Second {
			return fmt.Errorf("timeout waiting for remote processing of %s", filepath.Base(targetAudioPath))
		}
	}

	printRemoteEpisodeSummary(targetAudioPath, cli.Quiet)
	return nil
}

func printRemoteEpisodeSummary(targetAudioPath string, quiet bool) {
	if quiet {
		return
	}
	origDur, cleanDur := getEpisodeDurations(targetAudioPath, getOrCreateEpisodeStatus(targetAudioPath))
	cutDur := origDur - cleanDur
	pct := 0.0
	if origDur > 0 {
		pct = (cutDur / origDur) * 100
	}
	fmt.Printf("\nCompleted ad removal for %s\n", filepath.Base(targetAudioPath))
	fmt.Printf("Original duration: %s\n", formatTime(origDur))
	fmt.Printf("Cut duration:      %s (%.1f%% trimmed)\n", formatTime(cutDur), pct)
	fmt.Printf("Cleaned duration:  %s\n", formatTime(cleanDur))
}
