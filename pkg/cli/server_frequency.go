package cli

import (
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"fmt"

	"github.com/sarielhp/clihelp"
)

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
