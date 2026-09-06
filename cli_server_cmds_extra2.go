package main

import (
	"strings"

	"github.com/sarielhp/clihelp"
)

func buildServerSubcommands2(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {
	return []clihelp.Command{
		buildServerRescanSubcommand(opts, action),
		buildServerTimelineSubcommand(opts, action),
		buildServerOPMLSubcommand(opts, action),
		buildServerFrequencySubcommand(opts, action),
		buildServerDisableHourlySubcommand(opts, action),
		buildServerCleanOrphansSubcommand(opts, action),
	}
}

func buildServerRescanSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildServerTimelineSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildServerOPMLSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "opml",
		Description: "Import or export podcast subscriptions using OPML files",
		UsageLine:   "abs server opml <command> [args]",
		Subcommands: []clihelp.Command{
			{
				Name:        "import",
				Description: "Import podcast subscriptions from an OPML file into Audiobookshelf",
				UsageLine:   "abs server opml import <file> [options]",
				Parameters:  []clihelp.Param{{Name: "<file>", Description: "Path to the OPML file to import"}},
				Args:        clihelp.ExactArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "opml"
					opts.OPMLSubcmd = "import"
					if len(ctx.Args) > 0 {
						opts.OPMLFile = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "export",
				Description: "Export all Audiobookshelf podcast RSS feeds into an OPML file",
				UsageLine:   "abs server opml export <file> [options]",
				Parameters:  []clihelp.Param{{Name: "<file>", Description: "Path to write the exported OPML file"}},
				Args:        clihelp.ExactArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "opml"
					opts.OPMLSubcmd = "export"
					if len(ctx.Args) > 0 {
						opts.OPMLFile = ctx.Args[0]
					}
					return nil
				},
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "opml"
			if len(ctx.Args) > 0 {
				switch strings.ToLower(ctx.Args[0]) {
				case "import":
					opts.OPMLSubcmd = "import"
					if len(ctx.Args) > 1 {
						opts.OPMLFile = ctx.Args[1]
					}
				case "export":
					opts.OPMLSubcmd = "export"
					if len(ctx.Args) > 1 {
						opts.OPMLFile = ctx.Args[1]
					}
				default:
					opts.OPMLFile = ctx.Args[0]
				}
			}
			return nil
		},
	}
}

func buildServerFrequencySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "frequency",
		Description: "Analyze podcast release cadence and save frequency metadata",
		UsageLine:   "abs server frequency [<podcast>] [options]",
		Parameters: []clihelp.Param{
			{Name: "[<podcast>]", Description: "Optional podcast by name, index, or ID to analyze"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Filter to a single podcast"),
			clihelp.Bool(&opts.Refresh, "--refresh", false, "Force re-fetching latest 100 episodes from online RSS feed"),
			clihelp.Bool(&opts.DisableHourly, "--disable-hourly", false, "Automatically disable download and ad removal for hourly podcasts"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed metrics (span, days interval, hours interval)"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "frequency"
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 && opts.Podcast == "" {
				opts.Podcast = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildServerDisableHourlySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "disable-hourly",
		Aliases:     []string{"disable_hourly"},
		Description: "Disable policy for hourly podcasts",
		UsageLine:   "abs server disable-hourly [options]",
		Parameters: []clihelp.Param{
			{Name: "[<podcast>]", Description: "Podcast name, index, or ID"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Filter to a single podcast"),
			clihelp.Bool(&opts.Refresh, "--refresh", false, "Re-fetch latest episodes from feed"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "disable-hourly"
			opts.DisableHourly = true
			opts.Args = ctx.Args
			if len(ctx.Args) > 0 && opts.Podcast == "" {
				opts.Podcast = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildServerCleanOrphansSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "clean-orphans",
		Description: "Delete orphaned ABS podcast entries",
		UsageLine:   "abs server clean-orphans [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview items without deleting"),
			clihelp.Bool(&opts.ForceDelete, "-f, --force", false, "Delete items without confirmation"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output during pruning"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "clean-orphans"
			opts.Args = ctx.Args
			return nil
		},
	}
}
