package main

import (
	"github.com/sarielhp/clihelp"
)

func buildInfoCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "info",
		Description: "Deep inspection of a podcast or episode by short ID, title, or filename",
		UsageLine:   "abs info <id> [options]",
		Parameters: []clihelp.Param{
			{Name: "<id>", Description: "Podcast short ID (5 chars), episode short ID (eXXXXX), title, or path"},
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
		Description: "View or update auto-download, auto-cleanup, and ad removal policy for a podcast",
		UsageLine:   "abs policy <podcast-id> [options]",
		Parameters: []clihelp.Param{
			{Name: "<podcast-id>", Description: "Podcast short ID (5 chars), index, or title"},
		},
		Args: clihelp.ExactArgs(1),
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
		Description: "Manage ad removal queue (list, add, remove, or clear)",
		UsageLine:   "abs queue <list|add|remove|clear> [ids] [options]",
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Description: "List queued episodes across all podcasts or for a specific podcast",
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
				Description: "Add one or more episodes (or all uncleaned episodes of a podcast) to the queue",
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
				Description: "Clear queue for a specific podcast or for all podcasts",
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
		Parameters: []clihelp.Param{
			{Name: "<subcommand>", Description: "'list', 'add', 'remove', or 'clear'"},
			{Name: "[ids]", Description: "Podcast or episode IDs"},
		},
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress table formatting"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
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
		Description: "Fetch and refresh RSS feed for a podcast or all podcasts",
		UsageLine:   "abs fetch [podcast-id]",
		Parameters: []clihelp.Param{
			{Name: "[podcast-id]", Description: "Optional podcast short ID, index, or name (defaults to all)"},
		},
		Args: clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "fetch"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildPlayCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "play",
		Description: "Play episode audio by episode ID",
		UsageLine:   "abs play <episode-id>",
		Parameters: []clihelp.Param{
			{Name: "<episode-id>", Description: "Episode short ID (eXXXXX), filename, or path"},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "play"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildTranscriptCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "transcript",
		Description: "View or export transcript for an episode",
		UsageLine:   "abs transcript <episode-id> [options]",
		Parameters: []clihelp.Param{
			{Name: "<episode-id>", Description: "Episode short ID (eXXXXX), filename, or path"},
		},
		Args: clihelp.ExactArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.ExportFormat, "--export <format>", "", "Export format: 'txt' or 'srt'"),
			clihelp.Bool(&opts.ExportTXT, "--txt", false, "Export transcript to plain text file"),
			clihelp.Bool(&opts.ExportSRT, "--srt", false, "Export transcript to SubRip (SRT) subtitle file"),
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
