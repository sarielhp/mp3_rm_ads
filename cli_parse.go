package main

import (
	"flag"
	"os"

	"github.com/sarielhp/clihelp"
)

func parseFlags() CLIOptions {
	cli := CLIOptions{
		SaveTranscript: true,
	}

	args := os.Args[1:]

	var detectedCommand string
	if len(args) > 0 {
		switch args[0] {
		case "config":
			cli.IsConfigCommand = true
			detectedCommand = "config"
			args = args[1:]
		case "dir":
			cli.IsDirCommand = true
			detectedCommand = "dir"
			args = args[1:]
		case "file":
			cli.IsFileCommand = true
			detectedCommand = "file"
			args = args[1:]
		case "tui":
			cli.IsTUICommand = true
			detectedCommand = "tui"
			args = args[1:]
		case "timeline", "table":
			cli.IsTimelineCommand = true
			detectedCommand = "timeline"
			args = args[1:]
		case "cache":
			cli.IsCacheCommand = true
			detectedCommand = "cache"
			args = args[1:]
			if len(args) > 0 && (args[0] == "reset" || args[0] == "clear" || args[0] == "clean") {
				cli.ResetCache = true
				args = args[1:]
			} else {
				cli.ResetCache = true
			}
		case "test":
			cli.IsTestCommand = true
			detectedCommand = "test"
			args = args[1:]
			if len(args) > 0 && (args[0] == "whisper" || args[0] == "whisper-server") {
				cli.TestWhisper = true
				args = args[1:]
			} else if len(args) > 0 && args[0] == "abs" {
				args = args[1:]
				if len(args) > 0 && args[0] == "map" {
					cli.TestABSMap = true
					args = args[1:]
				} else if len(args) > 0 && args[0] == "download" {
					cli.TestABSDownload = true
					args = args[1:]
				} else {
					cli.TestABS = true
				}
			} else if len(args) > 0 && args[0] == "kitty" {
				cli.TestKitty = true
				args = args[1:]
			} else {
				cli.TestWhisper = true
			}
		case "test-whisper":
			cli.IsTestCommand = true
			cli.TestWhisper = true
			detectedCommand = "test"
			args = args[1:]
		case "test-abs":
			cli.IsTestCommand = true
			cli.TestABS = true
			detectedCommand = "test"
			args = args[1:]
		case "test-abs-map":
			cli.IsTestCommand = true
			cli.TestABSMap = true
			detectedCommand = "test"
			args = args[1:]
		case "test-abs-download":
			cli.IsTestCommand = true
			cli.TestABSDownload = true
			detectedCommand = "test"
			args = args[1:]
		case "test-kitty":
			cli.IsTestCommand = true
			cli.TestKitty = true
			detectedCommand = "test"
			args = args[1:]
		case "scan":
			cli.IsScanCommand = true
			detectedCommand = "scan"
			args = args[1:]
		case "status":
			cli.IsStatusCommand = true
			detectedCommand = "status"
			args = args[1:]
		}
	}

	for _, a := range args {
		switch a {
		case "help", "usage", "-h", "--h", "-help", "--help", "?", "-?":
			if detectedCommand != "" {
				app := buildUsageApp()
				app.Execute([]string{detectedCommand})
			} else {
				app := buildUsageApp()
				app.Execute([]string{})
			}
			os.Exit(0)
		}
	}

	testWhisperCmd := cli.TestWhisper
	testABSCmd := cli.TestABS
	testABSMapCmd := cli.TestABSMap
	testABSDownloadCmd := cli.TestABSDownload
	testKittyCmd := cli.TestKitty
	isTestCmd := cli.IsTestCommand

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flag.StringVar(&cli.Output, "o", "", "Output MP3 path or directory")
	flag.StringVar(&cli.Output, "output", "", "Output MP3 path or directory")
	flag.BoolVar(&cli.Quiet, "q", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Quiet, "quiet", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Verbose, "v", false, "Verbose output (show detailed info)")
	flag.BoolVar(&cli.Verbose, "verbose", false, "Verbose output (show detailed info)")
	flag.BoolVar(&cli.Silent, "s", false, "Suppress output unless error occurs")
	flag.BoolVar(&cli.Silent, "silent", false, "Suppress output unless error occurs")
	flag.BoolVar(&cli.ExportSRT, "srt", false, "Export/convert transcript JSON to SubRip (.srt)")
	flag.BoolVar(&cli.ExportTXT, "txt", false, "Export/convert transcript JSON to text (.txt)")
	flag.BoolVar(&cli.Recut, "recut", false, "Recut audio using .cuts.json (no Whisper/LLM)")
	flag.BoolVar(&cli.SaveTranscript, "no-transcript", false, "Disable saving default .transcript.json file")
	flag.BoolVar(&cli.UseChunks, "use-chunks", false, "Split long audio into chunks for reliable transcription")
	flag.BoolVar(&cli.ExtractKeywords, "extract-keywords", false, "Extract keywords via LLM to improve Whisper accuracy")
	flag.StringVar(&cli.TranscribeMin, "t", "", "Only transcribe first N minutes (e.g. -t 10m)")
	flag.StringVar(&cli.TranscribeMin, "transcribe-minutes", "", "Only transcribe first N minutes (e.g. -t 10m)")
	flag.BoolVar(&cli.ForceLLM, "force-llm", false, "Force re-running LLM ad detection even if .cuts.json exists")
	flag.BoolVar(&cli.ForceTranscribe, "force-transcribe", false, "Force re-transcribing audio even if .transcript.json exists")
	flag.StringVar(&cli.UseLLM, "use-llm", "", "Select active LLM profile by ID or name")
	flag.StringVar(&cli.UseLLM, "profile", "", "Select active LLM profile by ID or name")
	flag.IntVar(&cli.SetDefault, "set-default", 0, "Set default LLM profile ID in config file")
	flag.StringVar(&cli.PodcastsDir, "podcasts_dir", "", "Set default podcasts/media directory in config file")
	flag.StringVar(&cli.PodcastsDir, "podcasts-dir", "", "Set default podcasts/media directory in config file")
	flag.BoolVar(&cli.ListLLMs, "list-llms", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.ListLLMs, "list-profiles", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.ListWhispers, "list-whisper", false, "List all configured Whisper server profiles and exit")
	flag.BoolVar(&cli.ListWhispers, "list-whispers", false, "List all configured Whisper server profiles and exit")
	flag.IntVar(&cli.SetDefaultWhisper, "set-default-whisper", 0, "Set default Whisper server profile ID in config file")
	flag.StringVar(&cli.AddWhisper, "add-whisper", "", "Add new Whisper server profile (Name|URL|[SpeedFactor]|[DockerContainer]|[Language]|[Prompt]|[WakeCommand])")
	flag.IntVar(&cli.RemoveWhisper, "remove-whisper", 0, "Remove Whisper server profile by ID")
	flag.BoolVar(&cli.CopyOpenCode, "copy_llm_from_opencode", false, "Import LLM settings from OpenCode config")
	flag.StringVar(&cli.ABSURL, "abs-url", "", "Audiobookshelf server URL")
	flag.StringVar(&cli.ABSUser, "abs-user", "", "Audiobookshelf username")
	flag.StringVar(&cli.ABSPass, "abs-pass", "", "Audiobookshelf password")
	flag.BoolVar(&cli.ResetCache, "reset-cache", false, "Reset and purge all cached podcast metadata and cover images")
	var testWhisperFlag bool
	flag.BoolVar(&testWhisperFlag, "test-whisper", false, "Test whisper server connection and exit")
	var testABSFlag bool
	flag.BoolVar(&testABSFlag, "test-abs", false, "Test Audiobookshelf server connection and exit")

	flag.Usage = func() {
		if detectedCommand != "" {
			app := buildUsageApp()
			app.Execute([]string{detectedCommand})
		} else {
			app := buildUsageApp()
			app.Execute([]string{})
		}
	}
	flag.CommandLine.Parse(args)

	if testWhisperCmd || testWhisperFlag || testABSCmd || testABSFlag || testABSMapCmd || testABSDownloadCmd || testKittyCmd || isTestCmd {
		if testWhisperCmd || testWhisperFlag || isTestCmd {
			cli.TestWhisper = true
		}
		if testABSCmd || testABSFlag {
			cli.TestABS = true
		}
		if testABSMapCmd {
			cli.TestABSMap = true
		}
		if testABSDownloadCmd {
			cli.TestABSDownload = true
		}
		if testKittyCmd {
			cli.TestKitty = true
		}
		cli.IsTestCommand = true
	}

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "no-transcript" {
			cli.SaveTranscript = false
		}
		if f.Name == "podcasts_dir" || f.Name == "podcasts-dir" {
			cli.SetPodcastsDir = true
		}
		if f.Name == "abs-url" || f.Name == "abs-user" || f.Name == "abs-pass" {
			cli.SetABS = true
		}
	})

	return cli
}

