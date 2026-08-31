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
			buildProcCommand(opts, action),
			buildLsCommand(opts, action),
			buildRecutCommand(opts, action),
			buildExportCommand(opts, action),
			{
				Name:        "opml",
				Description: "Import or export podcast subscriptions using OPML files",
				UsageLine:   "abs opml <command> [args]",
				Subcommands: []clihelp.Command{
					{
						Name:        "import",
						Description: "Import podcast subscriptions from an OPML file into Audiobookshelf",
						UsageLine:   "abs opml import <file> [options]",
						Parameters: []clihelp.Param{
							{Name: "<file>", Description: "Path to the OPML file to import"},
						},
						Options: []clihelp.Option{
							clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "opml"
							opts.OPMLSubcmd = "import"
							if len(ctx.Args) > 0 {
								opts.OPMLFile = ctx.Args[0]
							}
							return nil
						},
					},
					{
						Name:        "export",
						Description: "Export all Audiobookshelf podcast RSS feeds into an OPML file",
						UsageLine:   "abs opml export <file> [options]",
						Parameters: []clihelp.Param{
							{Name: "<file>", Description: "Path to write the exported OPML file"},
						},
						Options: []clihelp.Option{
							clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug output"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "opml"
							opts.OPMLSubcmd = "export"
							if len(ctx.Args) > 0 {
								opts.OPMLFile = ctx.Args[0]
							}
							return nil
						},
					},
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "opml"
					if len(ctx.Args) > 0 {
						switch strings.ToLower(ctx.Args[0]) {
						case "import":
							opts.OPMLSubcmd = "import"
							if len(ctx.Args) > 1 {
								opts.OPMLFile = ctx.Args[1]
							}
						case "export":
							opts.OPMLSubcmd = "export"
							if len(ctx.Args) > 1 {
								opts.OPMLFile = ctx.Args[1]
							}
						default:
							opts.OPMLFile = ctx.Args[0]
						}
					}
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
					clihelp.Bool(&opts.Debug, "-d, --debug", false, "Enable debug mode with key logging and screen snapshots (F12)"),
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
					clihelp.Int(&opts.Count, "-k, --count <number>", 0, "Explicit number of episodes to download (overrides policy)"),
					clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
					clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host (e.g. cloud8)"),
					clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing (skip remote host)"),
					clihelp.Bool(&opts.PodcastsOnly, "--podcasts-only", false, "Only scan for new podcasts and create directories (skip episode downloads)"),
					clihelp.Bool(&opts.EpisodesOnly, "--episodes-only", false, "Only check and download new episodes (skip podcast folder scanning)"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "scan"
					if opts.Count > 0 {
						opts.CountGiven = true
					}
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "new",
				Description: "Check and download new podcast episodes based on download policies",
				UsageLine:   "abs new [podcasts_dir] [options]",
				Parameters: []clihelp.Param{
					{Name: "[podcasts_dir]", Description: "Optional podcasts directory path (defaults to configured podcasts_dir)"},
				},
				Args: clihelp.MaximumNArgs(1),
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID to check/download new episodes"),
					clihelp.Int(&opts.Count, "-k, --count <number>", 0, "Explicit number of episodes to download (overrides policy)"),
					clihelp.Bool(&opts.DownloadAll, "--all", false, "Download all episodes from entire feed catalog"),
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
					clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
					clihelp.Bool(&opts.Remote, "--remote", false, "Offload post-download audio processing to remote host (e.g. cloud8)"),
					clihelp.Bool(&opts.Local, "--local", false, "Force local post-download audio processing (skip remote host)"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "server"
					opts.ServerSubcmd = "scan"
					opts.EpisodesOnly = true
					if opts.Count > 0 {
						opts.CountGiven = true
					}
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "status",
				Description: "Show status overview of local library and remote worker (or 'status podcasts' for full table)",
				UsageLine:   "abs status [podcasts|<podcasts_dir>]",
				Parameters: []clihelp.Param{
					{Name: "[podcasts]", Description: "Use 'podcasts' or directory path to show full detailed table"},
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
			buildRemoteCommand(opts, action),
			buildBatchWorkerCommand(opts, action),
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
		clihelp.String(&opts.TranscribeMin, "-t, --tminutes <minutes>", "", "Only transcribe first N minutes (e.g. 10m)"),
		clihelp.String(&opts.Force, "-f, --force <stage>", "", "Force re-running: 'whisper', 'llm', or 'all'"),
		clihelp.String(&opts.UseLLM, "--profile <id/name>", "", "Select LLM profile ID or name"),
		clihelp.String(&opts.RemoteFFmpegHost, "--rffmpeg <host>", "", "Delegate FFmpeg audio cutting to remote SSH host (e.g. cloud8)"),
		clihelp.Bool(&opts.Remote, "--remote", false, "Offload batch processing to remote host (e.g. cloud8)"),
		clihelp.Bool(&opts.Local, "--local", false, "Force local processing (skip remote host)"),
		clihelp.Bool(&opts.NoCollect, "--no-collect", false, "Skip automatic remote collection before push"),
		clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview and count episodes needing transcription, learning, or cutting without modifying files"),
		clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Maximum number of untranscribed episodes to process"),
		clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Target a specific podcast by short ID, index, or name"),
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
	opts.IsScanCommand = (action == "scan" || action == "new" || (action == "server" && (opts.ServerSubcmd == "scan" || opts.ServerSubcmd == "new")))
	opts.IsStatusCommand = (action == "status")
	opts.IsRemoteCommand = (action == "remote")
	opts.IsBatchWorkerCommand = (action == "batch-worker")

	return action, opts
}
