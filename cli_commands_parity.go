package main

import (
	"github.com/sarielhp/clihelp"
)

func buildInfoCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "info",
		Description: "Library query, inspection, cuts and transcripts",
		UsageLine:   "abs info [options] [id|latest [N]|status|check]",
		Subcommands: []clihelp.Command{
			buildInfoLatestSubcommand(opts, action),
			buildInfoStatusSubcommand(opts, action),
			buildInfoCheckSubcommand(opts, action),
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
			if len(ctx.Args) > 0 {
				switch ctx.Args[0] {
				case "latest":
					opts.InfoSubcmd = "latest"
					opts.Args = ctx.Args[1:]
					return nil
				case "status":
					opts.InfoSubcmd = "status"
					opts.Args = ctx.Args[1:]
					return nil
				case "check":
					opts.InfoSubcmd = "check"
					opts.StatusSubcmd = "check"
					opts.IsTestCommand = true
					args := ctx.Args[1:]
					if len(args) > 0 && args[0] == "kitty" {
						opts.Args = args[1:]
					} else {
						opts.Args = args
					}
					return resolveTestCommandArgs(args, opts)
				}
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildInfoLatestSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "latest",
		Description: "List latest added episodes across all podcasts",
		UsageLine:   "abs info latest [N] [options]",
		Parameters: []clihelp.Param{
			{Name: "[N]", Description: "Number of episodes to show (default: 10)"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Int(&opts.Count, "-n, --limit <number>", 10, "Number of latest episodes to list"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			opts.InfoSubcmd = "latest"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildInfoStatusSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "status",
		Description: "Show status overview of library and worker",
		UsageLine:   "abs info status [options] [podcasts]",
		Args:        clihelp.RangeArgs(0, 2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			opts.InfoSubcmd = "status"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildInfoCheckSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "check",
		Description: "Test external services (Whisper, ABS, Kitty)",
		UsageLine:   "abs info check [options] [target]",
		Args:        clihelp.RangeArgs(0, 2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.TestWhisper, "--test-whisper", false, "Test whisper server connection"),
			clihelp.Bool(&opts.TestABS, "--test-abs", false, "Test Audiobookshelf connection"),
			clihelp.Bool(&opts.TestABSMap, "--test-abs-map", false, "Map local files to ABS metadata"),
			clihelp.Bool(&opts.TestABSDownload, "--test-abs-download", false, "Download all ABS data"),
			clihelp.Bool(&opts.TestKitty, "--test-kitty", false, "Test Kitty cover image display"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			opts.InfoSubcmd = "check"
			opts.StatusSubcmd = "check"
			opts.IsTestCommand = true
			if len(ctx.Args) > 0 && ctx.Args[0] == "kitty" {
				opts.Args = ctx.Args[1:]
			} else {
				opts.Args = ctx.Args
			}
			return resolveTestCommandArgs(ctx.Args, opts)
		},
	}
}

func buildQueueListSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildQueueAddSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildQueueRemoveSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildQueueClearSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
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
	}
}

func buildQueueRunSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "run",
		Description: "Process ad removal for queued episodes",
		UsageLine:   "abs queue run [podcast-id] [options]",
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Quiet mode"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Verbose output"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Simulate processing without making changes"),
			clihelp.Bool(&opts.Local, "--local", false, "Force local execution instead of remote"),
			clihelp.Bool(&opts.Remote, "--remote", false, "Force remote execution"),
			clihelp.String(&opts.RemoteHost, "--remote-host <host>", "", "Specify remote processing host"),
			clihelp.String(&opts.Force, "-f, --force <type>", "", "Force re-processing (all, whisper, llm)"),
			clihelp.String(&opts.UseLLM, "--use-llm <name|id>", "", "Select specific LLM profile"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue run",
				Description: "Process ad removal on all queued episodes",
			},
			{
				Line:        "abs queue run <podcast-id>",
				Description: "Process ad removal for queued episodes of a specific podcast",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "run"
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
			buildQueueListSubcommand(opts, action),
			buildQueueAddSubcommand(opts, action),
			buildQueueRemoveSubcommand(opts, action),
			buildQueueClearSubcommand(opts, action),
			buildQueueRunSubcommand(opts, action),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue list",
				Description: "List all episodes currently in the ad removal queue",
			},
			{
				Line:        "abs queue run",
				Description: "Process ad removal on all queued episodes",
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
