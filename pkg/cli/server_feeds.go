package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"abs/pkg/backend"
	"abs/pkg/podcast"
	"abs/pkg/util"

	"github.com/sarielhp/clihelp"
)

const (
	// feedCheckTimeout bounds a single feed fetch. A sweep waits on dozens of
	// unrelated origins, so one unresponsive publisher must not be able to
	// stall the run.
	feedCheckTimeout = 15 * time.Second
	// feedCheckAttempts is deliberately small: a feed that fails twice is
	// reported as unreadable and handed to the server rather than retried at
	// length here.
	feedCheckAttempts = 2
	// maxServerRefreshes caps concurrent server refreshes. Each one makes the
	// server fetch and parse a feed, so this is a limit on the server's work,
	// not on ours.
	maxServerRefreshes = 4
)

type feedCheckSummary struct {
	Results      []podcast.FeedCheckResult
	Unchanged    int
	Changed      int
	Unreadable   int
	Refreshed    int
	NewEpisodes  int
	Undownloaded int
	Elapsed      time.Duration
}

// resolveFeedTargets resolves the podcasts to check. Unlike the download and
// pruning paths it needs only metadata, so it takes the backend's cheap listing
// when one exists rather than transferring every episode of every podcast.
func resolveFeedTargets(b backend.Backend, cli CLIOptions) ([]backend.Podcast, error) {
	podcasts, err := backend.ListPodcastsFrom(b)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch podcasts from server: %w", err)
	}
	return filterServerTargets(podcasts, cli)
}

// checkServerFeeds fetches every target feed directly and conditionally, then
// asks the server to update only those that turned out to need it.
func checkServerFeeds(b backend.Backend, podcasts []backend.Podcast, cli CLIOptions) *feedCheckSummary {
	start := time.Now()
	index := podcast.BuildEpisodeIndex(b, podcasts)
	results := podcast.CheckFeedsForUpdates(podcasts, index, podcast.FeedCheckOptions{
		Concurrency: cli.FeedJobs,
		Force:       cli.Refresh,
		Timeout:     feedCheckTimeout,
		MaxAttempts: feedCheckAttempts,
	})

	summary := &feedCheckSummary{Results: results}
	for i := range results {
		r := &results[i]
		switch r.Status {
		case podcast.FeedUnchanged:
			summary.Unchanged++
		case podcast.FeedChanged:
			summary.Changed++
		default:
			summary.Unreadable++
		}
		summary.NewEpisodes += len(r.New)
		summary.Undownloaded += r.Undownloaded
	}

	summary.Refreshed = wakeServerForFeeds(b, results, cli.DryRun)
	if summary.Refreshed > 0 {
		waitForServerIndexing(b, results, cli.Quiet || cli.DryRun)
	}
	summary.Elapsed = time.Since(start)
	return summary
}

// wakeServerForFeeds resets the server's episode check date for the podcasts
// whose feeds actually changed, so the server picks up the new episodes. Feeds
// the origin confirmed unchanged are skipped entirely, which is what keeps the
// command from making the server refetch every feed on every run.
func wakeServerForFeeds(b backend.Backend, results []podcast.FeedCheckResult, dryRun bool) int {
	var targets []*podcast.FeedCheckResult
	for i := range results {
		if results[i].NeedsServer() {
			targets = append(targets, &results[i])
		}
	}
	if len(targets) == 0 || dryRun {
		return 0
	}

	workers := min(maxServerRefreshes, len(targets))
	jobs := make(chan *podcast.FeedCheckResult)
	var wg util.WaitGroup
	var mu util.Mutex
	refreshed := 0

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				if err := b.ResetPodcastDateCheck(target.Podcast.ID, target.Title); err != nil {
					continue
				}
				mu.Lock()
				refreshed++
				mu.Unlock()
			}
		}()
	}
	for _, target := range targets {
		jobs <- target
	}
	close(jobs)
	wg.Wait()

	return refreshed
}

func waitForServerIndexing(b backend.Backend, results []podcast.FeedCheckResult, quiet bool) {
	if b == nil || b.Name() != "podfetch" {
		return
	}
	pending := make(map[string]map[string]bool)
	for _, r := range results {
		if !r.NeedsServer() || len(r.New) == 0 {
			continue
		}
		guids := make(map[string]bool, len(r.New))
		for _, ep := range r.New {
			if g := strings.TrimSpace(ep.GUID); g != "" {
				guids[g] = true
			}
		}
		if len(guids) > 0 {
			pending[r.Podcast.ID] = guids
		}
	}
	if len(pending) == 0 {
		return
	}
	pollCatalogUntilIndexed(b, pending, quiet)
}

