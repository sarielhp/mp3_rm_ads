package cli

import (
	"abs/pkg/adremoval"
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
)

func buildServerDownloadSubcommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "download",
		Description: "Update feeds and download undownloaded episodes for podcasts",
		UsageLine:   "abs server download [podcast-id] [options]",
		Parameters:  []clihelp.Param{{Name: "[podcast-id]", Description: "Specify podcast by name, index, or ID"}},
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
			clihelp.Int(countVal, "-k, --count <number>", -1, "Number of undownloaded episodes to download"),
			clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
			clihelp.Bool(&opts.Fill, "-f, --fill", false, "Fill gaps in downloaded episodes"),
			clihelp.Int(keepVal, "-K, --keep <number>", -1, "Enforce keep count policies"),
			clihelp.BoolToggle(&opts.CheckNew, "--[no-]check-new", true, "Check new episodes published"),
			clihelp.Bool(&opts.Oldest, "--oldest", false, "Download oldest first"),
			clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed info"),
			clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host"),
			clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "download"
			opts.SyncSubcmd = "download"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 && opts.Podcast == "" {
				opts.Podcast = ctx.Args[0]
			}
			if *countVal != -1 {
				if *countVal <= 0 {
					return fmt.Errorf("download count must be positive")
				}
				opts.Count = *countVal
				opts.CountGiven = true
			} else {
				opts.Count = 1
			}
			if *keepVal > 0 {
				opts.KeepCount = keepVal
			}
			return nil
		},
	}
}

func handleServerDownload(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	return runServerDownloads(b, config, cli)
}

func runServerDownloads(b backend.Backend, config Config, cli CLIOptions) error {
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	if !cli.DryRun {
		if !cli.Quiet {
			fmt.Println("Refreshing feeds before downloading...")
		}
		// Only the podcasts whose feeds actually changed are handed to the
		// server. A feed that cannot be read is reported and skipped rather
		// than aborting the whole download run.
		summary := checkServerFeeds(b, podcasts, cli)
		if !cli.Quiet {
			fmt.Printf("%d feed(s) changed, %d unchanged, %d unreadable (%.1fs).\n",
				summary.Changed, summary.Unchanged, summary.Unreadable, summary.Elapsed.Seconds())
		}
	}
	return executeServerDownloads(b, config, cli, podcasts)
}

func executeServerDownloads(b backend.Backend, config Config, cli CLIOptions, podcasts []backend.Podcast) error {
	opts := podcast.DownloadOptions{
		Count:       cli.Count,
		Oldest:      cli.Oldest,
		DryRun:      cli.DryRun,
		NoWait:      true,
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
		count, err := podcast.DownloadPodcastEpisodes(b, item, opts)
		if err != nil {
			return fmt.Errorf("download %s: %w", title, err)
		}
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
		if err := b.WaitForActiveDownloads(podcasts, cli.Quiet, 5*time.Minute); err != nil {
			return fmt.Errorf("waiting for downloads: %w", err)
		}
	}
	if totalDownloaded > 0 && !cli.DryRun && !cli.NoWait {
		if len(config.PostProcessors) > 0 {
			runPostProcessors(config.PostProcessors, cli.Quiet)
		} else {
			adremoval.ProcessBatch(cli, config, "proc")
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
