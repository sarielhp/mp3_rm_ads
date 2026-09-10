package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func handleParityCommands(action string, config Config, cli CLIOptions) (bool, error) {
	var err error
	switch action {
	case "info":
		if cli.InfoSubcmd == "status" || cli.InfoSubcmd == "check" {
			err = handleMainStatus(&config, cli)
			return true, err
		}
		err = runInfoCommand(config, cli)
	case "queue":
		err = runQueueCommand(config, cli)
	case "player":
		err = runPlayerCommand(config, cli)
	case "policy":
		err = runPolicyCommand(config, cli)
	default:
		return false, nil
	}
	return true, err
}

func handleMainTest(config Config, cli CLIOptions) error {
	if cli.TestABSMap {
		if !absMapPodcasts(config, cli.Quiet) {
			return fmt.Errorf("abs map failed")
		}
	} else if cli.TestABSDownload {
		if !absDownloadAllData(config, cli.Quiet) {
			return fmt.Errorf("abs download failed")
		}
	} else if cli.TestKitty {
		testKittyImage(cli.Args)
	} else if cli.TestABS {
		if !testAudiobookshelfServer(config, cli.Quiet) {
			return fmt.Errorf("audiobookshelf test failed")
		}
	} else {
		if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
			return fmt.Errorf("whisper test failed")
		}
	}
	return nil
}

func handleMainExport(cli CLIOptions) {
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
}

func handleMainConfig(config *Config, cli CLIOptions) error {
	switch cli.ConfigCmd {
	case "get":
		if err := handleConfigGet(*config, cli.ConfigKey); err != nil {
			return err
		}
	case "set":
		if err := handleConfigSet(config, cli.ConfigKey, cli.ConfigVal); err != nil {
			return err
		}
	case "show":
		printConfig(*config)
	case "llm-list":
		listProfiles(*config)
	case "llm-default":
		if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
			setDefaultProfile(config, id)
		} else {
			return fmt.Errorf("invalid profile ID %q", cli.ConfigVal)
		}
	case "llm-import":
		copyLLMFromOpenCode(config)
	case "whisper-list", "whisper-default", "whisper-add", "whisper-del":
		return handleMainWhisperConfig(config, cli)
	case "cache-show":
		dir, entries, size := cacheStats()
		fmt.Printf("Cache directory: %q\n", dir)
		fmt.Printf("  entries: %d\n", entries)
		fmt.Printf("  size:    %.1f MB\n", float64(size)/(1024*1024))
		fmt.Println("Run 'abs config cache clear' to delete it.")
	case "cache-reset":
		if err := resetCache(); err != nil {
			return fmt.Errorf("error resetting cache: %w", err)
		}
		if !cli.Quiet {
			fmt.Println("Cache reset successfully.")
		}
	case "migrate":
		handleConfigMigrate(config, cli.ConfigVal)
	default:
		if cli.ProcessorCmd != "" {
			handleConfigProcessor(config, cli.ProcessorCmd, cli.ProcessorValue)
		} else if cli.PodcastsDir != "" {
			setPodcastsDir(config, cli.PodcastsDir)
		} else {
			printConfig(*config)
		}
	}
	return nil
}

func handleMainWhisperConfig(config *Config, cli CLIOptions) error {
	switch cli.ConfigCmd {
	case "whisper-list":
		listWhispers(*config)
	case "whisper-default":
		if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
			setDefaultWhisperProfile(config, id)
		} else {
			return fmt.Errorf("invalid Whisper profile ID %q", cli.ConfigVal)
		}
	case "whisper-add":
		addWhisperProfile(config, cli.ConfigVal)
	case "whisper-del":
		if id, err := strconv.Atoi(cli.ConfigVal); err == nil && id > 0 {
			removeWhisperProfile(config, id)
		} else {
			return fmt.Errorf("invalid Whisper profile ID %q", cli.ConfigVal)
		}
	}
	return nil
}

func handleMainStatus(config *Config, cli CLIOptions) error {
	if cli.StatusSubcmd == "check" {
		return handleMainTest(*config, cli)
	}
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
	absStatus(*config, showDetailedPodcasts, cli.Quiet)
	return nil
}

func handleMainProc(config Config, cli CLIOptions, action string) error {
	if cli.ProcSubcmd == "audit" {
		runAuditTranscripts(config, cli)
		return nil
	}
	if cli.ProcSubcmd == "recut" {
		cli.Recut = true
	}
	if cli.ProcSubcmd == "export" {
		handleMainExport(cli)
		return nil
	}
	if cli.ProcSubcmd == "collect" {
		if err := runRemotePull(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose); err != nil {
			return fmt.Errorf("error collecting from remote %s: %w", cli.RemoteHost, err)
		}
		return nil
	}
	if cli.ProcSubcmd == "clear" {
		if err := runRemoteClear(&config, cli.RemoteHost, nil, cli.Quiet); err != nil {
			return fmt.Errorf("error clearing remote queue on %s: %w", cli.RemoteHost, err)
		}
		return nil
	}
	processAudioFilesBatch(cli, config, action)
	return nil
}
