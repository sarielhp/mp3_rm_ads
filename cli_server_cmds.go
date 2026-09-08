package main

import (
	"os"

	"github.com/sarielhp/clihelp"
)

func buildSyncCommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "sync",
		Description: "Sync podcast RSS feeds, episodes, and server policies",
		UsageLine:   "abs sync [command] [options] [podcast-id]",
		Subcommands: buildSyncSubcommands(opts, action, countVal, keepVal),
		Examples: []clihelp.Example{
			{
				Line:        "abs sync",
				Description: "Sync all subscribed podcasts and download new episodes",
			},
			{
				Line:        "abs sync -p 'Huberman Lab' -k 3",
				Description: "Download 3 latest episodes for a specific podcast",
			},
			{
				Line:        "abs sync opml export podcasts.opml",
				Description: "Export server podcast RSS feeds to an OPML file",
			},
		},
		Options: []clihelp.Option{
			clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID"),
			clihelp.Int(countVal, "-k, --count <number>", -1, "Explicit number of episodes to download"),
			clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
			clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
			clihelp.Bool(&opts.Remote, "--remote", false, "Offload audio processing to remote host"),
			clihelp.Bool(&opts.Local, "--local", false, "Force local audio processing"),
			clihelp.Bool(&opts.PodcastsOnly, "--podcasts-only", false, "Only scan for podcasts (skip downloads)"),
			clihelp.Bool(&opts.EpisodesOnly, "--episodes-only", false, "Only check episodes (skip podcast scan)"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "sync"
			if len(ctx.Args) > 0 {
				arg := ctx.Args[0]
				if fi, err := os.Stat(arg); err != nil || !fi.IsDir() {
					if opts.Podcast == "" {
						opts.Podcast = arg
					}
				}
			}
			if *countVal > 0 {
				opts.Count = *countVal
				opts.CountGiven = true
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}