func buildUsageApp() *clihelp.App {
	return &clihelp.App{
		Name:        "abs",
		Description: "Automatic Podcast Ad & Sponsor Segment Remover",
		Commands: []clihelp.Command{
			{
				Name:        "file",
				Description: "Process individual MP3 or transcript JSON files",
				UsageLine:   "abs file <file1.mp3> [file2.mp3 ...] [options]",
				Options:     globalOptions(),
				Examples:    fileExamples(),
			},
			{
				Name:        "dir",
				Description: "Recursively process all MP3s in a directory",
				UsageLine:   "abs dir <directory> [options]",
				Options:     globalOptions(),
				Examples: []clihelp.Example{
					{Line: "abs dir ~/podcasts"},
					{Line: "abs dir ~/podcasts -q"},
				},
			},
			{
				Name:        "tui",
				Description: "Interactive TUI browser for podcasts and episodes",
				UsageLine:   "abs tui [directory]",
				Options: []clihelp.Option{
					{Flags: "[directory]", Description: "Podcasts directory (default: podcasts_dir from config)"},
				},
				Examples: []clihelp.Example{
					{Line: "abs tui"},
					{Line: "abs tui ~/podcasts"},
				},
			},
			{
				Name:        "config",
				Description: "Manage configuration",
				UsageLine:   "abs config [options]",
				Options: []clihelp.Option{
					{Flags: "--podcasts_dir DIR", Description: "Set default podcasts/media directory in config file"},
					{Flags: "--abs-url URL", Description: "Set Audiobookshelf server URL"},
					{Flags: "--abs-user USER", Description: "Set Audiobookshelf username"},
					{Flags: "--abs-pass PASS", Description: "Set Audiobookshelf password"},
					{Flags: "--list-llms", Description: "List all configured LLM profiles and exit"},
					{Flags: "--set-default ID", Description: "Set default LLM profile ID in config file"},
					{Flags: "--list-whispers", Description: "List all configured Whisper server profiles and exit"},
					{Flags: "--set-default-whisper ID", Description: "Set default Whisper server profile ID in config file"},
					{Flags: "--add-whisper SPEC", Description: "Add new Whisper server profile (Name|URL|[SpeedFactor]|[DockerContainer]|[Language]|[Prompt]|[WakeCommand])"},
					{Flags: "--remove-whisper ID", Description: "Remove Whisper server profile by ID"},
					{Flags: "--copy_llm_from_opencode", Description: "Import LLM settings from OpenCode config"},
				},
				Examples: []clihelp.Example{
					{Line: "abs config --podcasts_dir /path/to/podcasts"},
					{Line: "abs config --abs-url http://192.168.1.100:80"},
					{Line: "abs config --abs-user admin --abs-pass secret"},
					{Line: "abs config --list-llms"},
					{Line: "abs config --set-default 2"},
					{Line: "abs config --list-whispers"},
					{Line: "abs config --set-default-whisper 2"},
					{Line: "abs config --add-whisper \"Cloud8|http://cloud8:8000/v1/audio/transcriptions|7.0||||/home/sariel/info/misc/26/08/28/gc/cloud8 wake\""},
					{Line: "abs config --remove-whisper 3"},
					{Line: "abs config --copy_llm_from_opencode"},
				},
			},
			{
				Name:        "test",
				Description: "Test external services like Whisper server or Audiobookshelf",
				UsageLine:   "abs test <whisper|abs [connect|map|download]|kitty <image>>",
				Options: []clihelp.Option{
					{Flags: "--test-whisper", Description: "Test whisper server connection with retries"},
					{Flags: "--test-abs", Description: "Test Audiobookshelf server connection"},
					{Flags: "--test-abs-map", Description: "Map local podcast files to Audiobookshelf metadata"},
					{Flags: "--test-abs-download", Description: "Download all ABS data for all MP3 files"},
					{Flags: "--test-kitty", Description: "Test Kitty image protocol with an image file"},
				},
				Examples: []clihelp.Example{
					{Line: "abs test whisper"},
					{Line: "abs test abs connect"},
					{Line: "abs test abs map"},
					{Line: "abs test abs download"},
					{Line: "abs test kitty cover.jpg"},
					{Line: "abs test"},
				},
			},
			{
				Name:        "timeline",
				Description: "Display exact online availability timestamps table for recent podcast episodes",
				UsageLine:   "abs timeline [directory]",
				Options: []clihelp.Option{
					{Flags: "[directory]", Description: "Podcast directory or root podcasts directory (default: podcasts_dir from config)"},
				},
				Examples: []clihelp.Example{
					{Line: "abs timeline"},
					{Line: "abs timeline ~/podcasts/MyShow"},
				},
			},
			{
				Name:        "cache",
				Description: "Manage and reset local podcast metadata and cover image cache",
				UsageLine:   "abs cache [reset|clear]",
				Options: []clihelp.Option{
					{Flags: "reset", Description: "Completely purge all cached podcast metadata and cover images"},
				},
				Examples: []clihelp.Example{
					{Line: "abs cache reset"},
					{Line: "abs cache clear"},
				},
			},
			{
				Name:        "scan",
				Description: "Scan Audiobookshelf for new podcasts, create local directories, and download metadata/covers",
				UsageLine:   "abs scan [podcasts_dir]",
				Options: []clihelp.Option{
					{Flags: "[podcasts_dir]", Description: "Local podcasts directory (default: podcasts_dir from config)"},
				},
				Examples: []clihelp.Example{
					{Line: "abs scan"},
					{Line: "abs scan /media/podcasts/clean/"},
				},
			},
			{
				Name:        "status",
				Description: "Show a dry-run status report of all podcasts and episodes needing ad removal",
				UsageLine:   "abs status [podcasts_dir]",
				Options: []clihelp.Option{
					{Flags: "[podcasts_dir]", Description: "Local podcasts directory (default: podcasts_dir from config)"},
				},
				Examples: []clihelp.Example{
					{Line: "abs status"},
				},
			},
		},
	}
}

