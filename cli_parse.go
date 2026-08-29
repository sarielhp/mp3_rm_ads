package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sarielhp/clihelp"
)

func normalizeCLIArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	if args[0] == "test-whisper" {
		return append([]string{"test", "whisper"}, args[1:]...)
	}
	if args[0] == "test-abs" {
		return append([]string{"test", "abs"}, args[1:]...)
	}
	if args[0] == "test-abs-map" {
		return append([]string{"test", "abs", "map"}, args[1:]...)
	}
	if args[0] == "test-kitty" {
		return append([]string{"test", "kitty"}, args[1:]...)
	}

	subcmds := map[string]bool{
		"file":         true,
		"dir":          true,
		"tui":          true,
		"config":       true,
		"cfg":          true,
		"test":         true,
		"timeline":     true,
		"table":        true,
		"cache":        true,
		"scan":         true,
		"rescan":       true,
		"status":       true,
		"download":     true,
		"dl":           true,
		"new":          true,
		"check-new":    true,
		"new-episodes": true,
		"keep":         true,
		"clean":        true,
		"purge":        true,
		"list":         true,
		"ls":           true,
		"help":         true,
		"usage":        true,
	}

	if subcmds[strings.ToLower(args[0])] {
		return args
	}

	if _, err := strconv.Atoi(args[0]); err == nil {
		return append([]string{"download"}, args...)
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

	if hasMp3 {
		return append([]string{"file"}, args...)
	}
	if hasDir {
		return append([]string{"dir"}, args...)
	}
	if lastArgNum {
		return append([]string{"download"}, args...)
	}

	return args
}

