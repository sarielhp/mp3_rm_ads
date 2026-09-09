package main

import (
	"strconv"

	"github.com/sarielhp/clihelp"
)

func buildSyncSubcommands(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {
	cmds := []clihelp.Command{
		buildSyncFeedsSubcommand(opts, action),
		buildSyncDownloadSubcommand(opts, action, countVal, keepVal),
		buildSyncPruneSubcommand(opts, action, keepVal),
		buildSyncPolicySubcommand(opts, action),
		buildSyncListSubcommand(opts, action),
		buildSyncGetInfoSubcommand(opts, action),
	}
	return append(cmds, buildSyncSubcommands2(opts, action)...)
}

func buildSyncFeedsSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "feeds",
		Description: "Fetch and sync latest RSS feeds for podcasts",
		UsageLine:   "abs sync feeds [options] [podcast-id]",
		Parameters: []clihelp.Param{
			{Name: "[podcast-id]", Description: "Optional podcast identifier"},
		},
		Args: clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "feeds"
			opts.ServerSubcmd = "feeds"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildSyncDownloadSubcommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "download",
		Description: "Download undownloaded episodes for podcasts",
		UsageLine:   "abs sync download [podcast-id] [options]",
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
			*action = "sync"
			opts.SyncSubcmd = "download"
			opts.ServerSubcmd = "download"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 {
				if k, err := strconv.Atoi(ctx.Args[0]); err == nil {
					opts.Count = k
					opts.CountGiven = true
				} else if opts.Podcast == "" {
					opts.Podcast = ctx.Args[0]
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
	}
}

func buildSyncPruneSubcommand(opts *CLIOptions, action *string, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "prune",
		Description: "Delete older episodes per retention limit",
		UsageLine:   "abs sync prune [number] [options]",
		Parameters:  []clihelp.Param{{Name: "[number]", Description: "Number of latest episodes to keep per podcast"}},
		Args:        clihelp.RangeArgs(0, 1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast"),
			clihelp.Int(keepVal, "-k, --keep <number>", -1, "Keep policy count"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Dry run"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "prune"
			opts.ServerSubcmd = "keep"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 {
				if k, err := strconv.Atoi(ctx.Args[0]); err == nil {
					opts.KeepCount = &k
				} else if opts.Podcast == "" {
					opts.Podcast = ctx.Args[0]
				}
			} else if *keepVal > 0 {
				opts.KeepCount = keepVal
			}
			return nil
		},
	}
}

func buildSyncPolicySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "policy",
		Description: "View or update podcast download and AdR policy",
		UsageLine:   "abs sync policy <podcast-id> [<number>] [options]",
		Parameters: []clihelp.Param{
			{Name: "<podcast-id>", Description: "Target podcast identifier"},
			{Name: "[<number>]", Description: "Shorthand: auto-download latest K episodes with ad-removal all"},
		},
		Args: clihelp.RangeArgs(1, 2),
		Options: []clihelp.Option{
			clihelp.String(&opts.AutoDownloadStr, "--auto-download <bool>", "", "Enable automatic downloads (true/false)"),
			clihelp.String(&opts.DownloadPolicy, "--download-policy <mode>", "", "Policy mode ('none', 'latest', 'latest_k', 'all')"),
			clihelp.Int(&opts.DownloadK, "--download-k <num>", 0, "Number of latest episodes to download"),
			clihelp.String(&opts.AutoCleanupStr, "--auto-cleanup <bool>", "", "Enable automatic cleanup (true/false)"),
			clihelp.Int(&opts.CleanupDays, "--cleanup-days <days>", 0, "Retention window in days"),
			clihelp.String(&opts.AdRemovalMode, "--ad-removal <mode>", "", "Ad removal policy mode ('none', 'latest', 'all')"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs sync policy 42 1",
				Description: "Shorthand: auto-download latest 1 episode and remove all ads",
			},
			{
				Line:        "abs sync policy 'Huberman Lab' 3",
				Description: "Shorthand: auto-download latest 3 episodes and remove all ads",
			},
			{
				Line:        "abs sync policy 42 --download-policy all --ad-removal all",
				Description: "Configure podcast 42 to download all episodes and remove ads",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "policy"
			opts.ServerSubcmd = "policy"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildSyncListSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "list",
		Description: "List all available podcasts in server with episode counts",
		UsageLine:   "abs sync list [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output (Feed URLs and IDs)"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "list"
			opts.ServerSubcmd = "list"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildSyncGetInfoSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "get-info",
		Description: "Cache metadata for latest K episodes",
		UsageLine:   "abs sync get-info [<k>] [options]",
		Parameters:  []clihelp.Param{{Name: "[<k>]", Description: "Episodes count per podcast (default 100)"}},
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
			clihelp.Bool(&opts.Refresh, "-r, --refresh", false, "Force re-fetching metadata from feed"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed per-episode metadata"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "get-info"
			opts.ServerSubcmd = "get-info"
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
	}
}
