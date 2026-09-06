package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type lsPodcastItem struct {
	ShortID        string `json:"id"`
	Title          string `json:"title"`
	EpisodeCount   int    `json:"episode_count"`
	CleanCount     int    `json:"clean_count"`
	DownloadPolicy string `json:"download_policy"`
	AdRemoval      string `json:"ad_removal"`
	Retention      string `json:"retention"`
	LastEpisode    string `json:"last_episode"`
}

type lsEpisodeItem struct {
	path           string
	podcastDir     string
	podcastTitle   string
	podcastShortID string
	episodeShortID string
	episodeName    string
	modTime        time.Time
	pubTime        time.Time
	sizeBytes      int64
	origDuration   float64
	cleanDuration  float64
	hasTranscript  bool
	statusStr      string
	statusColor    string
}

type lsEpisodeJSON struct {
	ID                  string  `json:"id"`
	PodcastID           string  `json:"podcast_id"`
	PodcastTitle        string  `json:"podcast_title,omitempty"`
	Title               string  `json:"title"`
	Date                string  `json:"date"`
	OriginalDurationSec float64 `json:"original_duration_sec,omitempty"`
	CleanDurationSec    float64 `json:"clean_duration_sec,omitempty"`
	DurationSec         float64 `json:"duration_sec,omitempty"`
	Status              string  `json:"status"`
	HasTranscript       bool    `json:"has_transcript"`
	Path                string  `json:"path"`
}

func runLsCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	limit := cli.Count
	if limit <= 0 {
		limit = 10
	}

	args := cli.Args
	if cli.LsSubcmd == "latest" || (len(args) > 0 && args[0] == "latest") {
		if len(args) > 0 && args[0] == "latest" {
			args = args[1:]
		}
		if len(args) > 0 {
			if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
				limit = n
			}
		}
		return listLatestEpisodes(podcastsDir, limit, cli)
	}

	if len(args) == 0 || (len(args) == 1 && (args[0] == "podcasts" || args[0] == "all")) {
		return listAllPodcasts(podcastsDir, cli)
	}

	target := args[0]
	if n, err := strconv.Atoi(target); err == nil && n > 0 && !podcastExistsByIndexOrID(podcastsDir, target) {
		return listLatestEpisodes(podcastsDir, n, cli)
	}

	resolved, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}

	if resolved.IsPodcast() {
		return listSinglePodcastEpisodes(resolved.Podcast.Dir, resolved.Podcast.Title, resolved.Podcast.ShortID, cli)
	}

	if resolved.IsEpisode() {
		ep := resolved.Episode
		return listSinglePodcastEpisodes(ep.PodcastDir, ep.PodcastTitle, ep.PodcastShortID, cli)
	}

	return fmt.Errorf("could not resolve %q", target)
}

func podcastExistsByIndexOrID(podcastsDir, query string) bool {
	res, err := resolveAnyID(podcastsDir, query)
	return err == nil && res != nil
}

func listAllPodcasts(podcastsDir string, cli CLIOptions) error {
	entries := scanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcasts found.")
		}
		return nil
	}

	items := collectPodcastListItems(entries)

	if cli.JSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if cli.Quiet {
		for _, item := range items {
			fmt.Println(item.ShortID)
		}
		return nil
	}

	printPodcastsTable(items)
	return nil
}

func collectPodcastListItems(entries []podcastDirEntry) []lsPodcastItem {
	var items []lsPodcastItem
	for _, p := range entries {
		mp3s := findMP3Files(p.dir)
		cfg := loadPodcastConfig(p.dir)
		cleanCount := 0
		var newestTime time.Time

		for _, mp3 := range mp3s {
			if isEpisodeClean(mp3) {
				cleanCount++
			}
			pt := getEpisodePublicationTime(mp3)
			if pt.After(newestTime) {
				newestTime = pt
			}
		}

		retention := "-"
		if cfg.AutoCleanupDays > 0 {
			retention = fmt.Sprintf("%dd", cfg.AutoCleanupDays)
		}

		lastEpStr := "-"
		if !newestTime.IsZero() {
			lastEpStr = newestTime.Format("2006-01-02")
		}

		items = append(items, lsPodcastItem{
			ShortID:        p.shortID,
			Title:          p.title,
			EpisodeCount:   len(mp3s),
			CleanCount:     cleanCount,
			DownloadPolicy: cfg.DownloadPolicy,
			AdRemoval:      cfg.AdRemoval,
			Retention:      retention,
			LastEpisode:    lastEpStr,
		})
	}
	return items
}

