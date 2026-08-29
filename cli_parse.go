package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sarielhp/clihelp"
)

func isCommandPathOrPrefix(app *clihelp.App, args []string) bool {
	if len(args) == 0 {
		return false
	}
	currentCmds := app.Commands
	for idx, arg := range args {
		arg = strings.ToLower(arg)
		var found *clihelp.Command
		for i := range currentCmds {
			if strings.ToLower(currentCmds[i].Name) == arg {
				found = &currentCmds[i]
				break
			}
		}
		if found != nil {
			currentCmds = found.Subcommands
			continue
		}

		hasPrefixMatch := false
		for i := range currentCmds {
			if strings.HasPrefix(strings.ToLower(currentCmds[i].Name), arg) {
				hasPrefixMatch = true
				break
			}
		}
		if hasPrefixMatch {
			if idx == len(args)-1 {
				return true
			}
		}
		return false
	}
	return true
}

func normalizeCLIArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	firstArg := strings.ToLower(args[0])

	var dummyAct string
	var dummyOpts CLIOptions
	dummyApp := buildCLIApp(&dummyAct, &dummyOpts)
	if isCommandPathOrPrefix(dummyApp, []string{firstArg}) {
		return args
	}

	if _, err := strconv.Atoi(args[0]); err == nil {
		return append([]string{"server", "download"}, args...)
	}

	hasMp3 := false
	hasDir := false
	var lastArgNum bool

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(arg), ".mp3") || strings.HasSuffix(strings.ToLower(arg), ".json") {
			hasMp3 = true
			break
		}
		if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
			hasDir = true
			break
		}
	}

	if len(args) > 0 {
		last := args[len(args)-1]
		if _, err := strconv.Atoi(last); err == nil {
			lastArgNum = true
		}
	}

	if hasMp3 || hasDir {
		return append([]string{"proc"}, args...)
	}
	if lastArgNum {
		return append([]string{"server", "download"}, args...)
	}

	return args
}