func globalOptions() []clihelp.Option {
	return []clihelp.Option{
		{Flags: "-o, --output PATH", Description: "Output MP3 path or directory"},
		{Flags: "-q, --quiet", Description: "Suppress progress and informational output"},
		{Flags: "-v, --verbose", Description: "Verbose output (show detailed info)"},
		{Flags: "-s, --silent", Description: "Suppress output unless error occurs"},
		{Flags: "--srt", Description: "Export/convert transcript JSON to SubRip (.srt)"},
		{Flags: "--txt", Description: "Export/convert transcript JSON to text (.txt)"},
		{Flags: "--recut", Description: "Recut audio using .cuts.json (no Whisper/LLM)"},
		{Flags: "--no-transcript", Description: "Disable saving default .transcript.json file"},
		{Flags: "--use-chunks", Description: "Split long audio into chunks for reliable transcription"},
		{Flags: "--extract-keywords", Description: "Extract keywords via LLM to improve Whisper accuracy"},
		{Flags: "-t, --transcribe-minutes Nm", Description: "Only transcribe first N minutes (e.g. -t 10m)"},
		{Flags: "--force-llm", Description: "Force re-running LLM ad detection even if .cuts.json exists"},
		{Flags: "--force-transcribe", Description: "Force re-transcribing audio even if .transcript.json exists"},
		{Flags: "--use-llm ID_OR_NAME", Description: "Select active LLM profile by ID or name"},
		{Flags: "--list-llms", Description: "List all configured LLM profiles and exit"},
		{Flags: "--set-default ID", Description: "Set default LLM profile ID in config file"},
		{Flags: "--podcasts_dir DIR", Description: "Set default podcasts/media directory in config file"},
		{Flags: "--copy_llm_from_opencode", Description: "Import LLM settings from OpenCode config"},
		{Flags: "--test-whisper", Description: "Test whisper server connection and exit"},
		{Flags: "-h, --help", Description: "Show this detailed usage message"},
	}
}

func fileExamples() []clihelp.Example {
	return []clihelp.Example{
		{Line: "abs file episode.mp3"},
		{Line: "abs file episode1.mp3 episode2.mp3 episode3.mp3"},
		{Line: "abs file --recut episode.mp3"},
		{Line: "abs file -q --srt episode.mp3"},
		{Line: "abs file episode.transcript.json -srt -txt"},
		{Line: "abs file --use-llm 2 episode.mp3"},
		{Line: "abs file --copy_llm_from_opencode"},
	}
}
