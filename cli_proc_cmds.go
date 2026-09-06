package main

import (
	"github.com/sarielhp/clihelp"
)

func buildProcCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "proc",
		Description: "Process audio files or directories (recursive) for ad removal",
		UsageLine:   "abs proc [<path1> ...] [options]",
		Parameters: []clihelp.Param{
			{Name: "[<path1> ...]", Description: "Optional audio files (.mp3), transcript JSONs (.json), or directories (defaults to configured podcasts_dir)"},
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "collect",
				Description: "Pull completed batch results from remote host and update local library",
				UsageLine:   "abs proc collect [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "proc"
					opts.ProcSubcmd = "collect"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
			{
				Name:        "clear",
				Description: "Stop remote workers and empty all scheduled/pending jobs from remote queue",
				UsageLine:   "abs proc clear [host] [options]",
				Parameters: []clihelp.Param{
					{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "proc"
					opts.ProcSubcmd = "clear"
					if len(ctx.Args) > 0 {
						opts.RemoteHost = ctx.Args[0]
					}
					return nil
				},
			},
		},
		Options: getTranscriptionOptions(opts),
		Run: func(ctx *clihelp.Context) error {
			*action = "proc"
			if len(ctx.Args) > 0 {
				if ctx.Args[0] == "collect" {
					opts.ProcSubcmd = "collect"
					if len(ctx.Args) > 1 {
						opts.RemoteHost = ctx.Args[1]
					}
					return nil
				}
				if ctx.Args[0] == "clear" || ctx.Args[0] == "empty" {
					opts.ProcSubcmd = "clear"
					if len(ctx.Args) > 1 {
						opts.RemoteHost = ctx.Args[1]
					}
					return nil
				}
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildLsCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "ls",
		Description: "List podcast episodes, latest downloads, or podcast contents",
		UsageLine:   "abs ls [latest|podcasts|<podcast>] [options]",
		Subcommands: []clihelp.Command{
			{
				Name:        "podcasts",
				Description: "List all podcasts in the library",
				UsageLine:   "abs ls podcasts [options]",
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress formatting/headers"),
					clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "ls"
					opts.Args = []string{"podcasts"}
					return nil
				},
			},
			{
				Name:        "latest",
				Description: "List the latest downloaded/added episodes across all podcasts",
				UsageLine:   "abs ls latest [N] [options]",
				Parameters: []clihelp.Param{
					{Name: "[N]", Description: "Number of episodes to show (default: 10)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.Int(&opts.Count, "-n, --limit <number>", 10, "Number of latest episodes to list (default: 10)"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress formatting/headers"),
					clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show file sizes and full paths"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "ls"
					opts.LsSubcmd = "latest"
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Parameters: []clihelp.Param{
			{Name: "[latest|podcasts|<podcast>]", Description: "'latest', 'podcasts', or a podcast short ID, index, or name"},
		},
		Options: []clihelp.Option{
			clihelp.Int(&opts.Count, "-n, --limit <number>", 10, "Number of episodes to list (default: 10)"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress formatting/headers"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show file sizes and full paths"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "ls"
			if len(ctx.Args) > 0 && ctx.Args[0] == "latest" {
				opts.LsSubcmd = "latest"
				opts.Args = ctx.Args[1:]
				return nil
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildRecutCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "recut",
		Description: "Recut audio files or directories using existing .cuts.json metadata",
		UsageLine:   "abs recut [<path1> ...] [options]",
		Parameters: []clihelp.Param{
			{Name: "[<path1> ...]", Description: "Optional audio files (.mp3) or directories with existing .cuts.json metadata (defaults to configured podcasts_dir)"},
		},
		Options: []clihelp.Option{
			clihelp.String(&opts.Output, "-o, --output <path>", "", "Output MP3 path or directory"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
			clihelp.String(&opts.RemoteFFmpegHost, "--rffmpeg <host>", "", "Delegate FFmpeg audio cutting to remote SSH host"),
			clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Maximum number of episodes to recut"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "recut"
			opts.Recut = true
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildExportCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "export",
		Description: "Export transcript JSON to SRT subtitles or plain text",
		UsageLine:   "abs export <srt|txt> <path1> [path2 ...] [options]",
		Subcommands: []clihelp.Command{
			{
				Name:        "srt",
				Description: "Export transcript JSON to SubRip subtitle (.srt) format",
				UsageLine:   "abs export srt <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Transcript JSON files or directories to export"},
				},
				Args: clihelp.MinimumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output .srt file path or directory"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "export"
					opts.ExportFormat = "srt"
					opts.ExportSRT = true
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "txt",
				Description: "Export transcript JSON to plain text (.txt) format",
				UsageLine:   "abs export txt <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Transcript JSON files or directories to export"},
				},
				Args: clihelp.MinimumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output .txt file path or directory"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "export"
					opts.ExportFormat = "txt"
					opts.ExportTXT = true
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Args: clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "export"
			opts.Args = ctx.Args
			return nil
		},
	}
}
