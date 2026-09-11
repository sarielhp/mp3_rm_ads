package cli

import (
	"abs/pkg/backend"
	"abs/pkg/util"
	"fmt"

	"github.com/sarielhp/clihelp"
)

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
			opts.SyncSubcmd = "timeline"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func handleServerTimeline(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, cli.Quiet)
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	for _, item := range podcasts {
		if !cli.Quiet {
			fmt.Printf("\nTimeline for %s (%d episodes):\n", util.Bold(item.Media.Metadata.Title), len(item.Media.Episodes))
		}
		for i, ep := range item.Media.Episodes {
			if i >= 10 && !cli.Verbose {
				break
			}
			fmt.Printf("  - %s (%s)\n", ep.Title, ep.PubDate)
		}
	}
	return nil
}
