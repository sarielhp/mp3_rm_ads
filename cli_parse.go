package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sarielhp/clihelp"
	"github.com/sarielhp/clihelp/tree"
)

//go:embed VERSION
var embeddedVersion string

func getVersion() string {
	if strings.TrimSpace(embeddedVersion) != "" {
		return strings.TrimSpace(embeddedVersion)
	}
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		if data, err := os.ReadFile(filepath.Join(execDir, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "0.2.11"
}

func buildCLIApp(action *string, opts *CLIOptions) *clihelp.App {
	keepVal := -1
	countVal := -1

	return &clihelp.App{
		Name:                "abs",
		Description:         "Automatic Ad Segment Remover & Podcast Manager",
		UsageLine:           "abs [OPTIONS] <COMMAND>",
		Version:             getVersion(),
		GlobalNote:          "Run 'abs <command> --help' or 'abs help <command>' for command-specific options.",
		AbbrevCommands:      true,
		Pager:               true,
		InteractiveFallback: true,
		Commands: []clihelp.Command{
			buildConfigCommand(opts, action),
			buildInfoCommand(opts, action),
			buildOffloadCommand(opts, action),
			buildPlayerCommand(opts, action),
			buildQueueCommand(opts, action),
			buildRmAdsCommand(opts, action),
			buildSyncCommand(opts, action, &countVal, &keepVal),
			buildTUICommand(opts, action),
		},
	}
}

func buildTUICommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "tui",
		Description: "Interactive TUI browser for podcasts and episodes",
		UsageLine:   "abs tui [options] [directory]",
		Args:        clihelp.MaximumNArgs(1),
		Options: []clihelp.Option{
			clihelp.String(&opts.PodcastsDir, "--podcasts-dir <dir>", "", "Podcasts directory"),
			clihelp.Bool(&opts.Debug, "-d, --debug", false, "Enable debug mode with key logging and screen snapshots (F12)"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "tui"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func resolveTestCommandArgs(args []string, opts *CLIOptions) error {
	if len(args) == 0 {
		opts.TestWhisper = true
		return nil
	}
	switch args[0] {
	case "whisper", "whisper-server":
		opts.TestWhisper = true
	case "abs":
		if len(args) > 1 {
			switch args[1] {
			case "map":
				opts.TestABSMap = true
			case "download":
				opts.TestABSDownload = true
			case "connect":
				opts.TestABS = true
			default:
				return fmt.Errorf("unknown abs test target %q (expected: connect, map, download)", args[1])
			}
		} else {
			opts.TestABS = true
		}
	case "kitty":
		opts.TestKitty = true
	default:
		return fmt.Errorf("unknown test target %q (valid targets: whisper, abs, kitty)", args[0])
	}
	return nil
}

func hideOption(o clihelp.Option) clihelp.Option {
	o.Hidden = true
	return o
}

func getTranscriptionOptions(opts *CLIOptions) []clihelp.Option {
	return []clihelp.Option{
		clihelp.String(&opts.Output, "-o, --output <path>", "", "Output MP3 path or directory"),
		clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress outputs"),
		clihelp.Bool(&opts.Verbose, "-v, --verbose", false, "Show detailed debug information"),
		hideOption(clihelp.BoolToggle(&opts.SaveTranscript, "--[no-]transcript", true, "Save default .transcript.json file")),
		hideOption(clihelp.Bool(&opts.UseChunks, "--use-chunks", false, "Split audio into chunks")),
		hideOption(clihelp.Bool(&opts.ExtractKeywords, "--extract-keywords", false, "Extract keywords for transcription")),
		hideOption(clihelp.String(&opts.TranscribeMin, "-t, --tminutes <minutes>", "", "Transcribe first N minutes")),
		hideOption(clihelp.Bool(&opts.Recut, "--recut", false, "Recut audio using existing cuts metadata")),
		clihelp.String(&opts.Force, "-f, --force <stage>", "", "Force: 'whisper', 'llm', or 'all'"),
		clihelp.String(&opts.UseLLM, "--profile <id/name>", "", "Select LLM profile ID or name"),
		hideOption(clihelp.String(&opts.RemoteFFmpegHost, "--rffmpeg <host>", "", "Remote FFmpeg host")),
		hideOption(clihelp.Bool(&opts.Remote, "--remote", false, "Offload to remote host")),
		hideOption(clihelp.Bool(&opts.Local, "--local", false, "Force local processing")),
		hideOption(clihelp.Bool(&opts.NoCollect, "--no-collect", false, "Skip remote collection before push")),
		clihelp.Bool(&opts.DryRun, "--dry-run", false, "Preview actions without file changes"),
		hideOption(clihelp.Int(&opts.Count, "-n, --limit <number>", 0, "Max untranscribed episodes")),
		hideOption(clihelp.Int(&opts.Priority, "-P, --priority <level>", 0, "Priority level for processing")),
		clihelp.String(&opts.Podcast, "-p, --podcast <name>", "", "Target podcast by ID, index, or name"),
		hideOption(clihelp.String(&opts.WhisperEngine, "--whisper-engine <engine>", "", "Engine: local, docker, remote, gemini")),
		hideOption(clihelp.String(&opts.WhisperModel, "--whisper-model <model>", "", "Model name or alias (e.g. tiny.en, base)")),
	}
}

func parseFlags() (string, CLIOptions) {
	var action string
	opts := CLIOptions{
		SaveTranscript: true,
	}

	args := os.Args[1:]

	for i, a := range args {
		if a == "--tree" || (a == "help" && i+1 < len(args) && args[i+1] == "tree") {
			app := buildCLIApp(&action, &opts)
			tree.Render(os.Stdout, app, tree.Options{})
			os.Exit(0)
		}
	}

	app := buildCLIApp(&action, &opts)
	err := app.Execute(args)
	if err != nil {
		fatalError("Error: %v\n", err)
	}

	if action == "" {
		os.Exit(0)
	}

	opts.IsConfigCommand = (action == "config")
	opts.IsDirCommand = (action == "dir")
	opts.IsFileCommand = (action == "rm_ads")
	opts.IsTUICommand = (action == "tui")
	opts.IsTimelineCommand = (action == "sync" && opts.SyncSubcmd == "timeline")
	opts.IsTestCommand = (action == "info" && opts.InfoSubcmd == "check")
	opts.IsScanCommand = (action == "sync" && (opts.SyncSubcmd == "scan" || opts.SyncSubcmd == "new" || opts.SyncSubcmd == "feeds"))
	opts.IsStatusCommand = (action == "info" && opts.InfoSubcmd == "status")
	opts.IsRemoteCommand = (action == "offload")
	opts.IsBatchWorkerCommand = (action == "offload" && opts.RemoteSubcmd == "worker" && opts.BatchWorkerDir != "")
	opts.IsSyncCommand = (action == "sync")

	return action, opts
}
