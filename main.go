package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	defer func() {
		globalPlayer.Stop()
		if r := recover(); r != nil {
			globalPlayer.Stop()
			panic(r)
		}
	}()

	action, cli := parseFlags()

	defer func() {
		if !cli.Quiet {
			fmt.Println()
		}
	}()

	ensureConfigExists()
	config := loadConfig()

	switch action {
	case "test":
		if cli.TestABSMap {
			absMapPodcasts(config, cli.Quiet)
		} else if cli.TestABSDownload {
			absDownloadAllData(config, cli.Quiet)
		} else if cli.TestKitty {
			testKittyImage(cli.Args)
		} else if cli.TestABS {
			if !testAudiobookshelfServer(config, cli.Quiet) {
				os.Exit(1)
			}
		} else {
			if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
				os.Exit(1)
			}
		}
		return

	case "export":
		targetArgs := cli.Args
		if len(targetArgs) == 0 {
			fmt.Println("No input files or directories specified for export.")
			return
		}
		for _, arg := range targetArgs {
			fi, err := os.Stat(arg)
			if err == nil && fi.IsDir() {
				files, _ := filepath.Glob(filepath.Join(arg, "*.transcript.json"))
				for _, f := range files {
					if cli.ExportTXT || cli.ExportFormat == "txt" {
						convertJSONToTXT(f, nil, 0, cli.Output, cli.Quiet)
					} else {
						convertJSONToSRT(f, nil, cli.Output, cli.Quiet)
					}
				}
			} else {
				jsonPath := arg
				if !strings.HasSuffix(jsonPath, ".json") {
					jsonPath = stripExt(arg) + ".transcript.json"
				}
				if cli.ExportTXT || cli.ExportFormat == "txt" {
					convertJSONToTXT(jsonPath, nil, 0, cli.Output, cli.Quiet)
				} else {
					convertJSONToSRT(jsonPath, nil, cli.Output, cli.Quiet)
				}
			}
		}
		return

	case "config":
		switch cli.ConfigCmd {
		case "get":
			handleConfigGet(config, cli.ConfigKey)
		case "set":
			if err := handleConfigSet(&config, cli.ConfigKey, cli.ConfigVal); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "show":
			printConfig(config)
		case "llm-list":
			listProfiles(config)
		case "llm-default":
			if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
				setDefaultProfile(&config, id)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid profile ID '%s'\n", cli.ConfigVal)
				os.Exit(1)
			}
		case "llm-import":
			copyLLMFromOpenCode(&config)
		case "whisper-list":
			listWhispers(config)
		case "whisper-default":
			if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
				setDefaultWhisperProfile(&config, id)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid Whisper profile ID '%s'\n", cli.ConfigVal)
				os.Exit(1)
			}
		case "whisper-add":
			addWhisperProfile(&config, cli.ConfigVal)
		case "whisper-del":
			if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
				removeWhisperProfile(&config, id)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid Whisper profile ID '%s'\n", cli.ConfigVal)
				os.Exit(1)
			}
		case "cache-reset":
			if err := resetCache(); err != nil {
				fmt.Fprintf(os.Stderr, "Error resetting cache: %v\n", err)
				os.Exit(1)
			}
			if !cli.Quiet {
				fmt.Println("Cache reset successfully.")
			}
		case "migrate":
			handleConfigMigrate(&config, cli.ConfigVal)
		default:
			if cli.ProcessorCmd != "" {
				handleConfigProcessor(&config, cli.ProcessorCmd, cli.ProcessorValue)
			} else if cli.PodcastsDir != "" {
				setPodcastsDir(&config, cli.PodcastsDir)
			} else {
				printConfig(config)
			}
		}
		return

	case "status":
		showDetailedPodcasts := false
		targetDir := config.PodcastsDir
		if len(cli.Args) > 0 {
			arg := strings.ToLower(cli.Args[0])
			if arg == "podcasts" || arg == "podcast" || arg == "all" {
				showDetailedPodcasts = true
			} else if fi, err := os.Stat(cli.Args[0]); err == nil && fi.IsDir() {
				targetDir = cli.Args[0]
				showDetailedPodcasts = true
			}
		}
		if cli.Verbose {
			showDetailedPodcasts = true
		}
		if targetDir != "" {
			config.PodcastsDir = targetDir
		}
		absStatus(config, showDetailedPodcasts, cli.Quiet)
		return

	case "tui":
		initDebugLogger(cli.Debug)
		dir := config.PodcastsDir
		if len(cli.Args) > 0 {
			dir = cli.Args[0]
		}
		if dir == "" {
			fmt.Println("ERROR: podcasts_dir is not configured. Specify it as an argument or in config.")
			os.Exit(1)
		}
		bk := &TuiBackend{
			LoadPodcasts: func(dir string) ([]tuiPodcast, error) {
				return loadTUIPodcastsABS(dir, config)
			},
			LoadQueues:  loadAllQueues,
			SaveQueue:   saveQueue,
			GetDuration: getAudioDuration,
		}
		go triggerBackgroundCollect(&config)
		p := tea.NewProgram(newTuiModel(bk, dir, &config), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
		return

	case "server":
		handleServerCommand(config, cli)
		return

	case "remote":
		handleRemoteCommand(config, cli)
		return

	case "ls":
		if err := runLsCommand(config, cli); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return

	case "batch-worker":
		handleBatchWorkerCommand(config, cli)
		return

	case "proc", "recut":
		if action == "recut" {
			cli.Recut = true
		}
		if cli.ProcSubcmd == "collect" {
			if err := runRemotePull(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose); err != nil {
				fmt.Fprintf(os.Stderr, "Error collecting from remote %s: %v\n", cli.RemoteHost, err)
				os.Exit(1)
			}
			return
		}
		if cli.ProcSubcmd == "clear" {
			if err := runRemoteClear(&config, cli.RemoteHost, nil, cli.Quiet); err != nil {
				fmt.Fprintf(os.Stderr, "Error clearing remote queue on %s: %v\n", cli.RemoteHost, err)
				os.Exit(1)
			}
			return
		}
		processAudioFilesBatch(cli, config, action)
		return
	}
}

func removeWorkDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(dir, entry.Name())
			if entry.Name() == ".work" {
				os.RemoveAll(path)
			} else {
				removeWorkDirs(path)
			}
		}
	}
}