func printPodcastsTable(items []lsPodcastItem) {
	fmt.Printf("\nPodcasts in Library (%d total):\n", len(items))
	fmt.Printf("%s\n", strings.Repeat("=", 96))
	fmt.Printf("  %-5s │ %-26s │ %-8s │ %-5s │ %-15s │ %-12s │ %-9s │ %s\n",
		"ID", "Podcast Name", "Episodes", "Clean", "DL Policy", "Ads Policy", "Retention", "Last Ep")
	fmt.Printf("  %-5s ┼ %-26s ┼ %-8s ┼ %-5s ┼ %-15s ┼ %-12s ┼ %-9s ┼ %s\n",
		strings.Repeat("─", 5), strings.Repeat("─", 26), strings.Repeat("─", 8),
		strings.Repeat("─", 5), strings.Repeat("─", 15), strings.Repeat("─", 12),
		strings.Repeat("─", 9), strings.Repeat("─", 10))

	for _, item := range items {
		pName := truncateDisplayName(item.Title, 26)

		dlBadge := downloadPolicyBadge(item.DownloadPolicy, 3)
		adBadge := adRemovalModeBadge(item.AdRemoval)

		fmt.Printf("  %-5s │ %-26s │ %-8d │ %-5d │ %-15s │ %-12s │ %-9s │ %s\n",
			boldCyan(item.ShortID), pName, item.EpisodeCount, item.CleanCount, dlBadge, adBadge, item.Retention, item.LastEpisode)
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 96))
}

func listLatestEpisodes(podcastsDir string, limit int, cli CLIOptions) error {
	podEntries := scanPodcastDirs(podcastsDir)
	podIDMap := make(map[string]string)
	podTitleMap := make(map[string]string)
	for _, p := range podEntries {
		podIDMap[p.dir] = p.shortID
		podTitleMap[p.dir] = p.title
	}

	allMp3s := findMP3Files(podcastsDir)
	if len(allMp3s) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcast audio files (.mp3) found.")
		}
		return nil
	}

	items := collectLatestEpisodeItems(allMp3s, podTitleMap, podIDMap)
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})

	if limit > len(items) {
		limit = len(items)
	}
	latest := items[:limit]

	if cli.JSON {
		return outputLatestEpisodesJSON(latest)
	}

	if cli.Quiet {
		for _, item := range latest {
			fmt.Println(item.path)
		}
		return nil
	}

	printLatestEpisodesTable(latest, limit)
	return nil
}

