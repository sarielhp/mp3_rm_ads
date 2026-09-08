package main

import (
	"strings"

	"github.com/sarielhp/clihelp"
)

func buildSyncSubcommands2(opts *CLIOptions, action *string) []clihelp.Command {
	return []clihelp.Command{
		buildSyncRescanSubcommand(opts, action),
		buildSyncTimelineSubcommand(opts, action),
		buildSyncOPMLSubcommand(opts, action),
		buildServerFrequencySubcommand(opts, action),
		buildServerDisableHourlySubcommand(opts, action),
		buildServerCleanOrphansSubcommand(opts, action),
	}
}

func buildSyncRescanSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "rescan",
		Description: "Scan MP3 file lengths on disk against DB duration and update DB if shorter",
		UsageLine:   "abs sync rescan [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by index or title"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview actions without updating DB"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed trace outputs"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.String(&opts.SqliteDBPath, "--db-path <path>", "", "Path to absdatabase.sqlite"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "rescan"
			opts.ServerSubcmd = "rescan"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildSyncTimelineSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "timeline",
		Description: "Display exact online availability timestamps table for recent podcast episodes",
		UsageLine:   "abs sync timeline [directory]",
		Parameters: []clihelp.Param{
			{Name: "[directory]", Description: "Optional path to podcasts directory"},
		},
		Args: clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "timeline"
			opts.ServerSubcmd = "timeline"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildSyncOPMLImportSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "import",
		Description: "Import podcast subscriptions from an OPML file into the server",
		UsageLine:   "abs sync opml import <file> [options]",
		Parameters:  []clihelp.Param{{Name: "<file>", Description: "Path to the OPML file to import"}},
		Args:        clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs sync opml import subscriptions.opml",
				Description: "Import podcasts from subscriptions.opml into the server",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "opml"
			opts.ServerSubcmd = "opml"
			opts.OPMLSubcmd = "import"
			if len(ctx.Args) > 0 {
				opts.OPMLFile = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildSyncOPMLExportSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "export",
		Description: "Export podcast RSS feeds provided by the server into an OPML file",
		UsageLine:   "abs sync opml export <file> [options]",
		Parameters:  []clihelp.Param{{Name: "<file>", Description: "Path to write the exported OPML file"}},
		Args:        clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs sync opml export podcasts.opml",
				Description: "Export all server podcast feeds into podcasts.opml",
			},
			{
				Line:        "abs sync opml export ~/Downloads/podcasts.opml --verbose",
				Description: "Export feeds with detailed progress and show each feed URL",
			},
		},
		Notes: []clihelp.Note{
			{
				Heading: "What This Command Does",
				Text:    "Queries the active podcast server (Audiobookshelf or PodFetch) for all hosted podcast RSS feeds and compiles them into a standard OPML 2.0 XML file. This allows subscribing to your entire library on the server in any podcast player app in one swoop without adding feeds one by one.",
			},
			{
				Heading: "Importing Into AntennaPod",
				Text:    "To import all server podcasts into AntennaPod at once:\n1. Run 'abs sync opml export podcasts.opml' and transfer the file to your mobile device (via Nextcloud, Syncthing, email, or USB).\n2. Open AntennaPod on your device.\n3. Navigate to Subscriptions -> tap the top-right menu (⋮) -> 'Import/Export'.\n4. Select 'OPML import' and choose the exported 'podcasts.opml' file.\n5. AntennaPod will subscribe to all server-provided podcast feeds in one single step.",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "opml"
			opts.ServerSubcmd = "opml"
			opts.OPMLSubcmd = "export"
			if len(ctx.Args) > 0 {
				opts.OPMLFile = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildSyncOPMLSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "opml",
		Description: "Import or export podcast subscriptions using OPML files",
		UsageLine:   "abs sync opml <command> [args]",
		Subcommands: []clihelp.Command{
			buildSyncOPMLImportSubcommand(opts, action),
			buildSyncOPMLExportSubcommand(opts, action),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs sync opml export podcasts.opml",
				Description: "Export all server podcast feeds into an OPML file",
			},
			{
				Line:        "abs sync opml import subscriptions.opml",
				Description: "Import podcasts from an OPML file into the server",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "opml"
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
		UsageLine:   "abs sync frequency [<podcast>] [options]",
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
			*action = "sync"
			opts.SyncSubcmd = "frequency"
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
		Description: "Disable policy for hourly podcasts",
		UsageLine:   "abs sync disable-hourly [options]",
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
			*action = "sync"
			opts.SyncSubcmd = "disable-hourly"
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
		UsageLine:   "abs sync clean-orphans [options]",
		Args:        clihelp.NoArgs,
		Options: []clihelp.Option{
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview items without deleting"),
			clihelp.Bool(&opts.ForceDelete, "-f, --force", false, "Delete items without confirmation"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output during pruning"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			opts.SyncSubcmd = "clean-orphans"
			opts.Args = ctx.Args
			return nil
		},
	}
}
