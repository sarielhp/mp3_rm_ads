package adremoval

import (
	"abs/pkg/config"
	"abs/pkg/format"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/remote"
	"abs/pkg/transcribe"
	"abs/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProcessFiles removes ads from an already-resolved set of targets: audio
// files, or directories to be expanded into the episodes their podcast's
// ad-removal policy admits. Interpreting a command line into that set belongs
// to the caller.
func ProcessFiles(targets []string, opts ProcOptions, config Config, action string) {
	opts.Normalize()

	expandedArgs := expandDirectoryArgs(targets, opts, config)
	if len(expandedArgs) == 0 {
		if !opts.Quiet {
			fmt.Println("No files or directories with audio found to process.")
		}
		return
	}

	if opts.DryRun {
		handleProcDryRun(expandedArgs, opts, config)
		return
	}

	targetHost, err := resolveRemoteProcessingTargetHost(opts, config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if targetHost != "" {
		handleRemoteBatchExecution(expandedArgs, opts, config, targetHost)
		return
	}

	executeLocalBatchProcessing(expandedArgs, opts, config, action)
}

func expandDirectoryArgs(args []string, opts ProcOptions, appCfg Config) []string {
	var expandedArgs []string
	hasPrintedScanning := false
	printScanning := func(dir string) {
		if opts.Quiet {
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
			if !opts.DryRun {
				removeWorkDirs(arg)
			}
			rawMp3Files := util.FindMP3Files(arg)
			if len(rawMp3Files) == 0 {
				if !opts.Quiet {
					fmt.Printf("No MP3 files found in directory '%s'.\n", arg)
				}
				continue
			}

			filesByFolder := make(map[string][]string)
			for _, f := range rawMp3Files {
				// A re-download lands in a sibling folder suffixed "-1".
				epFolder := filepath.Dir(f)
				if strings.HasSuffix(epFolder, "-1") || strings.HasSuffix(epFolder, "-1/") {
					continue
				}
				// Group by podcast, not by episode. With the podfetch layout
				// <podcast>/<episode>/podcast.mp3 the episode folder holds no
				// podcast.json, so grouping by it reads an empty config whose
				// ad-removal mode defaults to "none" and silently drops every
				// episode. The "latest" policy likewise needs a podcast's
				// episodes in one group to pick the newest among them.
				folder := podcast.DetectPodcastDirForAudio(f)
				filesByFolder[folder] = append(filesByFolder[folder], f)
			}

			var podFolders []string
			for folder := range filesByFolder {
				podFolders = append(podFolders, folder)
			}
			sort.Strings(podFolders)

			for _, podFolder := range podFolders {
				fList := filesByFolder[podFolder]
				podCfg := loadPodcastConfig(podFolder, appCfg)
				if !opts.DryRun {
					ensurePodcastConfig(podFolder, podCfg, opts.Quiet)
				}
				// Compare the normalized mode: the raw field is empty when a
				// podcast has no config, which is not literally "none".
				if config.NormalizeAdRemovalMode(podCfg.AdRemoval) == AdRemovalNone {
					if opts.Verbose && !opts.Quiet {
						fmt.Printf("Podcast config set to 'none' for '%s'. Skipping.\n", filepath.Base(podFolder))
					}
					continue
				}
				filtered := podcast.FilterByAdRemovalPolicy(fList, podFolder, podCfg)
				expandedArgs = append(expandedArgs, filtered...)
			}
		} else {
			expandedArgs = append(expandedArgs, arg)
		}
	}

	if len(expandedArgs) > 1 {
		sort.SliceStable(expandedArgs, func(i, j int) bool {
			ti := podcast.GetEpisodePublicationTime(expandedArgs[i])
			tj := podcast.GetEpisodePublicationTime(expandedArgs[j])
			if ti.Equal(tj) {
				return expandedArgs[i] < expandedArgs[j]
			}
			return ti.After(tj)
		})
	}
	return expandedArgs
}

func resolveRemoteProcessingTargetHost(opts ProcOptions, config Config) (string, error) {
	if opts.Local {
		return "", nil
	}
	reqHost := opts.RemoteHost
	if opts.Remote && reqHost == "" {
		reqHost = config.RemoteHost
		if reqHost == "" {
			reqHost = config.RemoteFFmpegHost
		}
		if reqHost == "" {
			return "", fmt.Errorf("remote processing requested without a remote host")
		}
	}
	h, isRem, err := remote.ResolveProcessingHost(&config, reqHost, nil)
	if err != nil {
		return "", err
	}
	if err == nil && isRem {
		return h, nil
	}
	return "", nil
}

func handleRemoteBatchExecution(expandedArgs []string, opts ProcOptions, config Config, targetHost string) {
	if !opts.NoCollect && !opts.DryRun {
		if err := remote.RunRemotePull(&config, targetHost, nil, opts.Quiet, opts.Verbose); err != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Warning: remote collection from %s encountered an issue: %v\n", targetHost, err)
			}
		}
	}

	var filesToPush []string
	for _, f := range expandedArgs {
		if strings.HasSuffix(f, ".json") {
			continue
		}
		mainMP3File, _, _ := resolveAudioFiles(f, opts)
		if !opts.ForceTranscribe && !opts.ForceLLM && !opts.Recut && (pipeline.IsEpisodeClean(mainMP3File) || pipeline.IsEpisodeInRemoteFlight(mainMP3File)) {
			continue
		}
		filesToPush = append(filesToPush, f)
	}
	remote.SortAudioFilesByDuration(filesToPush)
	if opts.Count > 0 && len(filesToPush) > opts.Count {
		filesToPush = filesToPush[:opts.Count]
	}
	if len(filesToPush) == 0 {
		remoteWorkDir := config.RemoteWorkDir
		if remoteWorkDir == "" {
			remoteWorkDir = "~/abs_remote"
		}
		_ = remote.EnsureRemoteEnvironmentAndWorker(&config, targetHost, remoteWorkDir, nil, opts.Quiet)
		if !opts.Quiet {
			fmt.Println("All audio files are already transcribed, cleaned, or currently processing remotely.")
		}
		return
	}
	if err := remote.RunRemotePush(&config, filesToPush, targetHost, nil, opts.Priority, opts.Quiet, opts.Verbose); err != nil {
		fatalError("Error pushing batch to remote %s: %v\n", targetHost, err)
	}
}