func outputLatestEpisodesJSON(items []lsEpisodeItem) error {
	var jsonList []lsEpisodeJSON
	for _, it := range items {
		jsonList = append(jsonList, lsEpisodeJSON{
			ID:            it.episodeShortID,
			PodcastID:     it.podcastShortID,
			PodcastTitle:  it.podcastTitle,
			Title:         it.episodeName,
			Date:          it.modTime.Format("2006-01-02 15:04"),
			Status:        it.statusStr,
			DurationSec:   it.origDuration,
			HasTranscript: it.hasTranscript,
			Path:          it.path,
		})
	}
	data, err := json.MarshalIndent(jsonList, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func collectLatestEpisodeItems(allMp3s []string, podTitleMap, podIDMap map[string]string) []lsEpisodeItem {
	var items []lsEpisodeItem
	for _, mp3 := range allMp3s {
		fi, err := os.Stat(mp3)
		if err != nil {
			continue
		}
		podDir := filepath.Dir(mp3)
		epName := strings.TrimSuffix(filepath.Base(mp3), filepath.Ext(mp3))
		if strings.EqualFold(filepath.Base(mp3), "podcast.mp3") {
			epName = filepath.Base(podDir)
			podDir = filepath.Dir(podDir)
		}
		podTitle := podTitleMap[podDir]
		if podTitle == "" {
			podTitle = filepath.Base(podDir)
		}
		shortID := podIDMap[podDir]
		if shortID == "" {
			shortID = generatePodcastShortID(podTitle)
		}

		epShortID := getOrSetEpisodeShortID(podDir, shortID, mp3)
		statusStr, statusColor := getEpisodeStatusLabel(mp3)
		st := getOrCreateEpisodeStatus(mp3)
		origDur, cleanDur := getEpisodeDurations(mp3, st)
		txPath := stripExt(mp3) + ".transcript.json"
		_, errTx := os.Stat(txPath)

		items = append(items, lsEpisodeItem{
			path:           mp3,
			podcastDir:     podDir,
			podcastTitle:   podTitle,
			podcastShortID: shortID,
			episodeShortID: epShortID,
			episodeName:    epName,
			modTime:        fi.ModTime(),
			sizeBytes:      fi.Size(),
			origDuration:   origDur,
			cleanDuration:  cleanDur,
			hasTranscript:  errTx == nil,
			statusStr:      statusStr,
			statusColor:    statusColor,
		})
	}
	return items
}

func printLatestEpisodesTable(latest []lsEpisodeItem, limit int) {
	fmt.Printf("\nLatest %d Episodes Across All Podcasts:\n", limit)
	fmt.Printf("%s\n", strings.Repeat("=", 105))
	fmt.Printf("  %-16s │ %-5s │ %-6s │ %-20s │ %-10s │ %-8s │ %s\n",
		"Date / Time", "PodID", "EpID", "Podcast", "Status", "Dur", "Episode Title")
	fmt.Printf("  %-16s ┼ %-5s ┼ %-6s ┼ %-20s ┼ %-10s ┼ %-8s ┼ %s\n",
		strings.Repeat("─", 16), strings.Repeat("─", 5), strings.Repeat("─", 6),
		strings.Repeat("─", 20), strings.Repeat("─", 10), strings.Repeat("─", 8), strings.Repeat("─", 26))

	for _, item := range latest {
		dStr := item.modTime.Format("2006-01-02 15:04")
		pName := truncateDisplayName(item.podcastTitle, 20)
		shortStatus := formatShortStatus(item.statusStr)
		coloredStatus := shortStatus
		if item.statusColor == "green" {
			coloredStatus = boldGreen(shortStatus)
		} else if item.statusColor == "yellow" {
			coloredStatus = boldYellow(shortStatus)
		} else if item.statusColor == "cyan" {
			coloredStatus = bold(shortStatus)
		}

		durStr := "-"
		if item.origDuration > 0 {
			durStr = formatClock(item.origDuration)
		}

		fmt.Printf("  %-16s │ %-5s │ %-6s │ %-20s │ %-10s │ %-8s │ %s\n",
			dStr, item.podcastShortID, boldCyan(item.episodeShortID), pName, coloredStatus, durStr, truncateDisplayName(item.episodeName, 30))
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 105))
}

func listSinglePodcastEpisodes(podDir, title, shortID string, cli CLIOptions) error {
	mp3s := findMP3Files(podDir)
	if len(mp3s) == 0 {
		if !cli.Quiet {
			fmt.Printf("No audio files found for %s (%s).\n", title, shortID)
		}
		return nil
	}

	items := collectSinglePodcastEpisodes(mp3s, podDir, title, shortID)
	sort.Slice(items, func(i, j int) bool {
		return items[i].pubTime.After(items[j].pubTime)
	})

	if cli.JSON {
		return outputSinglePodcastEpisodesJSON(items)
	}

	if cli.Quiet {
		for _, item := range items {
			fmt.Println(item.path)
		}
		return nil
	}

	printSinglePodcastEpisodesTable(items, title, shortID)
	return nil
}

func outputSinglePodcastEpisodesJSON(items []lsEpisodeItem) error {
	var jsonList []lsEpisodeJSON
	for _, it := range items {
		jsonList = append(jsonList, lsEpisodeJSON{
			ID:                  it.episodeShortID,
			PodcastID:           it.podcastShortID,
			Title:               it.episodeName,
			Date:                it.pubTime.Format("2006-01-02"),
			OriginalDurationSec: it.origDuration,
			CleanDurationSec:    it.cleanDuration,
			Status:              it.statusStr,
			HasTranscript:       it.hasTranscript,
			Path:                it.path,
		})
	}
	data, err := json.MarshalIndent(jsonList, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func collectSinglePodcastEpisodes(mp3s []string, podDir, title, shortID string) []lsEpisodeItem {
	var items []lsEpisodeItem
	for _, mp3 := range mp3s {
		fi, err := os.Stat(mp3)
		if err != nil {
			continue
		}
		epName := strings.TrimSuffix(filepath.Base(mp3), filepath.Ext(mp3))
		if strings.EqualFold(filepath.Base(mp3), "podcast.mp3") {
			epName = filepath.Base(filepath.Dir(mp3))
		}
		st := getOrCreateEpisodeStatus(mp3)
		statusStr, statusColor := getEpisodeStatusLabel(mp3)
		origDur, cleanDur := getEpisodeDurations(mp3, st)
		epShortID := getOrSetEpisodeShortID(podDir, shortID, mp3)

		txPath := stripExt(mp3) + ".transcript.json"
		_, errTx := os.Stat(txPath)

		pubTime := getEpisodePublicationTime(mp3)
		if pubTime.IsZero() {
			pubTime = fi.ModTime()
		}

		items = append(items, lsEpisodeItem{
			path:           mp3,
			podcastDir:     podDir,
			podcastTitle:   title,
			podcastShortID: shortID,
			episodeShortID: epShortID,
			episodeName:    epName,
			modTime:        fi.ModTime(),
			pubTime:        pubTime,
			sizeBytes:      fi.Size(),
			origDuration:   origDur,
			cleanDuration:  cleanDur,
			hasTranscript:  errTx == nil,
			statusStr:      statusStr,
			statusColor:    statusColor,
		})
	}
	return items
}

func printSinglePodcastEpisodesTable(items []lsEpisodeItem, title, shortID string) {
	fmt.Printf("\nEpisodes for %s [%s] (%d total):\n", bold(displayName(title)), shortID, len(items))
	fmt.Printf("%s\n", strings.Repeat("=", 99))
	fmt.Printf("  %-6s │ %-10s │ %-8s │ %-8s │ %-10s │ %-3s │ %s\n",
		"ID", "Date", "Orig", "Clean", "Status", "Tx", "Episode Title")
	fmt.Printf("  %-6s ┼ %-10s ┼ %-8s ┼ %-8s ┼ %-10s ┼ %-3s ┼ %s\n",
		strings.Repeat("─", 6), strings.Repeat("─", 10), strings.Repeat("─", 8),
		strings.Repeat("─", 8), strings.Repeat("─", 10), strings.Repeat("─", 3), strings.Repeat("─", 38))

	for _, item := range items {
		origStr := formatClock(item.origDuration)
		if item.origDuration <= 0 {
			origStr = "-"
		}
		cleanStr := "-"
		if item.statusStr == "Clean" {
			if item.cleanDuration > 0 {
				cleanStr = formatClock(item.cleanDuration)
			} else if item.origDuration > 0 {
				cleanStr = formatClock(item.origDuration)
			}
		}

		shortStatus := formatShortStatus(item.statusStr)
		coloredStatus := shortStatus
		if item.statusColor == "green" {
			coloredStatus = boldGreen(shortStatus)
		} else if item.statusColor == "yellow" {
			coloredStatus = boldYellow(shortStatus)
		} else if item.statusColor == "cyan" {
			coloredStatus = bold(shortStatus)
		}

		txFlag := "-"
		if item.hasTranscript {
			txFlag = boldGreen("✓")
		}

		t := truncateDisplayName(item.episodeName, 38)
		dateStr := item.pubTime.Format("2006-01-02")

		fmt.Printf("  %-6s │ %-10s │ %-8s │ %-8s │ %-10s │ %-3s │ %s\n",
			boldCyan(item.episodeShortID), dateStr, origStr, cleanStr, coloredStatus, txFlag, t)
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 99))
}

func formatShortStatus(status string) string {
	switch status {
	case "Needs Ad Removal", "NeedsAd":
		return "✂ NeedsAd"
	case "Queued Remote", "Queued":
		return "⏳ Queued"
	case "In Progress", "Active":
		return "⚡ Active"
	case "Clean":
		return "✓ Clean"
	default:
		return status
	}
}

func isEpisodeClean(mp3Path string) bool {
	st := getOrCreateEpisodeStatus(mp3Path)
	return st.Status == StateDone || st.Status == StateCopiedBack || isEpisodeCompleted(mp3Path)
}

func getEpisodeDurations(mp3Path string, st *EpisodeStatusFile) (float64, float64) {
	origDur := 0.0
	cleanDur := 0.0
	if st != nil {
		origDur = st.Original.DurationSec
		cleanDur = st.Cleaned.DurationSec
	}
	if origDur == 0 {
		cutsFile := stripExt(mp3Path) + ".cuts.json"
		if data, err := os.ReadFile(cutsFile); err == nil {
			var cd CutsData
			if json.Unmarshal(data, &cd) == nil && cd.OriginalDurationSec > 0 {
				origDur = cd.OriginalDurationSec
				cleanDur = cd.OriginalDurationSec - cd.TotalCutDurationSec
			}
		}
	}
	if origDur == 0 {
		origDur = getAudioDuration(mp3Path)
		if st != nil && (st.Status == StateDone || st.Status == StateCopiedBack || isEpisodeCompleted(mp3Path)) {
			cleanDur = origDur
		}
	}
	return origDur, cleanDur
}

func getEpisodeStatusLabel(mp3Path string) (string, string) {
	st := getOrCreateEpisodeStatus(mp3Path)
	if st.Status == StateDone || st.Status == StateCopiedBack || isEpisodeCompleted(mp3Path) {
		return "Clean", "green"
	}
	if st.Status == StateQueuedRemote {
		return "Queued Remote", "cyan"
	}
	if st.Status == StateTranscribingRemotely || st.Status == StateCuttingRemotely || st.Status == StateTranscribingLocally || st.Status == StateCuttingLocally {
		return "In Progress", "yellow"
	}
	return "Needs Ad Removal", "yellow"
}
