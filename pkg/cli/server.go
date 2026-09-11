package cli

import (
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"fmt"
	"os"
	"strings"

	"github.com/sarielhp/clihelp"
)

func buildServerCommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "server",
		Description: "Manage podcast server (Audiobookshelf / PodFetch) feeds, downloads, and policies",
		UsageLine:   "abs server [command] [options] [podcast-id]",
		Subcommands: buildServerSubcommands(opts, action, countVal, keepVal),
		Examples: []clihelp.Example{
			{
				Line:        "abs server feeds",
				Description: "Check podcast feeds directly for newly published episodes",
			},
			{
				Line:        "abs server download -p 'Huberman Lab' -k 3",
				Description: "Download the 3 latest episodes for a specific podcast",
			},
			{
				Line:        "abs server opml export podcasts.opml",
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
			*action = "server"
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

func buildServerSubcommands(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {
	return []clihelp.Command{
		buildServerFeedsSubcommand(opts, action),
		buildServerDownloadSubcommand(opts, action, countVal, keepVal),
		buildServerPruneSubcommand(opts, action, keepVal),
		buildServerPolicySubcommand(opts, action),
		buildServerListSubcommand(opts, action),
		buildServerGetInfoSubcommand(opts, action),
		buildServerRescanSubcommand(opts, action),
		buildServerTimelineSubcommand(opts, action),
		buildServerOPMLSubcommand(opts, action),
		buildServerFrequencySubcommand(opts, action),
		buildServerDisableHourlySubcommand(opts, action),
		buildServerCleanOrphansSubcommand(opts, action),
		buildServerFlushSubcommand(opts, action),
		buildServerPublicationSubcommand(opts, action),
	}
}

func handleServerCommand(config Config, cli CLIOptions) error {
	subcmd := cli.ServerSubcmd
	if subcmd == "" {
		subcmd = cli.SyncSubcmd
	}
	switch subcmd {
	case "":
		showServerUsage()
		return nil
	case "feeds":
		return handleServerFeeds(config, cli)
	case "download":
		return handleServerDownload(config, cli)
	case "prune", "keep":
		return handleServerKeep(config, cli)
	case "policy":
		return runPolicyCommand(config, cli)
	case "list":
		return handleServerList(config, cli)
	case "get-info", "get_info":
		return handleServerGetInfo(config, cli)
	case "rescan":
		return handleServerRescan(config, cli)
	case "timeline":
		return handleServerTimeline(config, cli)
	case "opml":
		return handleServerOPML(config, cli)
	case "frequency":
		return handleServerFrequency(config, cli)
	case "disable-hourly", "disable_hourly":
		return handleServerDisableHourly(config, cli)
	case "clean-orphans":
		return handleServerCleanOrphans(config, cli)
	case "flush":
		return handleServerFlush(config, cli)
	case "publication-sync":
		return handleServerPublication(config, cli)
	default:
		return fmt.Errorf("unknown server subcommand %q", subcmd)
	}
}

func showServerUsage() {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)
	_ = app.RenderCommand(clihelp.Options{}, "server")
}

func matchBackendPodcast(podcasts []backend.Podcast, target string) (*backend.Podcast, error) {
	return podcast.MatchBackendPodcasts(podcasts, target)
}

func resolveServerTargetPodcasts(b backend.Backend, cli CLIOptions) ([]backend.Podcast, error) {
	podcasts, err := b.Podcasts()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch podcasts from server: %w", err)
	}
	return filterServerTargets(podcasts, cli)
}

func serverTargetName(cli CLIOptions) string {
	target := cli.Podcast
	if target == "" && len(cli.Args) > 0 {
		if cli.Args[0] != "update" {
			target = cli.Args[0]
		} else if len(cli.Args) > 1 {
			target = cli.Args[1]
		}
	}
	return target
}

func filterServerTargets(podcasts []backend.Podcast, cli CLIOptions) ([]backend.Podcast, error) {
	if target := serverTargetName(cli); target != "" {
		matched, err := matchBackendPodcast(podcasts, target)
		if err != nil {
			return nil, err
		}
		return []backend.Podcast{*matched}, nil
	}
	var active []backend.Podcast
	for _, p := range podcasts {
		if strings.TrimSpace(p.Media.Metadata.FeedURL) != "" {
			active = append(active, p)
		}
	}
	if len(active) > 0 {
		return active, nil
	}
	return podcasts, nil
}
