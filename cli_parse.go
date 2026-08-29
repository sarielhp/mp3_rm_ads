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
			for _, alias := range currentCmds[i].Aliases {
				if strings.ToLower(alias) == arg {
					found = &currentCmds[i]
					break
				}
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
			for _, alias := range currentCmds[i].Aliases {
				if strings.HasPrefix(strings.ToLower(alias), arg) {
					hasPrefixMatch = true
					break
				}
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
					clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress outputs unless an error occurs"),
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
			{
				Name:        "server",
				Description: "Manage and interact with the Audiobookshelf server",
				Subcommands: []clihelp.Command{
					{
						Name:        "scan",
						Description: "Scan Audiobookshelf for new podcasts (create directories, cache covers) and check for new episodes",
						UsageLine:   "abs server scan [podcasts_dir] [options]",
						Parameters: []clihelp.Param{
							{Name: "[podcasts_dir]", Description: "Optional podcasts directory path (defaults to configured podcasts_dir)"},
						},
						Args:             clihelp.MaximumNArgs(1),
						OptionsValidator: clihelp.MutuallyExclusive("--podcasts-only", "--episodes-only"),
						Options: []clihelp.Option{
							clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify a podcast by name, index, or ID to check/download new episodes"),
							clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
							clihelp.Bool(&opts.Silent, "-s, --silent", false, "Silent execution"),
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
						Name:        "list",
						Description: "List all available podcasts in Audiobookshelf with episode counts",
						UsageLine:   "abs server list [options]",
						Args:        clihelp.NoArgs,
						Options: []clihelp.Option{
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output (Feed URLs and IDs)"),
							clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress outputs"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "list"
							opts.Args = ctx.Args
							return nil
						},
					},
					{
						Name:        "download",
						Description: "Download undownloaded episodes for podcasts",
						UsageLine:   "abs server download [<number>] [options]",
						Parameters: []clihelp.Param{
							{Name: "[<number>]", Description: "Optional number of undownloaded episodes to download (defaults to 1)"},
						},
						Args: clihelp.MaximumNArgs(1),
						Options: []clihelp.Option{
							clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
							clihelp.Int(&countVal, "-k, --count <number>", -1, "Number of undownloaded episodes to download"),
							clihelp.Bool(&opts.Fill, "-f, --fill", false, "Fill gaps in downloaded episodes"),
							clihelp.Int(&keepVal, "-K, --keep <number>", -1, "Enforce keep count policies"),
							clihelp.BoolToggle(&opts.CheckNew, "--[no-]check-new", true, "Check new episodes published"),
							clihelp.Bool(&opts.Oldest, "--oldest", false, "Download oldest first"),
							clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for download completion"),
							clihelp.Bool(&opts.Silent, "-s, --silent", false, "Silent execution"),
							clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show output without executing"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed info"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "download"
							opts.Args = ctx.Args
							if len(ctx.Args) > 0 {
								k, err := strconv.Atoi(ctx.Args[0])
								if err == nil {
									opts.Count = k
									opts.CountGiven = true
								}
							} else if countVal != -1 {
								opts.Count = countVal
								opts.CountGiven = true
							} else {
								opts.Count = 1
							}
							if keepVal > 0 {
								opts.KeepCount = &keepVal
							}
							return nil
						},
					},
					{
						Name:        "keep",
						Description: "Delete older episodes keeping only the latest <number> episodes per podcast",
						UsageLine:   "abs server keep <number> [options]",
						Parameters: []clihelp.Param{
							{Name: "<number>", Description: "Number of latest episodes to keep per podcast"},
						},
						Args: clihelp.ExactArgs(1),
						Options: []clihelp.Option{
							clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast"),
							clihelp.Int(&keepVal, "-k, --keep <number>", -1, "Keep policy count"),
							clihelp.Bool(&opts.Silent, "-s, --silent", false, "Silent execution"),
							clihelp.Bool(&opts.DryRun, "--dry-run", false, "Dry run"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "keep"
							opts.Args = ctx.Args
							if len(ctx.Args) > 0 {
								k, err := strconv.Atoi(ctx.Args[0])
								if err == nil {
									opts.KeepCount = &k
								}
							} else if keepVal > 0 {
								opts.KeepCount = &keepVal
							}
							if opts.KeepCount == nil {
								return fmt.Errorf("keep count is required (e.g. abs server keep 5)")
							}
							return nil
						},
					},
					{
						Name:        "rescan",
						Description: "Scan MP3 file lengths on disk against DB duration and update DB if shorter",
						UsageLine:   "abs server rescan [options]",
						Args:        clihelp.NoArgs,
						Options: []clihelp.Option{
							clihelp.String(&opts.Podcast, "-p, --podcast <podcast>", "", "Specify podcast by index or title"),
							clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview actions without updating DB"),
							clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed trace outputs"),
							clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress standard output"),
							clihelp.String(&opts.SqliteDBPath, "--db-path <path>", "", "Path to absdatabase.sqlite"),
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "rescan"
							opts.Args = ctx.Args
							return nil
						},
					},
					{
						Name:        "timeline",
						Description: "Display exact online availability timestamps table for recent podcast episodes",
						UsageLine:   "abs server timeline [directory]",
						Parameters: []clihelp.Param{
							{Name: "[directory]", Description: "Optional path to podcasts directory"},
						},
						Args: clihelp.MaximumNArgs(1),
						Run: func(ctx *clihelp.Context) error {
							*action = "server"
							opts.ServerSubcmd = "timeline"
							opts.Args = ctx.Args
							return nil
						},
					},
				},
			},
			{
				Name:        "config",
				Description: "View and manage application configuration",
				UsageLine:   "abs config [<subcommand>] [options]",
				Subcommands: []clihelp.Command{
					{
						Name:        "get",
						Description: "Get the value of a configuration key",
						UsageLine:   "abs config get <key>",
						Parameters: []clihelp.Param{
							{Name: "<key>", Description: "Configuration key name (e.g., 'podcasts-dir', 'remote-ffmpeg', 'abs-url')"},
						},
						Args: clihelp.ExactArgs(1),
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "get"
							opts.ConfigKey = ctx.Args[0]
							return nil
						},
					},
					{
						Name:        "set",
						Description: "Set the value of a configuration key",
						UsageLine:   "abs config set <key> <value>",
						Parameters: []clihelp.Param{
							{Name: "<key>", Description: "Configuration key name"},
							{Name: "<value>", Description: "New configuration value"},
						},
						Args: clihelp.ExactArgs(2),
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "set"
							opts.ConfigKey = ctx.Args[0]
							opts.ConfigVal = ctx.Args[1]
							return nil
						},
					},
					{
						Name:        "show",
						Description: "Display current configuration summary table",
						UsageLine:   "abs config show",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "show"
							return nil
						},
					},
					{
						Name:        "llm",
						Description: "Manage LLM profiles for ad detection",
						Subcommands: []clihelp.Command{
							{
								Name:        "list",
								Description: "List all configured LLM profiles",
								UsageLine:   "abs config llm list",
								Args:        clihelp.NoArgs,
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "llm-list"
									return nil
								},
							},
							{
								Name:        "default",
								Description: "Set default LLM profile ID",
								UsageLine:   "abs config llm default <id>",
								Parameters: []clihelp.Param{
									{Name: "<id>", Description: "Profile ID to set as default"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "llm-default"
									opts.ConfigVal = ctx.Args[0]
									return nil
								},
							},
							{
								Name:        "import",
								Description: "Import LLM settings from OpenCode",
								UsageLine:   "abs config llm import",
								Args:        clihelp.NoArgs,
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "llm-import"
									return nil
								},
							},
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "llm-list"
							return nil
						},
					},
					{
						Name:        "whisper",
						Description: "Manage Whisper transcription profiles",
						Subcommands: []clihelp.Command{
							{
								Name:        "list",
								Description: "List all configured Whisper profiles",
								UsageLine:   "abs config whisper list",
								Args:        clihelp.NoArgs,
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "whisper-list"
									return nil
								},
							},
							{
								Name:        "default",
								Description: "Set default Whisper profile ID",
								UsageLine:   "abs config whisper default <id>",
								Parameters: []clihelp.Param{
									{Name: "<id>", Description: "Whisper profile ID to set as default"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "whisper-default"
									opts.ConfigVal = ctx.Args[0]
									return nil
								},
							},
							{
								Name:        "add",
								Description: "Add a new Whisper profile (name:url:speed[:container[:lang[:prompt]]])",
								UsageLine:   "abs config whisper add <spec>",
								Parameters: []clihelp.Param{
									{Name: "<spec>", Description: "Profile specification formatted string"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "whisper-add"
									opts.ConfigVal = ctx.Args[0]
									return nil
								},
							},
							{
								Name:        "del",
								Description: "Remove a Whisper profile by ID",
								UsageLine:   "abs config whisper del <id>",
								Parameters: []clihelp.Param{
									{Name: "<id>", Description: "Whisper profile ID to delete"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ConfigCmd = "whisper-del"
									opts.ConfigVal = ctx.Args[0]
									return nil
								},
							},
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "whisper-list"
							return nil
						},
					},
					{
						Name:        "cache",
						Description: "Manage and reset local podcast metadata and cover image cache",
						UsageLine:   "abs config cache [reset|clear]",
						Parameters: []clihelp.Param{
							{Name: "[reset|clear]", Description: "Action to perform (reset or clear cache)"},
						},
						Args: clihelp.MaximumNArgs(1),
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ConfigCmd = "cache-reset"
							return nil
						},
					},
					{
						Name:        "processor",
						Description: "Manage post-processing programs",
						Subcommands: []clihelp.Command{
							{
								Name:        "set",
								Description: "Add or replace a post-processor program",
								UsageLine:   "abs config processor set <program>",
								Parameters: []clihelp.Param{
									{Name: "<program>", Description: "The command line or path of the program to run"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ProcessorCmd = "set"
									opts.ProcessorValue = ctx.Args[0]
									return nil
								},
							},
							{
								Name:        "list",
								Description: "List configured post-processor programs",
								UsageLine:   "abs config processor list",
								Args:        clihelp.NoArgs,
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ProcessorCmd = "list"
									return nil
								},
							},
							{
								Name:        "del",
								Description: "Remove a post-processor program by number",
								UsageLine:   "abs config processor del <number>",
								Parameters: []clihelp.Param{
									{Name: "<number>", Description: "The index or number of the post-processor to delete"},
								},
								Args: clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ProcessorCmd = "del"
									opts.ProcessorValue = ctx.Args[0]
									return nil
								},
							},
						},
						Run: func(ctx *clihelp.Context) error {
							*action = "config"
							opts.ProcessorCmd = "list"
							return nil
						},
					},
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "config"
					opts.ConfigCmd = "show"
					return nil
				},
			},
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
		clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress outputs unless an error occurs"),
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
