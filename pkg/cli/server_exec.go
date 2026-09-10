package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"abs/pkg/backend"
	"abs/pkg/podcast"
	"github.com/sarielhp/clihelp"
)

func handleServerCommand(config Config, cli CLIOptions) error {
	subcmd := cli.ServerSubcmd
	if subcmd == "" {
		subcmd = cli.SyncSubcmd
	}
	switch subcmd {
	case "":
		showServerUsage()
		return nil
	case "feeds":
		return handleServerFeeds(config, cli)
	case "download":
		return handleServerDownload(config, cli)
	case "prune", "keep":
		return handleServerKeep(config, cli)
	case "policy":
		return runPolicyCommand(config, cli)
	case "list":
		return handleServerList(config, cli)
	case "get-info", "get_info":
		return handleServerGetInfo(config, cli)
	case "rescan":
		return handleServerRescan(config, cli)
	case "timeline":
		return handleServerTimeline(config, cli)
	case "opml":
		return handleServerOPML(config, cli)
	case "frequency":
		return handleServerFrequency(config, cli)
	case "disable-hourly", "disable_hourly":
		return handleServerDisableHourly(config, cli)
	case "clean-orphans":
		return handleServerCleanOrphans(config, cli)
	default:
		return fmt.Errorf("unknown server subcommand %q", subcmd)
	}
}

func showServerUsage() {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)
	_ = app.RenderCommand(clihelp.Options{}, "server")
}

func matchBackendPodcast(podcasts []backend.Podcast, target string) *backend.Podcast {
	clean := strings.TrimSpace(target)
	if clean == "" {
		return nil
	}
	if idx, err := strconv.Atoi(clean); err == nil && idx >= 1 && idx <= len(podcasts) {
		return &podcasts[idx-1]
	}
	for i := range podcasts {
		if podcasts[i].ID == clean || podcasts[i].Media.ID == clean {
			return &podcasts[i]
		}
	}
	lower := strings.ToLower(clean)
	for i := range podcasts {
		if strings.ToLower(podcasts[i].Media.Metadata.Title) == lower {
			return &podcasts[i]
		}
	}
	for i := range podcasts {
		if strings.Contains(strings.ToLower(podcasts[i].Media.Metadata.Title), lower) {
			return &podcasts[i]
		}
	}
	for i := range podcasts {
		short := podcast.GeneratePodcastShortID(podcasts[i].Media.Metadata.Title)
		if strings.EqualFold(short, clean) {
			return &podcasts[i]
		}
	}
	return nil
}

func resolveServerTargetPodcasts(b backend.Backend, cli CLIOptions) ([]backend.Podcast, error) {
	podcasts, err := b.Podcasts()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch podcasts from server: %w", err)
	}
	target := cli.Podcast
	if target == "" && len(cli.Args) > 0 {
		if cli.Args[0] != "update" {
			target = cli.Args[0]
		} else if len(cli.Args) > 1 {
			target = cli.Args[1]
		}
	}
	if target != "" {
		matched := matchBackendPodcast(podcasts, target)
		if matched == nil {
			return nil, fmt.Errorf("podcast matching %q not found on server", target)
		}
		return []backend.Podcast{*matched}, nil
	}
	var active []backend.Podcast
	for _, p := range podcasts {
		if strings.TrimSpace(p.Media.Metadata.FeedURL) != "" {
			active = append(active, p)
		}
	}
	if len(active) > 0 {
		return active, nil
	}
	return podcasts, nil
}

func handleServerFeeds(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	if !cli.Quiet {
		fmt.Printf("Waking up server and checking feeds for %d podcast(s)...\n", len(podcasts))
	}
	totalNew := 0
	for _, item := range podcasts {
		newCount, err := refreshSinglePodcastFeed(b, item, cli.Quiet, cli.Verbose)
		if err != nil && !cli.Quiet {
			fmt.Printf("! %s: %v\n", item.Media.Metadata.Title, err)
		}
		totalNew += newCount
	}
	if !cli.Quiet {
		fmt.Printf("\nChecked a total of %d podcast feed(s) (%d undownloaded episode(s) available).\n", len(podcasts), totalNew)
	}
	return nil
}

