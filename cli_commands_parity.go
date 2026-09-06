package main

import (
	"github.com/sarielhp/clihelp"
)

func buildInfoCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "info",
		Description: "Inspect podcast or episode metadata and cuts",
		UsageLine:   "abs info [options] <id>",
		Examples: []clihelp.Example{
			{Line: "abs info e12345", Description: "Inspect episode metadata and cuts breakdown"},
		},
		Args: clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
			clihelp.Bool(&opts.ShowCuts, "--cuts", false, "Display detailed breakdown of detected commercial cuts"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildPolicyCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "policy",
		Description: "View or update podcast download and AdR policy",
		UsageLine:   "abs policy [options] <podcast-id>",
		Args:        clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.AutoDownloadStr, "--auto-download <bool>", "", "Enable or disable automatic downloads (true/false)"),
			clihelp.String(&opts.DownloadPolicy, "--download-policy <mode>", "", "Download policy mode ('none', 'latest', 'latest_k', 'all')"),
			clihelp.Int(&opts.DownloadK, "--download-k <num>", 0, "Number of latest episodes to download when policy is latest_k"),
			clihelp.String(&opts.AutoCleanupStr, "--auto-cleanup <bool>", "", "Enable or disable automatic cleanup (true/false)"),
			clihelp.Int(&opts.CleanupDays, "--cleanup-days <days>", 0, "Retention window in days for automatic cleanup"),
			clihelp.String(&opts.AdRemovalMode, "--ad-removal <mode>", "", "Ad removal policy mode ('none', 'latest', 'all')"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "policy"
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

func buildFetchCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "fetch",
		Description: "Fetch and sync latest RSS feeds for podcasts",
		UsageLine:   "abs fetch [options] [podcast-id]",
		Args:        clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "fetch"
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

func buildTranscriptCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "transcript",
		Description: "View or export transcript for an episode",
		UsageLine:   "abs transcript [options] <id>",
		Args:        clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.ExportFormat, "--export <format>", "", "Export format: 'txt' or 'srt'"),
			hideOption(clihelp.Bool(&opts.ExportTXT, "--txt", false, "Export transcript to plain text file")),
			hideOption(clihelp.Bool(&opts.ExportSRT, "--srt", false, "Export transcript to SubRip (SRT) subtitle file")),
			clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output destination path"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress banner and print text only"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "transcript"
			opts.Args = ctx.Args
			return nil
		},
	}
}
