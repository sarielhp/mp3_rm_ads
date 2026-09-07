package main

import (
	"github.com/sarielhp/clihelp"
)

func buildInfoCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "info",
		Description: "Library query, inspection, cuts and transcripts",
		UsageLine:   "abs info [options] [id|latest [N]]",
		Subcommands: []clihelp.Command{
			{
				Name:        "latest",
				Description: "List latest added episodes across all podcasts",
				UsageLine:   "abs info latest [N] [options]",
				Parameters: []clihelp.Param{
					{Name: "[N]", Description: "Number of episodes to show (default: 10)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Int(&opts.Count, "-n, --limit <number>", 10, "Number of latest episodes to list"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress formatting/headers"),
					clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "info"
					opts.InfoSubcmd = "latest"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Args: clihelp.RangeArgs(0, 2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
			clihelp.Bool(&opts.ShowCuts, "--cuts", false, "Display detailed cuts breakdown"),
			clihelp.Bool(&opts.ShowTranscript, "--transcript", false, "Display episode transcript text"),
			clihelp.String(&opts.ExportFormat, "--export <format>", "", "Export transcript to format ('srt' or 'txt')"),
			clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Limit number of episodes to list"),
			clihelp.Bool(&opts.Latest, "-l, --latest", false, "List latest episodes across library"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress formatting/headers"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
			clihelp.String(&opts.Output, "-o, --output <path>", "", "Output destination for export"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			if len(ctx.Args) > 0 && ctx.Args[0] == "latest" {
				opts.InfoSubcmd = "latest"
				opts.Args = ctx.Args[1:]
				return nil
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildQueueCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "queue",
		Description: "Manage the ad removal (AdR) processing queue",
		UsageLine:   "abs queue [command]",
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Description: "List queued episodes across library or podcast",
				UsageLine:   "abs queue list [podcast-id] [options]",
				Args:        clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress table formatting and print paths only"),
					clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "queue"
					opts.QueueSubcmd = "list"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "add",
				Description: "Add episodes or podcast uncleaned episodes to queue",
				UsageLine:   "abs queue add <id...>",
				Args:        clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					*action = "queue"
					opts.QueueSubcmd = "add"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "remove",
				Description: "Remove one or more episodes from the queue",
				UsageLine:   "abs queue remove <id...>",
				Args:        clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					*action = "queue"
					opts.QueueSubcmd = "remove"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "clear",
				Description: "Clear queue for a specific podcast or all podcasts",
				UsageLine:   "abs queue clear [podcast-id]",
				Args:        clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					*action = "queue"
					opts.QueueSubcmd = "clear"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildPlayerCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "player",
		Description: "Control background audio playback",
		UsageLine:   "abs player [command]",
		Subcommands: []clihelp.Command{
			{
				Name:        "play",
				Description: "Play an episode or resume playback",
				UsageLine:   "abs player play [id]",
				Args:        clihelp.RangeArgs(0, 1),
				Run: func(ctx *clihelp.Context) error {
					*action = "player"
					opts.PlayerSubcmd = "play"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "stop",
				Description: "Stop background audio playback",
				UsageLine:   "abs player stop",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					*action = "player"
					opts.PlayerSubcmd = "stop"
					return nil
				},
			},
			{
				Name:        "pause",
				Description: "Toggle playback pause state",
				UsageLine:   "abs player pause",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					*action = "player"
					opts.PlayerSubcmd = "pause"
					return nil
				},
			},
			{
				Name:        "status",
				Description: "Display player status and progress",
				UsageLine:   "abs player status",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					*action = "player"
					opts.PlayerSubcmd = "status"
					return nil
				},
			},
			{
				Name:        "daemon",
				Hidden:      true,
				Description: "Internal background player daemon",
				Run: func(ctx *clihelp.Context) error {
					*action = "player"
					opts.PlayerSubcmd = "daemon"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Args: clihelp.RangeArgs(0, 2),
		Run: func(ctx *clihelp.Context) error {
			*action = "player"
			opts.Args = ctx.Args
			return nil
		},
	}
}