func refreshSinglePodcastFeed(b backend.Backend, item backend.Podcast, quiet, verbose bool) (int, error) {
	title := item.Media.Metadata.Title
	if title == "" {
		title = "Untitled Podcast"
	}
	_ = b.ResetPodcastDateCheck(item.ID, title)

	feedURL := item.Media.Metadata.FeedURL
	if feedURL == "" {
		return 0, fmt.Errorf("no feed URL configured for %s", title)
	}

	feedEpisodes, err := b.PodcastFeedEpisodes(feedURL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch feed episodes for %s: %w", title, err)
	}

	isDownloaded := podcast.BuildDownloadedChecker(b, item, item.ID)
	undownloaded := 0
	for _, ep := range feedEpisodes {
		if !isDownloaded(ep) {
			undownloaded++
		}
	}

	if !quiet {
		fmt.Printf("✓ %s: server awakened, feed checked (%d total episodes, %d undownloaded)\n", title, len(feedEpisodes), undownloaded)
		if verbose && undownloaded > 0 {
			for _, ep := range feedEpisodes {
				if !isDownloaded(ep) {
					fmt.Printf("    + %s (%s)\n", ep.Title, ep.PubDate)
				}
			}
		}
	}
	return undownloaded, nil
}

func handleServerDownload(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	if !cli.Quiet {
		fmt.Println("Refreshing feeds before downloading...")
	}
	for _, item := range podcasts {
		_, _ = refreshSinglePodcastFeed(b, item, true, false)
	}
	return executeServerDownloads(b, config, cli, podcasts)
}

func executeServerDownloads(b backend.Backend, config Config, cli CLIOptions, podcasts []backend.Podcast) error {
	opts := podcast.DownloadOptions{
		Count:       cli.Count,
		Oldest:      cli.Oldest,
		DryRun:      cli.DryRun,
		NoWait:      cli.NoWait,
		Fill:        cli.Fill,
		CountGiven:  cli.CountGiven,
		CheckNew:    cli.CheckNew,
		DownloadAll: cli.DownloadAll,
		Keep:        cli.KeepCount,
		Verbose:     cli.Verbose,
		Quiet:       cli.Quiet,
	}
	totalDownloaded := 0
	for idx, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = "Untitled"
		}
		if !cli.Quiet && len(podcasts) > 1 {
			fmt.Printf("\rDownloading episodes (%d/%d): %s\x1b[K", idx+1, len(podcasts), title)
			os.Stdout.Sync()
		}
		if fresh, err := b.GetPodcast(item.ID); err == nil && fresh != nil {
			item = *fresh
		}
		count := podcast.DownloadPodcastEpisodes(b, item, opts)
		if !cli.DryRun {
			totalDownloaded += count
		}
	}
	if !cli.Quiet && len(podcasts) > 1 {
		fmt.Print("\r\x1b[K")
	}
	return finalizeServerDownloads(b, config, cli, podcasts, totalDownloaded)
}

func finalizeServerDownloads(b backend.Backend, config Config, cli CLIOptions, podcasts []backend.Podcast, totalDownloaded int) error {
	if !cli.Quiet {
		fmt.Printf("Queued %d episode download(s) across %d podcast(s).\n", totalDownloaded, len(podcasts))
	}
	if totalDownloaded > 0 && !cli.NoWait && !cli.DryRun {
		if !cli.Quiet {
			fmt.Printf("Waiting for server to complete %d queued download(s)...\n", totalDownloaded)
		}
		_ = b.WaitForActiveDownloads(podcasts, cli.Quiet, 5*time.Minute)
	}
	if totalDownloaded > 0 && !cli.DryRun {
		if len(config.PostProcessors) > 0 {
			runPostProcessors(config.PostProcessors, cli.Quiet)
		} else {
			processAudioFilesBatch(cli, config, "proc")
		}
	}
	return nil
}

func runPostProcessors(processors []string, quiet bool) {
	if !quiet {
		fmt.Printf("\n=== Executing %d Post-Processor(s) ===\n", len(processors))
	}
	for _, proc := range processors {
		if !quiet {
			fmt.Printf("Running post-processor: %s...\n", proc)
		}
		parts := strings.Fields(proc)
		if len(parts) == 0 {
			continue
		}
		var cmd *exec.Cmd
		if len(parts) > 1 {
			cmd = exec.Command(parts[0], parts[1:]...)
		} else {
			cmd = exec.Command(parts[0])
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}
}

func handleServerKeep(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	keep := -1
	if cli.KeepCount != nil {
		keep = *cli.KeepCount
	}
	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = "Untitled"
		}
		deleted, err := b.ApplyKeepPolicy(item.ID, title, keep, cli.DryRun, cli.Verbose, cli.Quiet)
		if err != nil && !cli.Quiet {
			fmt.Printf("! Error applying keep policy to %s: %v\n", title, err)
		} else if !cli.Quiet {
			fmt.Printf("✓ %s: pruned %d episode(s) (limit: %d)\n", title, deleted, keep)
		}
	}
	return nil
}

