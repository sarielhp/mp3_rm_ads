package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type queueEpisodeItem struct {
	PodcastID  string `json:"podcast_id"`
	EpisodeID  string `json:"episode_id"`
	Title      string `json:"title"`
	AudioPath  string `json:"audio_path"`
	PodcastDir string `json:"podcast_dir"`
	Filename   string `json:"filename"`
}

func runQueueCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	subcmd := cli.QueueSubcmd
	args := cli.Args
	if subcmd == "" && len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "list":
			subcmd = "list"
			args = args[1:]
		case "add":
			subcmd = "add"
			args = args[1:]
		case "remove":
			subcmd = "remove"
			args = args[1:]
		case "clear":
			subcmd = "clear"
			args = args[1:]
		case "run":
			subcmd = "run"
			args = args[1:]
		default:
			subcmd = "list"
		}
	} else if subcmd == "" {
		subcmd = "list"
	}

	switch subcmd {
	case "list":
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return handleQueueList(podcastsDir, target, cli)
	case "add":
		if len(args) == 0 {
			return fmt.Errorf("missing target ID(s) to add to queue")
		}
		return handleQueueAdd(podcastsDir, args)
	case "remove":
		if len(args) == 0 {
			return fmt.Errorf("missing target ID(s) to remove from queue")
		}
		return handleQueueRemove(podcastsDir, args)
	case "clear":
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return handleQueueClear(podcastsDir, target)
	case "run":
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return handleQueueRun(cfg, cli, target)
	default:
		return fmt.Errorf("unknown queue action %q (use list, add, remove, clear, or run)", subcmd)
	}
}

