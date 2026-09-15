package cli

import (
	"fmt"
	"pod/pkg/backend"
	"strconv"

	"github.com/sarielhp/clihelp"
)

func buildServerPruneSubcommand(opts *CLIOptions, action *string, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "prune",
		Description: "Delete older episodes per retention limit",
		UsageLine:   "pod server prune [number] [options]",
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

func handleServerKeep(config Config, cli CLIOptions) error {
	b, err := backend.FromAppConfig(&config, reporter(cli))
	if err != nil {
		return fmt.Errorf("podcast server not configured: %w", err)
	}
	podcasts, err := resolveServerTargetPodcasts(b, cli)
	if err != nil {
		return err
	}
	keep := -1
	if cli.KeepCount != nil {
		keep = *cli.KeepCount
	}
	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = "Untitled"
		}
		deleted, err := b.ApplyKeepPolicy(item.ID, title, keep, cli.DryRun)
		if err != nil && !cli.Quiet {
			fmt.Printf("! Error applying keep policy to %s: %v\n", title, err)
		} else if !cli.Quiet {
			fmt.Printf("✓ %s: pruned %d episode(s) (limit: %d)\n", title, deleted, keep)
		}
	}
	return nil
}
