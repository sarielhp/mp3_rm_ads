package cli

import (
	"abs/pkg/adremoval"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
)

type queueEpisodeItem struct {
	PodcastID       string  `json:"podcast_id"`
	EpisodeID       string  `json:"episode_id"`
	Title           string  `json:"title"`
	AudioPath       string  `json:"audio_path"`
	PodcastDir      string  `json:"podcast_dir"`
	Filename        string  `json:"filename"`
	DurationSec     float64 `json:"duration_sec"`
	PublishedAt     string  `json:"published_at,omitempty"`
	ResolutionError string  `json:"resolution_error,omitempty"`
	Priority        int     `json:"priority"`
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
		case "priority":
			subcmd = "priority"
			args = args[1:]
		case "list", "ls":
			subcmd = "list"
			args = args[1:]
		case "add":
			subcmd = "add"
			args = args[1:]
		case "today":
			subcmd = "today"
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
	case "priority":
		return handleQueuePriority(podcastsDir, args)
	case "list", "ls":
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return handleQueueList(podcastsDir, target, cli)
	case "add":
		if len(args) == 0 {
			args = []string{"all"}
		}
		return handleQueueAdd(podcastsDir, args)
	case "today":
		if len(args) != 0 {
			return fmt.Errorf("queue today accepts no arguments")
		}
		return runQueueToday(cfg, podcastsDir, cli, time.Now())
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
		return fmt.Errorf("unknown queue action %q (use list, add, today, remove, clear, or run)", subcmd)
	}
}

func handleQueueList(podcastsDir, target string, cli CLIOptions) error {
	var entries []podcastDirEntry
	if target != "" {
		res, err := resolveQueueTarget(podcastsDir, target)
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
		entries = scanQueuePodcasts(podcastsDir)
	}

	var allItems []queueEpisodeItem
	for _, p := range entries {
		items, err := collectQueueDisplayItems(p)
		if err != nil {
			return err
		}
		allItems = append(allItems, items...)
	}

	sortQueueItems(allItems)
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
			fmt.Println(strings.Join(queueDisplayCells(it), " | "))
		}
		return nil
	}

	printQueueTable(allItems)
	return nil
}