func handleQueueList(podcastsDir, target string, cli CLIOptions) error {
	var entries []podcastDirEntry
	if target != "" {
		res, err := resolveAnyID(podcastsDir, target)
		if err != nil {
			return err
		}
		if res.IsPodcast() {
			entries = []podcastDirEntry{{
				dir:        res.Podcast.Dir,
				folderName: res.Podcast.FolderName,
				title:      res.Podcast.Title,
				shortID:    res.Podcast.ShortID,
			}}
		} else if res.IsEpisode() {
			entries = []podcastDirEntry{{
				dir:        res.Episode.PodcastDir,
				folderName: filepath.Base(res.Episode.PodcastDir),
				title:      res.Episode.PodcastTitle,
				shortID:    res.Episode.PodcastShortID,
			}}
		}
	} else {
		entries = scanPodcastDirs(podcastsDir)
	}

	var allItems []queueEpisodeItem
	for _, p := range entries {
		items := collectPodcastQueueItems(p)
		allItems = append(allItems, items...)
	}

	if cli.JSON {
		data, err := json.MarshalIndent(allItems, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if cli.Quiet {
		for _, it := range allItems {
			fmt.Println(it.AudioPath)
		}
		return nil
	}

	printQueueTable(allItems)
	return nil
}

func collectPodcastQueueItems(p podcastDirEntry) []queueEpisodeItem {
	qFile := filepath.Join(p.dir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err != nil {
		return nil
	}

	var filenames []string
	if err := json.Unmarshal(data, &filenames); err != nil || len(filenames) == 0 {
		return nil
	}

	var list []queueEpisodeItem
	for _, fn := range filenames {
		mp3Path := filepath.Join(p.dir, fn)
		epID := getOrSetEpisodeShortID(p.dir, p.shortID, mp3Path)
		title := episodeTitleFromPath(mp3Path)
		if title == "" {
			title = stripExt(fn)
		}

		list = append(list, queueEpisodeItem{
			PodcastID:  p.shortID,
			EpisodeID:  epID,
			Title:      title,
			AudioPath:  mp3Path,
			PodcastDir: p.dir,
			Filename:   fn,
		})
	}
	return list
}

func printQueueTable(items []queueEpisodeItem) {
	if len(items) == 0 {
		fmt.Println("AdR queue is currently empty.")
		return
	}

	fmt.Printf("\nAdR Queue (%d queued):\n", len(items))
	titleWidth := 30
	fileWidth := 30
	cols := queueTableColumns(titleWidth, fileWidth)

	fmt.Println(renderTableTop(cols))
	fmt.Println(renderTableHeader(cols))
	fmt.Println(renderTableDivider(cols))

	for _, it := range items {
		t := truncateDisplayName(it.Title, titleWidth)
		fn := truncateDisplayName(it.Filename, fileWidth)
		cells := []string{it.PodcastID, boldCyan(it.EpisodeID), t, fn}
		fmt.Println(renderTableRow(cells, cols))
	}

	fmt.Println(renderTableBottom(cols))
	fmt.Println()
}

func handleQueueAdd(podcastsDir string, targets []string) error {
	for _, query := range targets {
		res, err := resolveAnyID(podcastsDir, query)
		if err != nil {
			return fmt.Errorf("failed to resolve %q: %w", query, err)
		}

		if res.IsEpisode() {
			ep := res.Episode
			added := addEpisodeToQueueFile(ep.PodcastDir, ep.Filename)
			if added {
				fmt.Printf("Added to queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			} else {
				fmt.Printf("Already in queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			}
		} else if res.IsPodcast() {
			pod := res.Podcast
			count := addPodcastEpisodesToQueue(pod.Dir)
			fmt.Printf("Added %d uncleaned episode(s) of %s [%s] to queue\n",
				count, bold(displayName(pod.Title)), boldCyan(pod.ShortID))
		}
	}
	return nil
}

func addEpisodeToQueueFile(podDir, filename string) bool {
	added := false
	_ = updateQueue(podDir, func(entries []string) []string {
		for _, e := range entries {
			if strings.EqualFold(e, filename) {
				return entries
			}
		}
		added = true
		return append(entries, filename)
	})
	return added
}

func addPodcastEpisodesToQueue(podDir string) int {
	mp3s := findMP3Files(podDir)
	var candidates []string
	for _, mp3 := range mp3s {
		if !isEpisodeClean(mp3) {
			candidates = append(candidates, filepath.Base(mp3))
		}
	}
	if len(candidates) == 0 {
		return 0
	}

	addedCount := 0
	_ = updateQueue(podDir, func(entries []string) []string {
		existing := make(map[string]bool)
		for _, e := range entries {
			existing[strings.ToLower(e)] = true
		}
		for _, fn := range candidates {
			if !existing[strings.ToLower(fn)] {
				entries = append(entries, fn)
				existing[strings.ToLower(fn)] = true
				addedCount++
			}
		}
		return entries
	})
	return addedCount
}

func handleQueueRemove(podcastsDir string, targets []string) error {
	for _, query := range targets {
		res, err := resolveAnyID(podcastsDir, query)
		if err != nil {
			return fmt.Errorf("failed to resolve %q: %w", query, err)
		}

		if res.IsEpisode() {
			ep := res.Episode
			removed := removeEpisodeFromQueueFile(ep.PodcastDir, ep.Filename)
			if removed {
				fmt.Printf("Removed from queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			} else {
				fmt.Printf("Not found in queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			}
		} else if res.IsPodcast() {
			pod := res.Podcast
			_ = saveQueue(pod.Dir, []string{})
			fmt.Printf("Cleared queue for %s [%s]\n", bold(displayName(pod.Title)), boldCyan(pod.ShortID))
		}
	}
	return nil
}

func removeEpisodeFromQueueFile(podDir, filename string) bool {
	found := false
	_ = updateQueue(podDir, func(entries []string) []string {
		var filtered []string
		for _, e := range entries {
			if strings.EqualFold(e, filename) {
				found = true
			} else {
				filtered = append(filtered, e)
			}
		}
		if found {
			return filtered
		}
		return entries
	})
	return found
}

func handleQueueClear(podcastsDir, target string) error {
	if target != "" {
		res, err := resolveAnyID(podcastsDir, target)
		if err != nil {
			return err
		}
		if res.IsPodcast() {
			_ = saveQueue(res.Podcast.Dir, []string{})
			fmt.Printf("Queue cleared for %s [%s]\n", bold(displayName(res.Podcast.Title)), boldCyan(res.Podcast.ShortID))
			return nil
		} else if res.IsEpisode() {
			removeEpisodeFromQueueFile(res.Episode.PodcastDir, res.Episode.Filename)
			fmt.Printf("Removed [%s] from queue\n", boldCyan(res.Episode.ShortID))
			return nil
		}
	}

	entries := scanPodcastDirs(podcastsDir)
	clearedCount := 0
	for _, p := range entries {
		qFile := filepath.Join(p.dir, "queue.json")
		if _, err := os.Stat(qFile); err == nil {
			_ = saveQueue(p.dir, []string{})
			clearedCount++
		}
	}
	fmt.Printf("Queue cleared across %d podcast(s).\n", clearedCount)
	return nil
}

func handleQueueRun(cfg Config, cli CLIOptions, target string) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	items, err := resolveQueueRunItems(podcastsDir, target)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if !cli.Quiet {
			fmt.Println("AdR queue is currently empty.")
		}
		return nil
	}

	if !cli.Quiet {
		fmt.Printf("Found %d episode(s) in AdR queue.\n", len(items))
	}

	if cli.DryRun {
		for _, it := range items {
			fmt.Printf("[dry-run] Would process ad removal for: [%s] %s (%s)\n", it.EpisodeID, displayName(it.Title), it.Filename)
		}
		return nil
	}

	applyForceCLIOptions(&cli)
	return executeQueueRun(items, cli, cfg)
}

func resolveQueueRunItems(podcastsDir, target string) ([]queueEpisodeItem, error) {
	if target != "" {
		return resolveTargetQueueItems(podcastsDir, target)
	}
	var allItems []queueEpisodeItem
	entries := scanPodcastDirs(podcastsDir)
	for _, p := range entries {
		items := collectPodcastQueueItems(p)
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

func resolveTargetQueueItems(podcastsDir, target string) ([]queueEpisodeItem, error) {
	res, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return nil, err
	}
	if res.IsPodcast() {
		p := podcastDirEntry{
			dir:        res.Podcast.Dir,
			folderName: res.Podcast.FolderName,
			title:      res.Podcast.Title,
			shortID:    res.Podcast.ShortID,
		}
		return collectPodcastQueueItems(p), nil
	}
	if res.IsEpisode() {
		p := podcastDirEntry{
			dir:        res.Episode.PodcastDir,
			folderName: filepath.Base(res.Episode.PodcastDir),
			title:      res.Episode.PodcastTitle,
			shortID:    res.Episode.PodcastShortID,
		}
		items := collectPodcastQueueItems(p)
		var matched []queueEpisodeItem
		for _, it := range items {
			if strings.EqualFold(it.Filename, res.Episode.Filename) {
				matched = append(matched, it)
			}
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("episode %q [%s] is not in the AdR queue", res.Episode.Filename, res.Episode.ShortID)
		}
		return matched, nil
	}
	return nil, fmt.Errorf("unrecognized target %q", target)
}

func executeQueueRun(items []queueEpisodeItem, cli CLIOptions, cfg Config) error {
	total := len(items)
	processedCount := 0
	var failedEpisodes []string

	for i, it := range items {
		if !cli.Quiet {
			fmt.Printf("\n[%d/%d] Processing queued episode: %s [%s]\n", i+1, total, displayName(it.Title), boldCyan(it.EpisodeID))
		}

		if !fileExists(it.AudioPath) {
			if !cli.Quiet {
				fmt.Printf("Audio file not found on disk: %s (removing from queue)\n", it.Filename)
			}
			removeEpisodeFromQueueFile(it.PodcastDir, it.Filename)
			continue
		}

		if !cli.ForceTranscribe && !cli.ForceLLM && !cli.Recut && isEpisodeClean(it.AudioPath) {
			if !cli.Quiet {
				fmt.Printf("Episode already has ads removed: %s (removing from queue)\n", it.Filename)
			}
			removeEpisodeFromQueueFile(it.PodcastDir, it.Filename)
			continue
		}

		err := processSingleQueuedTarget(it.PodcastDir, it.AudioPath, "rm_ads", cli, cfg)
		if err != nil {
			if !cli.Quiet {
				fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", it.Filename, err)
			}
			failedEpisodes = append(failedEpisodes, it.Filename)
			continue
		}
		processedCount++
	}

	if !cli.Quiet {
		fmt.Printf("\nFinished queue run: %d/%d episode(s) processed successfully.\n", processedCount, total)
		if len(failedEpisodes) > 0 {
			fmt.Printf("Failed episode(s): %s\n", strings.Join(failedEpisodes, ", "))
		}
	}

	if len(failedEpisodes) > 0 {
		return fmt.Errorf("%d episode(s) failed during queue run", len(failedEpisodes))
	}
	return nil
}
