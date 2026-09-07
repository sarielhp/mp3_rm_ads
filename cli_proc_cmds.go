package main

import (
	"github.com/sarielhp/clihelp"
)

func buildRmAdsCommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "rm_ads",
		Description: "Process audio files for ad removal",
		UsageLine:   "abs rm_ads [command]",
		Subcommands: []clihelp.Command{
			buildRmAdsRecutSubcommand(opts, action),
			buildRmAdsExportSubcommand(opts, action),
			buildRmAdsCollectSubcommand(opts, action),
			buildRmAdsClearSubcommand(opts, action),
		},
		Options: getTranscriptionOptions(opts),
		Run: func(ctx *clihelp.Context) error {
			*action = "rm_ads"
			if len(ctx.Args) > 0 {
				switch ctx.Args[0] {
				case "collect":
					opts.ProcSubcmd = "collect"
					if len(ctx.Args) > 1 {
						opts.RemoteHost = ctx.Args[1]
					}
					return nil
				case "clear":
					opts.ProcSubcmd = "clear"
					if len(ctx.Args) > 1 {
						opts.RemoteHost = ctx.Args[1]
					}
					return nil
				case "recut":
					opts.ProcSubcmd = "recut"
					opts.Recut = true
					opts.Args = ctx.Args[1:]
					return nil
				case "export":
					opts.ProcSubcmd = "export"
					opts.Args = ctx.Args[1:]
					return nil
				}
			}
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildRmAdsCollectSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "collect",
		Description: "Pull completed batches from remote host",
		UsageLine:   "abs rm_ads collect [host] [options]",
		Parameters: []clihelp.Param{
			{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "rm_ads"
			opts.ProcSubcmd = "collect"
			if len(ctx.Args) > 0 {
				opts.RemoteHost = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildRmAdsClearSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "clear",
		Description: "Stop remote workers and clear remote queue",
		UsageLine:   "abs rm_ads clear [host] [options]",
		Parameters: []clihelp.Param{
			{Name: "[host]", Description: "Target remote SSH host (defaults to configured remote_host)"},
		},
		Args: clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "rm_ads"
			opts.ProcSubcmd = "clear"
			if len(ctx.Args) > 0 {
				opts.RemoteHost = ctx.Args[0]
			}
			return nil
		},
	}
}

func buildRmAdsRecutSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "recut",
		Description: "Recut audio files using existing cuts metadata",
		UsageLine:   "abs rm_ads recut [options] [path...]",
		Options: []clihelp.Option{
			clihelp.String(&opts.Output, "-o, --output <path>", "", "Output MP3 path or directory"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
			clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
			clihelp.String(&opts.RemoteFFmpegHost, "--rffmpeg <host>", "", "Delegate FFmpeg audio cutting to remote SSH host"),
			clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Maximum number of episodes to recut"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "rm_ads"
			opts.ProcSubcmd = "recut"
			opts.Recut = true
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildRmAdsExportSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "export",
		Description: "Export transcript JSON to SRT subtitles or text",
		UsageLine:   "abs rm_ads export [command] [options] <path...>",
		Subcommands: []clihelp.Command{
			{
				Name:        "srt",
				Description: "Export transcript to SubRip (.srt) subtitle format",
				UsageLine:   "abs rm_ads export srt <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Transcript JSON files or directories to export"},
				},
				Args: clihelp.MinimumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output .srt file path or directory"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "rm_ads"
					opts.ProcSubcmd = "export"
					opts.ExportFormat = "srt"
					opts.ExportSRT = true
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "txt",
				Description: "Export transcript to plain text (.txt) format",
				UsageLine:   "abs rm_ads export txt <path1> [path2 ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "<path1> [path2 ...]", Description: "Transcript JSON files or directories to export"},
				},
				Args: clihelp.MinimumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output .txt file path or directory"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "rm_ads"
					opts.ProcSubcmd = "export"
					opts.ExportFormat = "txt"
					opts.ExportTXT = true
					opts.Args = ctx.Args
					return nil
				},
			},
		},
		Options: []clihelp.Option{
			clihelp.String(&opts.ExportFormat, "--format <format>", "", "Export format: 'srt' or 'txt'"),
			clihelp.String(&opts.Output, "-o, --output <path>", "", "Custom output file path or directory"),
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		},
		Args: clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "rm_ads"
			opts.ProcSubcmd = "export"
			opts.Args = ctx.Args
			return nil
		},
	}
}