func handleServerList(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := b.Podcasts()
	if err != nil {
		return fmt.Errorf("failed to fetch podcasts: %w", err)
	}
	if len(podcasts) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcasts found on server.")
		}
		return nil
	}
	for idx, p := range podcasts {
		shortID := podcast.GeneratePodcastShortID(p.Media.Metadata.Title)
		fmt.Printf("%3d. %s [%s] (%d episodes)\n", idx+1, bold(p.Media.Metadata.Title), boldCyan(shortID), len(p.Media.Episodes))
		if cli.Verbose {
			fmt.Printf("     ID:      %s\n", p.ID)
			if p.Media.Metadata.FeedURL != "" {
				fmt.Printf("     Feed:    %s\n", p.Media.Metadata.FeedURL)
			}
			if p.RelPath != "" {
				fmt.Printf("     Folder:  %s\n", p.RelPath)
			}
		}
	}
	return nil
}

func handleServerOPML(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	targetFile := cli.OPMLFile
	if targetFile == "" && len(cli.Args) > 0 {
		targetFile = cli.Args[0]
	}
	switch cli.OPMLSubcmd {
	case "export":
		if targetFile == "" {
			return fmt.Errorf("missing required <file> argument for 'abs server opml export <file>'")
		}
		data, err := b.ExportOPML(backend.OPMLExportOptions{Quiet: cli.Quiet, Verbose: cli.Verbose})
		if err != nil {
			return fmt.Errorf("OPML export failed: %w", err)
		}
		if err := os.WriteFile(targetFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write OPML file: %w", err)
		}
		if !cli.Quiet {
			fmt.Printf("Exported podcast subscriptions to %s\n", targetFile)
		}
		return nil
	case "import":
		if targetFile == "" {
			return fmt.Errorf("missing required <file> argument for 'abs server opml import <file>'")
		}
		data, err := os.ReadFile(targetFile)
		if err != nil {
			return fmt.Errorf("failed to read OPML file: %w", err)
		}
		res, err := b.ImportOPML(data, backend.OPMLImportOptions{Quiet: cli.Quiet, Verbose: cli.Verbose})
		if err != nil {
			return fmt.Errorf("OPML import failed: %w", err)
		}
		if !cli.Quiet {
			fmt.Printf("Imported %d new feed(s) from %s\n", res.Subscribed, targetFile)
		}
		return nil
	default:
		return fmt.Errorf("must specify 'import <file>' or 'export <file>'")
	}
}

func handleServerCleanOrphans(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	opts := podcast.CleanOrphansOptions{
		DryRun:  cli.DryRun,
		Force:   cli.ForceDelete,
		Quiet:   cli.Quiet,
		Verbose: cli.Verbose,
	}
	_, err = podcast.RunCleanOrphans(b, opts)
	return err
}

func handleServerRescan(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	opts := backend.RescanOptions{
		PodcastsDir: config.PodcastsDir,
		PodcastID:   cli.Podcast,
		DryRun:      cli.DryRun,
		Verbose:     cli.Verbose,
		Quiet:       cli.Quiet,
	}
	res, err := b.Rescan(opts)
	if err != nil {
		return fmt.Errorf("rescan failed: %w", err)
	}
	if !cli.Quiet {
		fmt.Printf("Rescan completed: checked %d episodes, updated %d.\n", res.CheckedCount, res.RescanCount)
	}
	return nil
}

func handleServerTimeline(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	for _, item := range podcasts {
		if !cli.Quiet {
			fmt.Printf("\nTimeline for %s (%d episodes):\n", bold(item.Media.Metadata.Title), len(item.Media.Episodes))
		}
		for i, ep := range item.Media.Episodes {
			if i >= 10 && !cli.Verbose {
				break
			}
			fmt.Printf("  - %s (%s)\n", ep.Title, ep.PubDate)
		}
	}
	return nil
}

func handleServerGetInfo(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := b.Podcasts()
	if err != nil {
		return fmt.Errorf("failed to fetch podcasts: %w", err)
	}
	totalEpisodes := 0
	for _, p := range podcasts {
		totalEpisodes += len(p.Media.Episodes)
	}
	fmt.Printf("Server:          %s\n", b.Name())
	fmt.Printf("Total Podcasts:  %d\n", len(podcasts))
	fmt.Printf("Total Episodes:  %d\n", totalEpisodes)
	return nil
}

func handleServerFrequency(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	for _, item := range podcasts {
		eps, err := podcast.GetEpisodesForFrequency(b, item, config.PodcastsDir, cli.Refresh, nil)
		if err != nil && !cli.Quiet {
			fmt.Printf("! %s: %v\n", item.Media.Metadata.Title, err)
			continue
		}
		if !cli.Quiet {
			fmt.Printf("✓ %s: %d episodes analyzed\n", item.Media.Metadata.Title, len(eps))
		}
	}
	return nil
}

func handleServerDisableHourly(config Config, cli CLIOptions) error {
	cli.DisableHourly = true
	return handleServerFrequency(config, cli)
}
