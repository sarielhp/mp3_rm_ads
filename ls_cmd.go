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

type lsEpisodeItem struct {
	path           string
	podcastDir     string
	podcastTitle   string
	podcastShortID string
	episodeName    string
	modTime        time.Time
	sizeBytes      int64
	origDuration   float64
	cleanDuration  float64
	statusStr      string
	statusColor    string
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
		return listLatestEpisodes(podcastsDir, limit, cli.Quiet, cli.Verbose)
	}

	if len(args) == 0 {
		return listLatestEpisodes(podcastsDir, limit, cli.Quiet, cli.Verbose)
	}

	target := args[0]
	if n, err := strconv.Atoi(target); err == nil && n > 0 && !podcastExistsByIndexOrID(podcastsDir, target) {
		return listLatestEpisodes(podcastsDir, n, cli.Quiet, cli.Verbose)
	}

	podDir, title, found := resolvePodcastDirByIDOrName(podcastsDir, target)
	if !found {
		if fi, err := os.Stat(target); err == nil && fi.IsDir() {
			podDir = target
			title = filepath.Base(target)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("podcast matching %q not found in %s", target, podcastsDir)
	}

	shortID := getOrSetPodcastShortID(podDir, title)
	return listSinglePodcastEpisodes(podDir, title, shortID, cli.Quiet, cli.Verbose)
}

func podcastExistsByIndexOrID(podcastsDir, query string) bool {
	_, _, found := resolvePodcastDirByIDOrName(podcastsDir, query)
	return found
}

func listLatestEpisodes(podcastsDir string, limit int, quiet, verbose bool) error {
	podEntries := scanPodcastDirs(podcastsDir)
	podIDMap := make(map[string]string)
	podTitleMap := make(map[string]string)
	for _, p := range podEntries {
		podIDMap[p.dir] = p.shortID
		podTitleMap[p.dir] = p.title
	}

	allMp3s := findMP3Files(podcastsDir)
	if len(allMp3s) == 0 {
		if !quiet {
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

	if quiet {
		for _, item := range latest {
			fmt.Println(item.path)
		}
		return nil
	}

	printLatestEpisodesTable(latest, limit)
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
		podTitle := podTitleMap[podDir]
		if podTitle == "" {
			podTitle = filepath.Base(podDir)
		}
		shortID := podIDMap[podDir]
		if shortID == "" {
			shortID = generatePodcastShortID(podTitle)
		}

		epName := strings.TrimSuffix(filepath.Base(mp3), filepath.Ext(mp3))
		statusStr, statusColor := getEpisodeStatusLabel(mp3)

		items = append(items, lsEpisodeItem{
			path:           mp3,
			podcastDir:     podDir,
			podcastTitle:   podTitle,
			podcastShortID: shortID,
			episodeName:    epName,
			modTime:        fi.ModTime(),
			sizeBytes:      fi.Size(),
			statusStr:      statusStr,
			statusColor:    statusColor,
		})
	}
	return items
}

func printLatestEpisodesTable(latest []lsEpisodeItem, limit int) {
	fmt.Printf("\nLatest %d Episodes Across All Podcasts:\n", limit)
	fmt.Printf("%s\n", strings.Repeat("=", 95))
	fmt.Printf("  %-16s │ %-6s │ %-24s │ %-18s │ %s\n", "Date / Time", "ID", "Podcast", "Status", "Episode Title")
	fmt.Printf("  %-16s ┼ %-6s ┼ %-24s ┼ %-18s ┼ %s\n", strings.Repeat("─", 16), strings.Repeat("─", 6), strings.Repeat("─", 24), strings.Repeat("─", 18), strings.Repeat("─", 22))

	for _, item := range latest {
		dStr := item.modTime.Format("2006-01-02 15:04")
		pName := displayName(item.podcastTitle)
		r := []rune(pName)
		if len(r) > 24 {
			pName = string(r[:21]) + "..."
		}
		coloredStatus := item.statusStr
		if item.statusColor == "green" {
			coloredStatus = boldGreen(item.statusStr)
		} else if item.statusColor == "yellow" {
			coloredStatus = boldYellow(item.statusStr)
		} else if item.statusColor == "cyan" {
			coloredStatus = bold(item.statusStr)
		}

		fmt.Printf("  %-16s │ %-6s │ %-24s │ %-18s │ %s\n", dStr, item.podcastShortID, pName, coloredStatus, item.episodeName)
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 95))
}

func listSinglePodcastEpisodes(podDir, title, shortID string, quiet, verbose bool) error {
	mp3s := findMP3Files(podDir)
	if len(mp3s) == 0 {
		if !quiet {
			fmt.Printf("No audio files found for %s (%s).\n", title, shortID)
		}
		return nil
	}

	items := collectSinglePodcastEpisodes(mp3s, podDir, title, shortID)
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})

	if quiet {
		for _, item := range items {
			fmt.Println(item.path)
		}
		return nil
	}

	printSinglePodcastEpisodesTable(items, title, shortID)
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
		st := getOrCreateEpisodeStatus(mp3)
		statusStr, statusColor := getEpisodeStatusLabel(mp3)
		origDur, cleanDur := getEpisodeDurations(mp3, st)

		items = append(items, lsEpisodeItem{
			path:           mp3,
			podcastDir:     podDir,
			podcastTitle:   title,
			podcastShortID: shortID,
			episodeName:    epName,
			modTime:        fi.ModTime(),
			sizeBytes:      fi.Size(),
			origDuration:   origDur,
			cleanDuration:  cleanDur,
			statusStr:      statusStr,
			statusColor:    statusColor,
		})
	}
	return items
}

func printSinglePodcastEpisodesTable(items []lsEpisodeItem, title, shortID string) {
	fmt.Printf("\nEpisodes for %s [%s] (%d total):\n", bold(title), shortID, len(items))
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("  %-3s │ %-8s │ %-8s │ %-7s │ %s\n", "#", "Orig", "Clean", "Status", "Episode Title")
	fmt.Printf("  %-3s ┼ %-8s ┼ %-8s ┼ %-7s ┼ %s\n", strings.Repeat("─", 3), strings.Repeat("─", 8), strings.Repeat("─", 8), strings.Repeat("─", 7), strings.Repeat("─", 40))

	for idx, item := range items {
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

		shortStatus := item.statusStr
		if shortStatus == "Needs Ad Removal" {
			shortStatus = "NeedsAd"
		} else if shortStatus == "Queued Remote" {
			shortStatus = "Queued"
		} else if shortStatus == "In Progress" {
			shortStatus = "Active"
		}

		coloredStatus := shortStatus
		if item.statusColor == "green" {
			coloredStatus = boldGreen(shortStatus)
		} else if item.statusColor == "yellow" {
			coloredStatus = boldYellow(shortStatus)
		} else if item.statusColor == "cyan" {
			coloredStatus = bold(shortStatus)
		}

		t := item.episodeName
		r := []rune(t)
		if len(r) > 40 {
			t = string(r[:37]) + "..."
		}

		fmt.Printf("  %-3d │ %-8s │ %-8s │ %-7s │ %s\n", idx+1, origStr, cleanStr, coloredStatus, t)
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))
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
