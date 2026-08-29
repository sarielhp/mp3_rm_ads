package main

import (
	"fmt"
	"strconv"

	"github.com/sarielhp/clihelp"
)

func buildServerCommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "server",
		Description: "Manage and interact with the Audiobookshelf server",
		Subcommands: []clihelp.Command{
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
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
					clihelp.Bool(&opts.PodcastsOnly, "--podcasts-only", false, "Only scan for new podcasts and create directories (skip episode downloads)"),
					clihelp.Bool(&opts.EpisodesOnly, "--episodes-only", false, "Only check and download new episodes (skip podcast folder scanning)"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "scan"
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
					clihelp.Bool(&opts.Fill, "-f, --fill", false, "Fill gaps in downloaded episodes"),
					clihelp.Int(keepVal, "-K, --keep <number>", -1, "Enforce keep count policies"),
					clihelp.BoolToggle(&opts.CheckNew, "--[no-]check-new", true, "Check new episodes published"),
					clihelp.Bool(&opts.Oldest, "--oldest", false, "Download oldest first"),
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed info"),
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
				Name:        "rescan",
				Description: "Scan MP3 file lengths on disk against DB duration and update DB if shorter",
				UsageLine:   "abs server rescan [options]",
				Args:        clihelp.NoArgs,
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by index or title"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview actions without updating DB"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed trace outputs"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.String(&opts.SqliteDBPath, "--db-path <path>", "", "Path to absdatabase.sqlite"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "rescan"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "timeline",
				Description: "Display exact online availability timestamps table for recent podcast episodes",
				UsageLine:   "abs server timeline [directory]",
				Parameters: []clihelp.Param{
					{Name: "[directory]", Description: "Optional path to podcasts directory"},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "timeline"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "opml",
				Description: "Generate an OPML file containing RSS feeds of all podcasts on Audiobookshelf",
				UsageLine:   "abs server opml [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <file>", "", "Output OPML file path (default: stdout)"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed progress output"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress status outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "opml"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
	}
}