func buildCLIApp(action *string, opts *CLIOptions) *clihelp.App {
	keepVal := -1
	countVal := -1

	app := &clihelp.App{
		Name:        "abs",
		Description: "Automatic Ad Segment Remover & Podcast Manager",
		GlobalNote:  "Run 'abs <command> --help' or 'abs help <command>' for command-specific options.",
		Commands: []clihelp.Command{
			{
				Name:        "file",
				Description: "Process individual MP3 or transcript JSON files",
				UsageLine:   "abs file <file1.mp3> [file2.mp3 ...] [options]",
				Options:     getTranscriptionOptions(opts),
				Run: func(ctx *clihelp.Context) error {
					*action = "file"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "dir",
				Description: "Recursively process all MP3s in a directory",
				UsageLine:   "abs dir <directory> [options]",
				Options:     getTranscriptionOptions(opts),
				Run: func(ctx *clihelp.Context) error {
					*action = "dir"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "tui",
				Description: "Interactive TUI browser for podcasts and episodes",
				UsageLine:   "abs tui [directory]",
				Options: []clihelp.Option{
					clihelp.String(&opts.PodcastsDir, "--podcasts-dir, --podcasts_dir <dir>", "", "Podcasts directory"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "tui"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "timeline",
				Aliases:     []string{"table"},
				Description: "Display exact online availability timestamps table for recent podcast episodes",
				UsageLine:   "abs timeline [directory]",
				Run: func(ctx *clihelp.Context) error {
					*action = "timeline"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "status",
				Description: "Show a dry-run status report of all podcasts and episodes needing ad removal",
				UsageLine:   "abs status [podcasts_dir]",
				Run: func(ctx *clihelp.Context) error {
					*action = "status"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "cache",
				Description: "Manage and reset local podcast metadata and cover image cache",
				UsageLine:   "abs cache [reset|clear]",
				Run: func(ctx *clihelp.Context) error {
					*action = "cache"
					opts.Args = ctx.Args
					if len(ctx.Args) > 0 && (ctx.Args[0] == "reset" || ctx.Args[0] == "clear" || ctx.Args[0] == "clean") {
						opts.ResetCache = true
					} else {
						opts.ResetCache = true
					}
					return nil
				},
			},
			{
				Name:        "test",
				Description: "Test external services like Whisper server or Audiobookshelf",
				UsageLine:   "abs test <whisper|abs [connect|map|download]|kitty <image>>",
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
				Name:        "scan",
				Description: "Scan Audiobookshelf for new podcasts, create local directories, and cache covers",
				UsageLine:   "abs scan [podcasts_dir]",
				Run: func(ctx *clihelp.Context) error {
					*action = "scan"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "rescan",
				Description: "Scan MP3 file lengths on disk against DB duration and update DB if shorter",
				UsageLine:   "abs rescan [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, -P, --podcast <podcast>", "", "Specify podcast by index or title"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview actions without updating DB"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed trace outputs"),
					clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress standard output"),
					clihelp.String(&opts.SqliteDBPath, "--db-path, --sqlite-db-path <path>", "", "Path to absdatabase.sqlite"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "rescan"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "List all available podcasts in Audiobookshelf with episode counts",
				UsageLine:   "abs list [options]",
				Options: []clihelp.Option{
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed output (Feed URLs and IDs)"),
					clihelp.Bool(&opts.Silent, "-s, --silent", false, "Suppress outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "list"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "download",
				Aliases:     []string{"dl"},
				Description: "Download undownloaded episodes for podcasts",
				UsageLine:   "abs download [<number>] [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, -P, --podcast <podcast>", "", "Specify podcast by name, index, or ID"),
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
					*action = "download"
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
				Name:        "new",
				Aliases:     []string{"check-new", "new-episodes"},
				Description: "Check for new episodes published since last download and queue download",
				UsageLine:   "abs new [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, -P, --podcast <podcast>", "", "Specify podcast"),
					clihelp.Bool(&opts.NoWait, "--no-wait", false, "Do not wait for completion"),
					clihelp.Bool(&opts.Silent, "-s, --silent", false, "Silent execution"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Dry run"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "new"
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "keep",
				Aliases:     []string{"clean", "purge"},
				Description: "Delete older episodes keeping only the latest <number> episodes per podcast",
				UsageLine:   "abs keep <number> [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.Podcast, "-p, -P, --podcast <podcast>", "", "Specify podcast"),
					clihelp.Int(&keepVal, "-K, --keep <number>", -1, "Keep policy count"),
					clihelp.Bool(&opts.Silent, "-s, --silent", false, "Silent execution"),
					clihelp.Bool(&opts.DryRun, "--dry-run", false, "Dry run"),
					clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Detailed outputs"),
				},
				Run: func(ctx *clihelp.Context) error {
					*action = "keep"
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
						return fmt.Errorf("keep count is required (e.g. abs keep 5)")
					}
					return nil
				},
			},
			{
				Name:        "config",
				Description: "Manage configuration",
				UsageLine:   "abs config [options]",
				Options: []clihelp.Option{
					clihelp.String(&opts.PodcastsDir, "--podcasts-dir, --podcasts_dir <dir>", "", "Set podcasts directory"),
					clihelp.String(&opts.ABSURL, "--abs-url, --host <url>", "", "Set Audiobookshelf server URL"),
					clihelp.String(&opts.ABSUser, "--abs-user <user>", "", "Set Audiobookshelf username"),
					clihelp.String(&opts.ABSPass, "--abs-pass <pass>", "", "Set Audiobookshelf password"),
					clihelp.String(&opts.ABSToken, "--abs-token, --token <token>", "", "Set Audiobookshelf API Bearer Token"),
					clihelp.String(&opts.SqliteDBPath, "--db-path, --sqlite-db-path <path>", "", "Set Audiobookshelf SQLite DB path"),
					clihelp.Bool(&opts.ListLLMs, "--list-llms, --list-profiles", false, "List LLM profiles"),
					clihelp.Int(&opts.SetDefault, "--set-default <id>", 0, "Set default LLM profile ID"),
					clihelp.Bool(&opts.ListWhispers, "--list-whispers, --list-whisper", false, "List Whisper profiles"),
					clihelp.Int(&opts.SetDefaultWhisper, "--set-default-whisper <id>", 0, "Set default Whisper profile ID"),
					clihelp.String(&opts.AddWhisper, "--add-whisper <spec>", "", "Add Whisper profile"),
					clihelp.Int(&opts.RemoveWhisper, "--remove-whisper <id>", 0, "Remove Whisper profile"),
					clihelp.Bool(&opts.CopyOpenCode, "--copy_llm_from_opencode", false, "Import LLM settings from OpenCode"),
					clihelp.Bool(&opts.ConfigInfo, "-i, --info", false, "Show detailed config summary info"),
				},
				Subcommands: []clihelp.Command{
					{
						Name:        "processor",
						Aliases:     []string{"processors", "proc"},
						Description: "Manage post-processing programs",
						Subcommands: []clihelp.Command{
							{
								Name:        "set",
								Aliases:     []string{"add"},
								Description: "Add or replace a post-processor program",
								UsageLine:   "abs config processor set <program>",
								Args:        clihelp.ExactArgs(1),
								Run: func(ctx *clihelp.Context) error {
									*action = "config"
									opts.ProcessorCmd = "set"
									opts.ProcessorValue = ctx.Args[0]
									return nil
								},
							},
							{
								Name:        "list",
								Aliases:     []string{"ls"},
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
								Aliases:     []string{"delete", "rm", "remove"},
								Description: "Remove a post-processor program by number",
								UsageLine:   "abs config processor del <number>",
								Args:        clihelp.ExactArgs(1),
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
					opts.Args = ctx.Args
					return nil
				},
			},
			{
				Name:        "help",
				Description: "Display usage help message for abs or a specific command",
				UsageLine:   "abs help [<command>]",
				Run: func(ctx *clihelp.Context) error {
					*action = "help"
					app := buildCLIApp(action, opts)
					if len(ctx.Args) > 0 {
						app.RenderCommand(clihelp.Options{}, ctx.Args[0])
					} else {
						app.RenderGlobal(clihelp.Options{})
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
		clihelp.Bool(&opts.ExportSRT, "--srt", false, "Export transcript to SubRip (.srt)"),
		clihelp.Bool(&opts.ExportTXT, "--txt", false, "Export transcript to text (.txt)"),
		clihelp.Bool(&opts.Recut, "--recut", false, "Recut audio using existing .cuts.json"),
		clihelp.BoolToggle(&opts.SaveTranscript, "--[no-]transcript", true, "Save default .transcript.json file"),
		clihelp.Bool(&opts.UseChunks, "--use-chunks", false, "Split long audio into chunks for Whisper"),
		clihelp.Bool(&opts.ExtractKeywords, "--extract-keywords", false, "Extract keywords to improve transcription"),
		clihelp.String(&opts.TranscribeMin, "-t, --transcribe-minutes <minutes>", "", "Only transcribe first N minutes (e.g. 10m)"),
		clihelp.Bool(&opts.ForceLLM, "--force-llm", false, "Force LLM ad detection"),
		clihelp.Bool(&opts.ForceTranscribe, "--force-transcribe", false, "Force Whisper transcription"),
		clihelp.String(&opts.UseLLM, "--use-llm, --profile <id_or_name>", "", "Select LLM profile ID or name"),
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
		switch a {
		case "help", "usage", "-h", "--h", "-help", "--help", "?", "-?":
			app := buildCLIApp(&action, &opts)
			if len(args) > 0 && args[0] != "help" && args[0] != "usage" && !strings.HasPrefix(args[0], "-") {
				app.RenderCommand(clihelp.Options{}, args[0])
			} else {
				app.RenderGlobal(clihelp.Options{})
			}
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
		app.RenderGlobal(clihelp.Options{})
		os.Exit(0)
	}

	opts.IsConfigCommand = (action == "config")
	opts.IsDirCommand = (action == "dir")
	opts.IsFileCommand = (action == "file")
	opts.IsTUICommand = (action == "tui")
	opts.IsTimelineCommand = (action == "timeline")
	opts.IsTestCommand = (action == "test")
	opts.IsScanCommand = (action == "scan")
	opts.IsStatusCommand = (action == "status")

	return action, opts
}