func collectPodcastQueueItems(p podcastDirEntry) ([]queueEpisodeItem, error) {
	filenames, err := pipeline.ReadQueue(p.dir)
	if err != nil {
		return nil, err
	}

	var list []queueEpisodeItem
	for _, fn := range filenames {
		mp3Path, err := pipeline.ResolveQueueAudioPath(p.dir, fn)
		if err != nil {
			return nil, fmt.Errorf("queue %s: %w", p.dir, err)
		}
		epID := podcast.EpisodeShortIDReadOnly(p.dir, p.shortID, mp3Path)
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
	return list, nil
}

func printQueueTable(items []queueEpisodeItem) {
	if len(items) == 0 {
		fmt.Println("AdR queue is currently empty.")
		return
	}

	fmt.Printf("\nAdR Queue (%d queued):\n", len(items))
	cols := queueTableColumns(48)

	fmt.Println(renderTableTop(cols))
	fmt.Println(renderTableHeader(cols))
	fmt.Println(renderTableDivider(cols))

	for _, it := range items {
		cells := queueDisplayCells(it)
		cells[1] = boldCyan(cells[1])
		fmt.Println(renderTableRow(cells, cols))
	}

	fmt.Println(renderTableBottom(cols))
	fmt.Println()
}

func handleQueueAdd(podcastsDir string, targets []string) error {
	for _, query := range targets {
		if strings.EqualFold(query, "all") || query == "*" || query == "--all" {
			entries := scanQueuePodcasts(podcastsDir)
			totalAdded := 0
			for _, p := range entries {
				count, err := addPodcastEpisodesToQueue(p.dir)
				if err != nil {
					return err
				}
				if count > 0 {
					fmt.Printf("Added %d uncleaned episode(s) of %s [%s] to queue\n",
						count, bold(displayName(p.title)), boldCyan(p.shortID))
					totalAdded += count
				}
			}
			fmt.Printf("Added a total of %d uncleaned episode(s) across %d podcast(s) to queue.\n", totalAdded, len(entries))
			continue
		}

		res, err := resolveQueueTarget(podcastsDir, query)
		if err != nil {
			return fmt.Errorf("failed to resolve %q: %w", query, err)
		}

		if res.IsEpisode() {
			ep := res.Episode
			added, err := pipeline.AddToQueueChecked(ep.PodcastDir, queueFilenameForPath(ep.PodcastDir, ep.Path))
			if err != nil {
				return err
			}
			if added {
				fmt.Printf("Added to queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			} else {
				fmt.Printf("Already in queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			}
		} else if res.IsPodcast() {
			pod := res.Podcast
			count, err := addPodcastEpisodesToQueue(pod.Dir)
			if err != nil {
				return err
			}
			fmt.Printf("Added %d uncleaned episode(s) of %s [%s] to queue\n",
				count, bold(displayName(pod.Title)), boldCyan(pod.ShortID))
		}
	}
	return nil
}

func addPodcastEpisodesToQueue(podDir string) (int, error) {
	mp3s := findMP3Files(podDir)
	var candidates []string
	for _, mp3 := range mp3s {
		if pipeline.IsQueueAudioPath(mp3) && !isEpisodeClean(mp3) {
			candidates = append(candidates, queueFilenameForPath(podDir, mp3))
		}
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	addedCount := 0
	err := updateQueue(podDir, func(entries []string) []string {
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
	if err != nil {
		return 0, err
	}
	return addedCount, nil
}

func handleQueueRemove(podcastsDir string, targets []string) error {
	for _, query := range targets {
		res, err := resolveQueueTarget(podcastsDir, query)
		if err != nil {
			return fmt.Errorf("failed to resolve %q: %w", query, err)
		}

		if res.IsEpisode() {
			ep := res.Episode
			removed, err := pipeline.RemoveQueuedAudio(ep.PodcastDir, ep.Path)
			if err != nil {
				return err
			}
			if removed {
				fmt.Printf("Removed from queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			} else {
				fmt.Printf("Not found in queue: [%s] %s\n", boldCyan(ep.ShortID), displayName(ep.Title))
			}
		} else if res.IsPodcast() {
			pod := res.Podcast
			if err := clearPodcastQueue(pod.Dir); err != nil {
				return err
			}
			fmt.Printf("Cleared queue for %s [%s]\n", bold(displayName(pod.Title)), boldCyan(pod.ShortID))
		}
	}
	return nil
}

func handleQueueClear(podcastsDir, target string) error {
	if target != "" {
		res, err := resolveQueueTarget(podcastsDir, target)
		if err != nil {
			return err
		}
		if res.IsPodcast() {
			if err := clearPodcastQueue(res.Podcast.Dir); err != nil {
				return err
			}
			fmt.Printf("Queue cleared for %s [%s]\n", bold(displayName(res.Podcast.Title)), boldCyan(res.Podcast.ShortID))
			return nil
		} else if res.IsEpisode() {
			if _, err := pipeline.RemoveQueuedAudio(res.Episode.PodcastDir, res.Episode.Path); err != nil {
				return err
			}
			fmt.Printf("Removed [%s] from queue\n", boldCyan(res.Episode.ShortID))
			return nil
		}
	}

	entries := scanQueuePodcasts(podcastsDir)
	clearedCount := 0
	for _, p := range entries {
		qFile := filepath.Join(p.dir, "queue.json")
		if _, err := os.Stat(qFile); err == nil {
			if err := clearPodcastQueue(p.dir); err != nil {
				return err
			}
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
	sortQueueItems(items)
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

	cli.Normalize()
	return executeQueueRun(items, cli, cfg)
}

func resolveQueueRunItems(podcastsDir, target string) ([]queueEpisodeItem, error) {
	if target != "" {
		return resolveTargetQueueItems(podcastsDir, target)
	}
	var allItems []queueEpisodeItem
	entries := scanQueuePodcasts(podcastsDir)
	for _, p := range entries {
		items, err := collectPodcastQueueItems(p)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

func resolveTargetQueueItems(podcastsDir, target string) ([]queueEpisodeItem, error) {
	res, err := resolveQueueTarget(podcastsDir, target)
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
		return collectPodcastQueueItems(p)
	}
	if res.IsEpisode() {
		p := podcastDirEntry{
			dir:        res.Episode.PodcastDir,
			folderName: filepath.Base(res.Episode.PodcastDir),
			title:      res.Episode.PodcastTitle,
			shortID:    res.Episode.PodcastShortID,
		}
		items, err := collectPodcastQueueItems(p)
		if err != nil {
			return nil, err
		}
		var matched []queueEpisodeItem
		for _, it := range items {
			if queuePathsMatch(it.PodcastDir, it.Filename, res.Episode.Path) {
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

func queueFilenameForPath(podDir, audioPath string) string {
	rel, err := filepath.Rel(podDir, audioPath)
	if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return rel
	}
	return filepath.Base(audioPath)
}

func queuePathsMatch(podDir, queuedFilename, audioPath string) bool {
	path, err := pipeline.ResolveQueueAudioPath(podDir, queuedFilename)
	return err == nil && filepath.Clean(path) == filepath.Clean(audioPath)
}

func executeQueueRun(items []queueEpisodeItem, cli CLIOptions, cfg Config) error {
	total := len(items)
	processedCount := 0
	var failedEpisodes []string

	for i := range items {
		sortQueueItems(items[i:])
		it := items[i]
		if !cli.Quiet {
			fmt.Printf("\n[%d/%d] Processing queued episode: %s [%s]\n", i+1, total, displayName(it.Title), boldCyan(it.EpisodeID))
		}

		if !fileExists(it.AudioPath) {
			if !cli.Quiet {
				fmt.Printf("Audio file not found on disk: %s (retained in queue)\n", it.Filename)
			}
			failedEpisodes = append(failedEpisodes, it.Filename)
			continue
		}

		if !cli.ForceTranscribe && !cli.ForceLLM && !cli.Recut && isEpisodeClean(it.AudioPath) {
			if !cli.Quiet {
				fmt.Printf("Episode already has ads removed: %s (removing from queue)\n", it.Filename)
			}
			if _, err := pipeline.RemoveQueuedAudio(it.PodcastDir, it.AudioPath); err != nil {
				return err
			}
			continue
		}

		err := adremoval.ProcessQueuedTarget(it.PodcastDir, it.AudioPath, "rm_ads", cli.ProcOptions, cfg)
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

func buildQueueCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "queue",
		Description: "Manage the ad removal (AdR) processing queue",
		UsageLine:   "abs queue [command]",
		Subcommands: []clihelp.Command{
			buildQueueListSubcommand(opts, action),
			buildQueueLsSubcommand(opts, action),
			buildQueueAddSubcommand(opts, action),
			buildQueueTodaySubcommand(opts, action),
			buildQueuePrioritySubcommand(opts, action),
			buildQueueRemoveSubcommand(opts, action),
			buildQueueClearSubcommand(opts, action),
			buildQueueRunSubcommand(opts, action),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue list",
				Description: "List all episodes currently in the ad removal queue",
			},
			{
				Line:        "abs queue run",
				Description: "Process ad removal on all queued episodes",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueListSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "list",
		Description: "List queued episodes across library or podcast",
		UsageLine:   "abs queue list [podcast-id] [options]",
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Print episode details without table borders"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "list"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueLsSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	cmd := buildQueueListSubcommand(opts, action)
	cmd.Name = "ls"
	cmd.UsageLine = "abs queue ls [podcast-id] [options]"
	return cmd
}

func buildQueueAddSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "add",
		Description: "Add uncleaned episodes to the ad removal queue (defaults to all)",
		UsageLine:   "abs queue add [id... | all]",
		Parameters: []clihelp.Param{
			{Name: "[id... | all]", Description: "Episode ID(s), podcast ID(s), or 'all' to queue all uncleaned episodes across library"},
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue add",
				Description: "Queue all episodes needing ad removal across all podcasts",
			},
			{
				Line:        "abs queue add all",
				Description: "Queue all episodes needing ad removal across all podcasts",
			},
			{
				Line:        "abs queue add <podcast-id>",
				Description: "Queue all uncleaned episodes for a specific podcast",
			},
			{
				Line:        "abs queue add <episode-id>",
				Description: "Queue a specific episode for ad removal",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "add"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueRemoveSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "remove",
		Description: "Remove one or more episodes from the queue",
		UsageLine:   "abs queue remove <id...>",
		Args:        clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "remove"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueClearSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "clear",
		Description: "Clear queue for a specific podcast or all podcasts",
		UsageLine:   "abs queue clear [podcast-id]",
		Args:        clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "clear"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueRunSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "run",
		Description: "Process ad removal for queued episodes",
		UsageLine:   "abs queue run [podcast-id] [options]",
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Quiet mode"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Verbose output"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Simulate processing without making changes"),
			clihelp.Bool(&opts.Local, "--local", false, "Force local execution instead of remote"),
			clihelp.Bool(&opts.Remote, "--remote", false, "Force remote execution"),
			clihelp.String(&opts.RemoteHost, "--remote-host <host>", "", "Specify remote processing host"),
			clihelp.String(&opts.Force, "-f, --force <type>", "", "Force re-processing (all, whisper, llm)"),
			clihelp.String(&opts.UseLLM, "--use-llm <name|id>", "", "Select specific LLM profile"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue run",
				Description: "Process ad removal on all queued episodes",
			},
			{
				Line:        "abs queue run <podcast-id>",
				Description: "Process ad removal for queued episodes of a specific podcast",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "run"
			opts.Args = ctx.Args
			return nil
		},
	}
}