func pollCatalogUntilIndexed(b backend.Backend, pending map[string]map[string]bool, quiet bool) {
	deadline := time.Now().Add(25 * time.Second)
	indexer, ok := b.(backend.CatalogIndexer)
	if !ok {
		return
	}
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		eps, err := indexer.CatalogEpisodes()
		if err != nil {
			return
		}
		for _, ep := range eps {
			if guids, exists := pending[ep.PodcastID]; exists {
				delete(guids, ep.GUID)
				if len(guids) == 0 {
					delete(pending, ep.PodcastID)
				}
			}
		}
		if len(pending) == 0 {
			break
		}
		if !quiet {
			fmt.Printf("\rWaiting for server to index %d podcast(s)...\x1b[K", len(pending))
			os.Stdout.Sync()
		}
	}
	if !quiet {
		fmt.Print("\r\x1b[K")
	}
}

func reportFeedCheck(summary *feedCheckSummary, cli CLIOptions) {
	if cli.Quiet {
		return
	}
	for i := range summary.Results {
		r := &summary.Results[i]
		if cli.Verbose || r.Status != podcast.FeedUnchanged || len(r.New) > 0 {
			printFeedCheckLine(r, cli.Verbose)
		}
	}

	total := len(summary.Results)
	fmt.Printf("\nChecked %d feed(s) in %.1fs: %d unchanged, %d changed, %d unreadable.\n",
		total, summary.Elapsed.Seconds(), summary.Unchanged, summary.Changed, summary.Unreadable)
	fmt.Printf("%d new episode(s) not yet in the server catalog (%d undownloaded episode(s) available).\n",
		summary.NewEpisodes, summary.Undownloaded)
	if summary.Refreshed > 0 {
		fmt.Printf("Woke the server for %d podcast(s); the rest needed no server work.\n", summary.Refreshed)
	} else {
		fmt.Println("No server work was needed.")
	}
}

func printFeedCheckLine(r *podcast.FeedCheckResult, verbose bool) {
	switch {
	case r.Status == podcast.FeedUnknown:
		fmt.Printf("! %s: could not read feed: %v\n", r.Title, r.Err)
	case len(r.New) > 0:
		fmt.Printf("+ %s: %d new episode(s) (%d in feed, %d undownloaded)\n",
			r.Title, len(r.New), r.EpisodeCount, r.Undownloaded)
		if verbose {
			for _, ep := range r.New {
				fmt.Printf("    + %s (%s)\n", ep.Title, ep.PubDate)
			}
		}
	default:
		fmt.Printf("  %s: unchanged (%s, %d episodes, %d undownloaded)\n",
			r.Title, r.Reason, r.EpisodeCount, r.Undownloaded)
	}
}

func buildServerFeedsSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "feeds",
		Description: "Check podcast feeds directly for newly published episodes",
		UsageLine:   "abs server feeds [command] [options] [podcast-id]",
		Subcommands: []clihelp.Command{
			buildServerFeedsUpdateSubcommand(opts, action),
		},
		Parameters: []clihelp.Param{
			{Name: "[podcast-id]", Description: "Optional podcast identifier"},
		},
		Args: clihelp.MaximumNArgs(2),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
			clihelp.Bool(&opts.Refresh, "-F, --force", false, "Ignore cached feed validators and re-read every feed"),
			clihelp.Int(&opts.FeedJobs, "-j, --jobs <number>", 0, "Feeds to fetch concurrently (default 16)"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs server feeds",
				Description: "Check all feeds for new episodes, waking the server only for those that changed",
			},
			{
				Line:        "abs server feeds update -v",
				Description: "Scan all podcast feeds and report the verdict for each one",
			},
			{
				Line:        "abs server feeds update -p 'Huberman Lab'",
				Description: "Check the feed for a specific podcast",
			},
			{
				Line:        "abs server feeds update --force -j 24",
				Description: "Re-read every feed, ignoring cached validators, 24 at a time",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "feeds"
			opts.SyncSubcmd = "feeds"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 {
				if ctx.Args[0] == "update" {
					if len(ctx.Args) > 1 && opts.Podcast == "" {
						opts.Podcast = ctx.Args[1]
					}
				} else if opts.Podcast == "" {
					opts.Podcast = ctx.Args[0]
				}
			}
			return nil
		},
	}
}

func buildServerFeedsUpdateSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "update",
		Description: "Check podcast feeds directly for newly published episodes",
		UsageLine:   "abs server feeds update [options] [podcast-id]",
		Parameters: []clihelp.Param{
			{Name: "[podcast-id]", Description: "Optional podcast identifier"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
			clihelp.Bool(&opts.Refresh, "-F, --force", false, "Ignore cached feed validators and re-read every feed"),
			clihelp.Int(&opts.FeedJobs, "-j, --jobs <number>", 0, "Feeds to fetch concurrently (default 16)"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "feeds"
			opts.SyncSubcmd = "feeds"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 && opts.Podcast == "" {
				opts.Podcast = ctx.Args[0]
			}
			return nil
		},
	}
}

func handleServerFeeds(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveFeedTargets(b, cli)
	if err != nil {
		return err
	}
	if !cli.Quiet {
		fmt.Printf("Checking %d podcast feed(s) directly for new episodes...\n", len(podcasts))
	}
	reportFeedCheck(checkServerFeeds(b, podcasts, cli), cli)
	return nil
}
