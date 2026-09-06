package main

import (
	"github.com/sarielhp/clihelp"
)

func buildProcCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "proc",
		Description: "Process audio files for ad removal",
		UsageLine:   "abs proc [command]",
		Subcommands: []clihelp.Command{
			{
				Name:        "collect",
				Description: "Pull completed batches from remote host",
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
				Description: "Stop remote workers and clear remote queue",
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
				if ctx.Args[0] == "clear" {
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
		Description: "List library podcasts, episodes, or latest downloads",
		UsageLine:   "abs ls [command]",
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
				Description: "List latest added episodes across all podcasts",
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
		Description: "Recut audio files using existing cuts metadata",
		UsageLine:   "abs recut [options] [path...]",
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
		Description: "Export transcript JSON to SRT subtitles or text",
		UsageLine:   "abs export [command]",
		Subcommands: []clihelp.Command{
			{
				Name:        "srt",
				Description: "Export transcript to SubRip (.srt) subtitle format",
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
				Description: "Export transcript to plain text (.txt) format",
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
