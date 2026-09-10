package cli

import (
	"fmt"
	"strconv"

	"github.com/sarielhp/clihelp"
)

func buildServerSubcommands(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {
	cmds := []clihelp.Command{
		buildServerFeedsSubcommand(opts, action),
		buildServerDownloadSubcommand(opts, action, countVal, keepVal),
		buildServerPruneSubcommand(opts, action, keepVal),
		buildServerPolicySubcommand(opts, action),
		buildServerListSubcommand(opts, action),
		buildServerGetInfoSubcommand(opts, action),
	}
	return append(cmds, buildServerSubcommands2(opts, action)...)
}

func buildServerFeedsUpdateSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "update",
		Description: "Wake up server and check for newly published episodes across feeds",
		UsageLine:   "abs server feeds update [options] [podcast-id]",
		Parameters: []clihelp.Param{
			{Name: "[podcast-id]", Description: "Optional podcast identifier"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
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

func buildServerFeedsSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "feeds",
		Description: "Wake up server and check for newly published episodes across feeds",
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
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs server feeds",
				Description: "Wake up server and check all feeds for new episodes",
			},
			{
				Line:        "abs server feeds update",
				Description: "Wake up server and scan all podcast feeds",
			},
			{
				Line:        "abs server feeds update -p 'Huberman Lab'",
				Description: "Wake up server and check feeds for a specific podcast",
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

func buildServerPruneSubcommand(opts *CLIOptions, action *string, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "prune",
		Description: "Delete older episodes per retention limit",
		UsageLine:   "abs server prune [number] [options]",
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
			*action = "server"
			opts.ServerSubcmd = "keep"
			opts.SyncSubcmd = "prune"
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

func buildServerPolicySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "policy",
		Description: "View or update podcast download and AdR policy",
		UsageLine:   "abs server policy <podcast-id> [<number>] [options]",
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
				Line:        "abs server policy 42 1",
				Description: "Shorthand: auto-download latest 1 episode and remove all ads",
			},
			{
				Line:        "abs server policy 'Huberman Lab' 3",
				Description: "Shorthand: auto-download latest 3 episodes and remove all ads",
			},
			{
				Line:        "abs server policy 42 --download-policy all --ad-removal all",
				Description: "Configure podcast 42 to download all episodes and remove ads",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "policy"
			opts.SyncSubcmd = "policy"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildServerListSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "list",
		Description: "List all available podcasts in server with episode counts",
		UsageLine:   "abs server list [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output (Feed URLs and IDs)"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "list"
			opts.SyncSubcmd = "list"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildServerGetInfoSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "get-info",
		Description: "Cache metadata for latest K episodes",
		UsageLine:   "abs server get-info [<k>] [options]",
		Parameters:  []clihelp.Param{{Name: "[<k>]", Description: "Episodes count per podcast (default 100)"}},
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
			clihelp.Bool(&opts.Refresh, "-r, --refresh", false, "Force re-fetching metadata from feed"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed per-episode metadata"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "get-info"
			opts.SyncSubcmd = "get-info"
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
