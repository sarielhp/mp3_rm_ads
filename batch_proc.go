package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func processAudioFilesBatch(cli CLIOptions, config Config, action string) {
	if cli.ProcSubcmd == "collect" {
		if err := runRemotePull(&config, cli.RemoteHost, nil, cli.Quiet, cli.Verbose); err != nil {
			fatalError("Error collecting from remote %s: %v\n", cli.RemoteHost, err)
		}
		return
	}

	hasError := false
	_ = hasError
	if cli.Force != "" {
		f := strings.ToLower(cli.Force)
		if f == "all" || strings.Contains(f, "whisper") || strings.Contains(f, "transcribe") {
			cli.ForceTranscribe = true
		}
		if f == "all" || strings.Contains(f, "llm") || strings.Contains(f, "ads") {
			cli.ForceLLM = true
		}
	}

	args := cli.Args
	podcastsDir := config.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if cli.Podcast != "" {
		targetDir, _, found := resolvePodcastDirByIDOrName(podcastsDir, cli.Podcast)
		if !found {
			if !cli.Quiet {
				fmt.Printf("Podcast matching '%s' not found.\n", cli.Podcast)
			}
			return
		}
		args = []string{targetDir}
	} else if len(args) == 1 {
		arg := args[0]
		if !strings.HasSuffix(strings.ToLower(arg), ".mp3") && !strings.HasSuffix(strings.ToLower(arg), ".json") {
			if targetDir, _, found := resolvePodcastDirByIDOrName(podcastsDir, arg); found {
				args = []string{targetDir}
			} else if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
				args = []string{arg}
			} else {
				if !cli.Quiet {
					fmt.Printf("Podcast matching '%s' not found.\n", arg)
				}
				return
			}
		}
	} else if len(args) == 0 {
		if config.PodcastsDir != "" {
			args = []string{config.PodcastsDir}
		} else {
			fmt.Println("ERROR: No files or directories specified, and podcasts_dir is not configured.")
			return
		}
	}
	var expandedArgs []string

	hasPrintedScanning := false
	printScanning := func(dir string) {
		if cli.Quiet {
			return
		}
		if !hasPrintedScanning {
			fmt.Println()
			hasPrintedScanning = true
		}
		fmt.Printf("Scanning: %s\n", dir)
	}

	for _, arg := range args {
		fi, err := os.Stat(arg)
		if err == nil && fi.IsDir() {
			printScanning(arg)
			removeWorkDirs(arg)
			rawMp3Files := findMP3Files(arg)
			if len(rawMp3Files) == 0 {
				if !cli.Quiet {
					fmt.Printf("No MP3 files found in directory '%s'.\n", arg)
				}
				continue
			}

			filesByFolder := make(map[string][]string)
			for _, f := range rawMp3Files {
				folder := filepath.Dir(f)
				filesByFolder[folder] = append(filesByFolder[folder], f)
			}

			var podFolders []string
			for folder := range filesByFolder {
				podFolders = append(podFolders, folder)
			}
			sort.Strings(podFolders)

			for _, podFolder := range podFolders {
				if strings.HasSuffix(podFolder, "-1") || strings.HasSuffix(podFolder, "-1/") {
					continue
				}
				fList := filesByFolder[podFolder]
				podCfg := loadPodcastConfig(podFolder)
				if podCfg.AdRemoval == AdRemovalNone {
					if cli.Verbose && !cli.Quiet {
						fmt.Printf("Podcast config set to 'none' for '%s'. Skipping.\n", filepath.Base(podFolder))
					}
					continue
				}
				filtered := filterMP3FilesByPodcastConfig(fList, podFolder, podCfg)
				expandedArgs = append(expandedArgs, filtered...)
			}
		} else {
			expandedArgs = append(expandedArgs, arg)
		}
	}

	if len(expandedArgs) > 1 {
		sort.SliceStable(expandedArgs, func(i, j int) bool {
			ti := getEpisodePublicationTime(expandedArgs[i])
			tj := getEpisodePublicationTime(expandedArgs[j])
			if ti.Equal(tj) {
				return expandedArgs[i] < expandedArgs[j]
			}
			return ti.After(tj)
		})
	}

	if len(expandedArgs) == 0 {
		if !cli.Quiet {
			fmt.Println("No files or directories with audio found to process.")
		}
		return
	}

	if cli.DryRun {
		handleProcDryRun(expandedArgs, cli, config)
		return
	}

	targetHost := ""
	if !cli.Local {
		reqHost := ""
		if cli.Remote {
			reqHost = config.RemoteHost
			if reqHost == "" {
				reqHost = "cloud8"
			}
		}
		h, isRem, err := ResolveProcessingHost(&config, reqHost, nil)
		if err == nil && isRem {
			targetHost = h
		}
	}

	if targetHost != "" {
		if !cli.NoCollect && !cli.DryRun {
			if err := runRemotePull(&config, targetHost, nil, cli.Quiet, cli.Verbose); err != nil {
				if !cli.Quiet {
					fmt.Fprintf(os.Stderr, "Warning: remote collection from %s encountered an issue: %v\n", targetHost, err)
				}
			}
		}

		var filesToPush []string
		for _, f := range expandedArgs {
			if strings.HasSuffix(f, ".json") {
				continue
			}
			mainMP3File, _, _ := resolveAudioFiles(f, cli)
			if !cli.ForceTranscribe && !cli.ForceLLM && !cli.Recut && (isEpisodeCompleted(mainMP3File) || isEpisodeInRemoteFlight(mainMP3File)) {
				continue
			}
			filesToPush = append(filesToPush, f)
		}
		sortAudioFilesByDuration(filesToPush)
		if cli.Count > 0 && len(filesToPush) > cli.Count {
			filesToPush = filesToPush[:cli.Count]
		}
		if len(filesToPush) == 0 {
			remoteWorkDir := config.RemoteWorkDir
			if remoteWorkDir == "" {
				remoteWorkDir = "~/abs_remote"
			}
			_ = ensureRemoteEnvironmentAndWorker(&config, targetHost, remoteWorkDir, nil, cli.Quiet)
			if !cli.Quiet {
				fmt.Println("All audio files are already transcribed, cleaned, or currently processing remotely.")
			}
			return
		}
		if err := runRemotePush(&config, filesToPush, targetHost, nil, cli.Priority, cli.Quiet, cli.Verbose); err != nil {
			fatalError("Error pushing batch to remote %s: %v\n", targetHost, err)
		}
		return
	}

	wakeWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet)

	selectedProfile := selectProfile(config, cli.UseLLM)
	batchStartTime := time.Now()

	totalFiles := len(expandedArgs)
	processedCount := 0

	for idx, inputFile := range expandedArgs {
		errFlag, processedFlag, stopFlag := processSingleAudioFile(idx, len(expandedArgs), processedCount, inputFile, cli, config, action, batchStartTime, selectedProfile)
		if stopFlag {
			break
		}
		if errFlag {
			hasError = true
		}
		if processedFlag {
			processedCount++
		}
	}

	if (processedCount > 1 || totalFiles > 1) && !cli.Quiet {
		batchDuration := time.Since(batchStartTime)
		fmt.Printf("\nBatch Completed! Processed %d file(s) in %s.\n", processedCount, formatClock(batchDuration.Seconds()))
	}

	os.Stdout.Sync()
	os.Stderr.Sync()
}
