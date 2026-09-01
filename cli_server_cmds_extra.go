package main

import (
	"fmt"
	"strconv"

	"github.com/sarielhp/clihelp"
)

func buildServerSubcommands(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {
	cmds := []clihelp.Command{
		{
			Name:        "scan",
			Description: "Scan Audiobookshelf for new podcasts (create directories, cache covers) and check for new episodes",
			UsageLine:   "abs server scan [podcasts_dir] [options]",
			Parameters: []clihelp.Param{
				{Name: "[podcasts_dir]", Description: "Optional podcasts directory path (defaults to configured podcasts_dir)"},
			},
			Args:             clihelp.MaximumNArgs(1),
			OptionsValidator: clihelp.MutuallyExclusive("--podcasts-only", "--episodes-only"),
			Options: []clihelp.Option{
				clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID to check/download new episodes"),
				clihelp.Int(countVal, "-k, --count <number>", -1, "Explicit number of episodes to download (overrides policy)"),
				clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
				clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
				clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
				clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
				clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host (e.g. cloud8)"),
				clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing (skip remote host)"),
				clihelp.Bool(&opts.PodcastsOnly, "--podcasts-only", false, "Only scan for new podcasts and create directories (skip episode downloads)"),
				clihelp.Bool(&opts.EpisodesOnly, "--episodes-only", false, "Only check and download new episodes (skip podcast folder scanning)"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "scan"
				if *countVal > 0 {
					opts.Count = *countVal
					opts.CountGiven = true
				}
				opts.Args = ctx.Args
				return nil
			},
		},
		{
			Name:        "new",
			Description: "Check and download new podcast episodes based on download policies",
			UsageLine:   "abs server new [podcasts_dir] [options]",
			Parameters: []clihelp.Param{
				{Name: "[podcasts_dir]", Description: "Optional podcasts directory path (defaults to configured podcasts_dir)"},
			},
			Args: clihelp.MaximumNArgs(1),
			Options: []clihelp.Option{
				clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID to check/download new episodes"),
				clihelp.Int(countVal, "-k, --count <number>", -1, "Explicit number of episodes to download (overrides policy)"),
				clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
				clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
				clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
				clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
				clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host (e.g. cloud8)"),
				clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing (skip remote host)"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "scan"
				opts.EpisodesOnly = true
				if *countVal > 0 {
					opts.Count = *countVal
					opts.CountGiven = true
				}
				opts.Args = ctx.Args
				return nil
			},
		},
		{
			Name:        "list",
			Description: "List all available podcasts in Audiobookshelf with episode counts",
			UsageLine:   "abs server list [options]",
			Args:        clihelp.NoArgs,
			Options: []clihelp.Option{
				clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output (Feed URLs and IDs)"),
				clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "list"
				opts.Args = ctx.Args
				return nil
			},
		},
		{
			Name:        "download",
			Description: "Download undownloaded episodes for podcasts",
			UsageLine:   "abs server download [<number>] [options]",
			Parameters: []clihelp.Param{
				{Name: "[<number>]", Description: "Optional number of undownloaded episodes to download (defaults to 1)"},
			},
			Args: clihelp.MaximumNArgs(1),
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
				clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host (e.g. cloud8)"),
				clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing (skip remote host)"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "download"
				opts.Args = ctx.Args
				if len(ctx.Args) > 0 {
					k, err := strconv.Atoi(ctx.Args[0])
					if err == nil {
						opts.Count = k
						opts.CountGiven = true
					}
				} else if *countVal != -1 {
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
		},
		{
			Name:        "keep",
			Description: "Delete older episodes keeping only the latest <number> episodes per podcast",
			UsageLine:   "abs server keep <number> [options]",
			Parameters: []clihelp.Param{
				{Name: "<number>", Description: "Number of latest episodes to keep per podcast"},
			},
			Args: clihelp.ExactArgs(1),
			Options: []clihelp.Option{
				clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast"),
				clihelp.Int(keepVal, "-k, --keep <number>", -1, "Keep policy count"),
				clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				clihelp.Bool(&opts.DryRun, "--dry-run", false, "Dry run"),
				clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "keep"
				opts.Args = ctx.Args
				if len(ctx.Args) > 0 {
					k, err := strconv.Atoi(ctx.Args[0])
					if err == nil {
						opts.KeepCount = &k
					}
				} else if *keepVal > 0 {
					opts.KeepCount = keepVal
				}
				if opts.KeepCount == nil {
					return fmt.Errorf("keep count is required (e.g. abs server keep 5)")
				}
				return nil
			},
		},
		{
			Name:        "get_info",
			Description: "Force Audiobookshelf to fetch and cache metadata for the latest K episodes without downloading audio",
			UsageLine:   "abs server get_info [<k>] [options]",
			Parameters: []clihelp.Param{
				{Name: "[<k>]", Description: "Number of latest episodes to fetch metadata for per podcast (defaults to 100)"},
			},
			Args: clihelp.MaximumNArgs(1),
			Options: []clihelp.Option{
				clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID (defaults to all podcasts)"),
				clihelp.Bool(&opts.Refresh, "-r, --refresh", false, "Force re-fetching latest metadata from online RSS feeds (bypasses cache)"),
				clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed per-episode metadata"),
			},
			Run: func(ctx *clihelp.Context) error {
				*action = "server"
				opts.ServerSubcmd = "get_info"
				opts.Count = 100
				opts.CountGiven = false
				opts.Args = ctx.Args
				if len(ctx.Args) > 0 {
					if k, err := strconv.Atoi(ctx.Args[0]); err == nil && k > 0 {
						opts.Count = k
						opts.CountGiven = true
					}
				}
				return nil
			},
		},
	}
	return append(cmds, buildServerSubcommands2(opts, action, countVal, keepVal)...)
}