func getVersion() string {
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		if data, err := os.ReadFile(filepath.Join(execDir, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "0.1.26"
}

func buildCLIApp(action *string, opts *CLIOptions) *clihelp.App {
	keepVal := -1
	countVal := -1

	app := &clihelp.App{
		Name:                "abs",
		Description:         "Automatic Ad Segment Remover & Podcast Manager",
		Version:             getVersion(),
		GlobalNote:          "Run 'abs <command> --help' or 'abs help <command>' for command-specific options.",
		AbbrevCommands:      true,
		Pager:               true,
		InteractiveFallback: true,
		Commands: []clihelp.Command{
			{
				Name:        "proc",
				Description: "Process audio files or directories (recursive) for ad removal",
				UsageLine:   "abs proc [<path1> ...] [options]",
				Parameters: []clihelp.Param{
					{Name: "[<path1> ...]", Description: "Optional audio files (.mp3), transcript JSONs (.json), or directories (defaults to configured podcasts_dir)"},
				},
				Options: getTranscriptionOptions(opts),
				Run: func(ctx *clihelp.Context) error {
					*action = "proc"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
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
					clihelp.String(&opts.RemoteFFmpegHost, "--remote-ffmpeg <host>", "", "Delegate FFmpeg audio cutting to remote SSH host"),
					clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Maximum number of episodes to recut"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "recut"
					opts.Recut = true
					opts.Args = ctx.Args
					return nil
				},
			},
			{
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
					{
						Name:        "opml",
						Description: "Generate an OPML file containing RSS feeds of all podcasts on Audiobookshelf",
						UsageLine:   "abs export opml [options]",
						Options: []clihelp.Option{
							clihelp.String(&opts.Output, "-o, --output <file>", "", "Output OPML file path (default: stdout)"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed progress output"),
							clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress status outputs"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "opml"
							opts.Args = ctx.Args
							return nil
						},
					},
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "export"
					if len(ctx.Args) > 0 {
						switch strings.ToLower(ctx.Args[0]) {
						case "srt":
							opts.ExportFormat = "srt"
							opts.ExportSRT = true
							opts.Args = ctx.Args[1:]
						case "txt":
							opts.ExportFormat = "txt"
							opts.ExportTXT = true
							opts.Args = ctx.Args[1:]
						case "opml":
							*action = "server"
							opts.ServerSubcmd = "opml"
							opts.Args = ctx.Args[1:]
						default:
							opts.ExportFormat = "srt"
							opts.ExportSRT = true
							opts.Args = ctx.Args
						}
					}
					return nil
				},
			},
			{
				Name:        "opml",
				Description: "Generate an OPML file containing RSS feeds of all podcasts on Audiobookshelf",
				UsageLine:   "abs opml [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Output, "-o, --output <file>", "", "Output OPML file path (default: stdout)"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed progress output"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress status outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "opml"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "tui",
				Description: "Interactive TUI browser for podcasts and episodes",
				UsageLine:   "abs tui [directory]",
				Parameters: []clihelp.Param{
					{Name: "[directory]", Description: "Optional path to podcasts directory (defaults to configured podcasts_dir)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.PodcastsDir, "--podcasts-dir <dir>", "", "Podcasts directory"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "tui"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "scan",
				Description: "Scan Audiobookshelf for new podcasts and check for new episodes",
				UsageLine:   "abs scan [podcasts_dir] [options]",
				Parameters: []clihelp.Param{
					{Name: "[podcasts_dir]", Description: "Optional podcasts directory path (defaults to configured podcasts_dir)"},
				},
				Args:             clihelp.MaximumNArgs(1),
				OptionsValidator: clihelp.MutuallyExclusive("--podcasts-only", "--episodes-only"),
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID to check/download new episodes"),
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
					clihelp.Bool(&opts.PodcastsOnly, "--podcasts-only", false, "Only scan for new podcasts and create directories (skip episode downloads)"),
					clihelp.Bool(&opts.EpisodesOnly, "--episodes-only", false, "Only check and download new episodes (skip podcast folder scanning)"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "scan"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "status",
				Description: "Show a dry-run status report of all podcasts and episodes needing ad removal",
				UsageLine:   "abs status [podcasts_dir]",
				Parameters: []clihelp.Param{
					{Name: "[podcasts_dir]", Description: "Optional path to podcasts directory"},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					*action = "status"
					opts.Args = ctx.Args
					return nil
				},
			},

			{
				Name:        "test",
				Description: "Test external services like Whisper server or Audiobookshelf",
				UsageLine:   "abs test <whisper|abs [connect|map|download]|kitty <image>>",
				Parameters: []clihelp.Param{
					{Name: "<whisper|abs|kitty>", Description: "Target service/device to test ('whisper', 'abs', or 'kitty')"},
					{Name: "[args]", Description: "Optional test arguments (e.g., 'map' or 'download' for abs, or path to an image file for kitty)"},
				},
				Args: clihelp.RangeArgs(0, 2),
				Options: []clihelp.Option{
					clihelp.Bool(&opts.TestWhisper, "--test-whisper", false, "Test whisper server connection"),
					clihelp.Bool(&opts.TestABS, "--test-abs", false, "Test Audiobookshelf connection"),
					clihelp.Bool(&opts.TestABSMap, "--test-abs-map", false, "Map local files to ABS metadata"),
					clihelp.Bool(&opts.TestABSDownload, "--test-abs-download", false, "Download all ABS data"),
					clihelp.Bool(&opts.TestKitty, "--test-kitty", false, "Test Kitty cover image display"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "test"
					opts.Args = ctx.Args
					if len(ctx.Args) > 0 {
						switch ctx.Args[0] {
						case "whisper", "whisper-server":
							opts.TestWhisper = true
						case "abs":
							if len(ctx.Args) > 1 {
								switch ctx.Args[1] {
								case "map":
									opts.TestABSMap = true
								case "download":
									opts.TestABSDownload = true
								default:
									opts.TestABS = true
								}
							} else {
								opts.TestABS = true
							}
						case "kitty":
							opts.TestKitty = true
						}
					}
					if !opts.TestWhisper && !opts.TestABS && !opts.TestABSMap && !opts.TestABSDownload && !opts.TestKitty {
						opts.TestWhisper = true
					}
					return nil
				},
			},
			buildServerCommand(opts, action, &countVal, &keepVal),
			buildConfigCommand(opts, action),
			{
				Name:        "help",
				Description: "Display usage help message for abs or a specific command",
				UsageLine:   "abs help [<command>]",
				Run: func(ctx *clihelp.Context) error {
					if len(ctx.Args) > 0 {
						topic := strings.ToLower(ctx.Args[0])
						if topic == "tree" || topic == "--tree" || topic == "t" {
							ctx.App.RenderTree(clihelp.Options{Theme: ctx.App.Theme, Pager: ctx.App.Pager})
							os.Exit(0)
						}
						ctx.App.RenderCommand(clihelp.Options{Theme: ctx.App.Theme, Pager: ctx.App.Pager}, ctx.Args...)
					} else {
						ctx.App.RenderGlobal(clihelp.Options{Theme: ctx.App.Theme, Pager: ctx.App.Pager})
					}
					os.Exit(0)
					return nil
				},
			},
		},
	}
	return app
}

func getTranscriptionOptions(opts *CLIOptions) []clihelp.Option {
	return []clihelp.Option{
		clihelp.String(&opts.Output, "-o, --output <path>", "", "Output MP3 path or directory"),
		clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		clihelp.BoolToggle(&opts.SaveTranscript, "--[no-]transcript", true, "Save default .transcript.json file"),
		clihelp.Bool(&opts.UseChunks, "--use-chunks", false, "Split long audio into chunks for Whisper"),
		clihelp.Bool(&opts.ExtractKeywords, "--extract-keywords", false, "Extract keywords to improve transcription"),
		clihelp.String(&opts.TranscribeMin, "-t, --transcribe-minutes <minutes>", "", "Only transcribe first N minutes (e.g. 10m)"),
		clihelp.String(&opts.Force, "-f, --force <stage>", "", "Force re-running: 'whisper', 'llm', or 'all'"),
		clihelp.String(&opts.UseLLM, "--profile <id_or_name>", "", "Select LLM profile ID or name"),
		clihelp.String(&opts.RemoteFFmpegHost, "--remote-ffmpeg <host>", "", "Delegate FFmpeg audio cutting to remote SSH host (e.g. cloud8)"),
		clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Maximum number of untranscribed episodes to process"),
	}
}

func parseFlags() (string, CLIOptions) {
	var action string
	opts := CLIOptions{
		SaveTranscript: true,
	}

	args := os.Args[1:]
	normArgs := normalizeCLIArgs(args)

	for _, a := range normArgs {
		if a == "--tree" {
			app := buildCLIApp(&action, &opts)
			app.RenderTree(clihelp.Options{})
			os.Exit(0)
		}
	}

	app := buildCLIApp(&action, &opts)
	err := app.Execute(normArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if action == "" {
		os.Exit(0)
	}

	opts.IsConfigCommand = (action == "config")
	opts.IsDirCommand = (action == "dir")
	opts.IsFileCommand = (action == "proc" || action == "recut")
	opts.IsTUICommand = (action == "tui")
	opts.IsTimelineCommand = (action == "timeline" || (action == "server" && opts.ServerSubcmd == "timeline"))
	opts.IsTestCommand = (action == "test")
	opts.IsScanCommand = (action == "scan")
	opts.IsStatusCommand = (action == "status")

	return action, opts
}