func executeLocalBatchProcessing(expandedArgs []string, opts ProcOptions, config Config, action string) error {
	wp := getActiveWhisperProfile(config)
	if opts.WhisperEngine != "" {
		wp.Engine = WhisperEngine(opts.WhisperEngine)
	}
	if wp.Engine != WhisperEngineLocal && wp.Engine != WhisperEngineGemini {
		transcribe.WakeServer(config.WhisperURL, config.WhisperWakeCommand, opts.Quiet)
	}

	selectedProfile := selectProfile(config, opts.UseLLM)
	batchStartTime := time.Now()

	totalFiles := len(expandedArgs)
	processedCount := 0
	failures := 0

	for idx, inputFile := range expandedArgs {
		hasError, processedFlag, stopFlag := processSingleAudioFile(idx, len(expandedArgs), processedCount, inputFile, opts, config, action, batchStartTime, selectedProfile)
		if hasError || (!processedFlag && !stopFlag && (opts.ForceTranscribe || opts.ForceLLM || opts.Recut || !pipeline.IsEpisodeClean(inputFile))) {
			failures++
		}
		if stopFlag {
			break
		}
		if processedFlag {
			processedCount++
		}
	}

	if (processedCount > 1 || totalFiles > 1) && !opts.Quiet {
		batchDuration := time.Since(batchStartTime)
		fmt.Printf("\nBatch Completed! Processed %d file(s) in %s.\n", processedCount, format.FormatClock(batchDuration.Seconds()))
	}

	os.Stdout.Sync()
	os.Stderr.Sync()
	if failures > 0 {
		return fmt.Errorf("%d episode(s) failed or could not be processed", failures)
	}
	return nil
}
