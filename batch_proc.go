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
			fmt.Fprintf(os.Stderr, "Error collecting from remote %s: %v\n", cli.RemoteHost, err)
			os.Exit(1)
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

	go wakeWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet)

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
			if !cli.Quiet {
				fmt.Println("All audio files are already transcribed, cleaned, or currently processing remotely.")
			}
			return
		}
		if err := runRemotePush(&config, filesToPush, targetHost, nil, cli.Priority, cli.Quiet, cli.Verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error pushing batch to remote %s: %v\n", targetHost, err)
			os.Exit(1)
		}
		return
	}

	selectedProfile := selectProfile(config, cli.UseLLM)
	batchStartTime := time.Now()

	totalFiles := len(expandedArgs)
	processedCount := 0

	for idx, inputFile := range expandedArgs {
		fileStartTime := time.Now()

		if strings.HasSuffix(inputFile, ".json") {
			processJSONFile(inputFile, cli)
			continue
		}

		mainMP3File, precutFile, sourceAudioFile := resolveAudioFiles(inputFile, cli)

		baseName := stripExt(mainMP3File)
		jsonFile := cli.TranscriptPath
		if jsonFile == "" {
			jsonFile = baseName + ".transcript.json"
		}

		outputFile := resolveOutputFile(mainMP3File, cli, totalFiles)

		speedFactor := config.WhisperSpeedFactor
		if speedFactor <= 0 {
			speedFactor = 7.0
		}

		shortName := displayName(filepath.Base(inputFile))

		if !cli.ForceTranscribe && !cli.ForceLLM && !cli.Recut && isEpisodeCompleted(mainMP3File) {
			if cli.Verbose && !cli.Quiet {
				fmt.Printf("skipping: %s\n", shortName)
			}
			continue
		}

		if cli.Count > 0 && processedCount >= cli.Count {
			if !cli.Quiet {
				fmt.Printf("\nReached maximum episode processing limit (%d). Done.\n", cli.Count)
			}
			break
		}

		fileLock, err := acquireFileLock(mainMP3File)
		if err != nil {
			if !cli.Quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to acquire lock for %s: %v\n", shortName, err)
			}
		} else if fileLock == nil {
			if !cli.Quiet {
				fmt.Printf("⏭️  Skipping '%s' (currently being processed by another instance)\n", shortName)
			}
			continue
		}

		processedCount++

		if !cli.Quiet {
			if cli.Count > 0 {
				printSeparator()
				dir := filepath.Dir(inputFile)
				base := filepath.Base(inputFile)
				fmt.Printf("Processing (%d/%d limit):\n  %s\n  %s\n", processedCount, cli.Count, dir, bold(base))
			} else if totalFiles > 1 {
				printSeparator()
				dir := filepath.Dir(inputFile)
				base := filepath.Base(inputFile)
				fmt.Printf("Processing (%d/%d):\n  %s\n  %s\n", idx+1, totalFiles, dir, bold(base))
			} else {
				fmt.Printf("Processing: %s\n", bold(shortName))
			}
		}
		totalDuration := getAudioDuration(sourceAudioFile)
		updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
			st.Status = StateTranscribingLocally
			st.Original.DurationSec = totalDuration
			if fi, err := os.Stat(sourceAudioFile); err == nil {
				st.Original.SizeBytes = fi.Size()
			}
		})

		if cli.TranscribeMin != "" {
			totalDuration = handleTranscribeMin(&sourceAudioFile, totalDuration, cli)
		}

		if cli.Recut {
			handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName, totalDuration, selectedProfile, config, cli, fileStartTime)
			fileLock.Release()
			continue
		}

		costInfo := getProfileCost(selectedProfile)

		if !cli.Quiet {
			fmt.Printf("  Duration: %s\n", bold(formatTime(totalDuration)))
			fmt.Printf("  Profile:  %s\n", bold(fmt.Sprintf("[%d] %s (%s)", selectedProfile.ID, selectedProfile.Name, selectedProfile.Model)))
			fmt.Printf("  Pricing:  %s\n", costInfo.CostStr)
		}

		t0Step1 := time.Now()
		isNewlyTranscribed := false

		whisperLanguage := config.WhisperLanguage
		whisperPrompt := config.WhisperPrompt
		id3Tags := map[string]string{}

		transcriptionData, err := loadOrTranscribe(sourceAudioFile, jsonFile, config, cli, selectedProfile, totalDuration, speedFactor, whisperLanguage, whisperPrompt, id3Tags, &isNewlyTranscribed, &t0Step1)
		if err != nil {
			hasError = true
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fileLock.Release()
			continue
		}

		detectedLang := transcriptionData.Language
		if detectedLang == "" && len(transcriptionData.Segments) > 0 {
			detectedLang = transcriptionData.Segments[0].Language
		}
		if !cli.Quiet && detectedLang != "" {
			langLabel := "(auto-detected)"
			if whisperLanguage != "" {
				langLabel = "(config override)"
			}
			fmt.Printf("   Detected language: %s %s\n", strings.ToUpper(detectedLang), langLabel)
		}

		if whisperLanguage == "" && isNewlyTranscribed {
			fullText := transcriptionData.Text
			if fullText == "" {
				for _, seg := range transcriptionData.Segments {
					fullText += seg.Text + " "
				}
			}
			scriptLang := detectScriptLanguage(fullText)
			if scriptLang != "" && scriptLang != detectedLang {
				transcriptionData.Language = scriptLang
				if !cli.Quiet {
					fmt.Printf("   Corrected language from %s to %s (detected from script)\n", strings.ToUpper(detectedLang), strings.ToUpper(scriptLang))
				}
			}
		}

		if !validateTranscriptSanity(transcriptionData, totalDuration, cli.Quiet) {
			hasError = true
			fileLock.Release()
			continue
		}

		if isNewlyTranscribed && cli.SaveTranscript {
			saveJSONTranscript(mainMP3File, transcriptionData, jsonFile, cli.Quiet, id3Tags)
		}

		if cli.ExportSRT {
			convertJSONToSRT(jsonFile, transcriptionData, cli.TranscriptPath, cli.Quiet)
		}

		if cli.ExportTXT {
			convertJSONToTXT(jsonFile, transcriptionData, totalDuration, cli.TranscriptPath, cli.Quiet)
		}

		if cli.ExportSRT || cli.ExportTXT {
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Printf("Export completed in %s\n", formatClock(fileTotalDuration.Seconds()))
			}
			fileLock.Release()
			continue
		}

		if cli.TranscribeMin != "" {
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Printf("Preview transcription completed in %s\n", formatClock(fileTotalDuration.Seconds()))
				fmt.Println("   Transcript saved - original file was not modified.")
			}
			if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
				os.Remove(sourceAudioFile)
			}
			fileLock.Release()
			continue
		}

		formattedTranscript := formatTranscript(transcriptionData, totalDuration)

		t0Step2 := time.Now()
		if !cli.Quiet {
			fmt.Println()
			fmt.Println(boldYellow("Step 2/3: Detecting ad/sponsor segments via LLM (" + selectedProfile.Model + ")..."))
		}
		adSegments := detectAdsLLM(formattedTranscript, selectedProfile)
		if len(adSegments) > 0 {
			adSegments = mergeIntervals(adSegments)
		}
		step2Duration := time.Since(t0Step2)
		if !cli.Quiet && cli.Verbose {
			fmt.Printf("Step 2/3 (Ad Detection) finished in %s\n", formatClock(step2Duration.Seconds()))
		}

		if len(adSegments) == 0 {
			saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
			updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
				st.Status = StateDone
				st.Cleaned = EpisodeAudioMeta{
					Filename:    filepath.Base(outputFile),
					DurationSec: totalDuration,
				}
				st.Ads = nil
			})
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Println("No ad segments detected by LLM!")
				printTimingSummary(cli.Verbose, totalDuration, totalDuration, 0, 0, 0, step1Duration(t0Step1), step2Duration, 0, fileTotalDuration)
			}
			if sourceAudioFile != outputFile {
				copyFile(sourceAudioFile, outputFile)
			}
			fmt.Printf("Result saved to: '%s'\n", outputFile)
			fileLock.Release()
			continue
		}

		if cli.Verbose && !cli.Quiet {
			fmt.Println()
			fmt.Println("AD SEGMENTS DETECTED TO REMOVE:")
			for _, ad := range adSegments {
				duration := ad.End - ad.Start
				reason := ad.Reason
				if reason == "" {
					reason = "Ad segment"
				}
				fmt.Printf("  - [%s -> %s] (%.1fs): %s\n", formatTime(ad.Start), formatTime(ad.End), duration, reason)
			}
			fmt.Println()
		}

		cutsResult := saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
		keepSegments := cutsResult.KeepSegments

		t0Step3 := time.Now()
		updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
			st.Status = StateCuttingLocally
		})
		if !cli.Quiet {
			fmt.Println()
			fmt.Printf("Step 3/3: Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
		}

		workDir := workDirFor(outputFile)
		os.MkdirAll(workDir, 0755)
		tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
		verifyTempFile(tempOutputFile)

		remoteHost := config.RemoteFFmpegHost
		if cli.RemoteFFmpegHost != "" {
			remoteHost = cli.RemoteFFmpegHost
		}
		if cutAudioFFmpegWithHost(sourceAudioFile, keepSegments, tempOutputFile, remoteHost) {
			step3Duration := time.Since(t0Step3)
			if !cli.Quiet && cli.Verbose {
				fmt.Printf("Step 3/3 (Audio Cutting) finished in %s\n", formatClock(step3Duration.Seconds()))
			}

			if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
				checkPrecutSymlink(precutFile)
				safeMove(mainMP3File, precutFile)
				if !cli.Quiet {
					fmt.Printf("Original file preserved at: '%s'\n", precutFile)
				}
			}

			safeMove(tempOutputFile, outputFile)
			os.RemoveAll(workDir)

			newDuration := getAudioDuration(outputFile)
			actualCut := totalDuration - newDuration
			pctCut := 0.0
			if totalDuration > 0 {
				pctCut = actualCut / totalDuration * 100
			}

			updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
				st.Status = StateDone
				if fileExists(precutFile) {
					st.Original.Filename = filepath.Base(precutFile)
					if fi, err := os.Stat(precutFile); err == nil {
						st.Original.SizeBytes = fi.Size()
					}
				}
				st.Cleaned.Filename = filepath.Base(outputFile)
				st.Cleaned.DurationSec = newDuration
				st.Cleaned.AdDurationSec = actualCut
				if fi, err := os.Stat(outputFile); err == nil {
					st.Cleaned.SizeBytes = fi.Size()
				}
				st.Ads = make([]EpisodeAdCut, 0, len(adSegments))
				for _, ad := range adSegments {
					st.Ads = append(st.Ads, EpisodeAdCut{
						Start:  ad.Start,
						End:    ad.End,
						Reason: ad.Reason,
					})
				}
			})
			fileTotalDuration := time.Since(fileStartTime)

			if !cli.Quiet {
				printFullSummary(cli.Verbose, totalDuration, newDuration, actualCut, pctCut, len(adSegments),
					step1Duration(t0Step1), step2Duration, step3Duration, fileTotalDuration)
				fmt.Printf("\nSuccess! Ad-free episode saved to: '%s'\n", outputFile)
			}
			syncAudiobookshelfDuration(&config, outputFile, newDuration)
		} else {
			hasError = true
			os.Remove(tempOutputFile)
			os.RemoveAll(workDir)
			fmt.Fprintf(os.Stderr, "Failed to output ad-free audio for '%s'.\n", inputFile)
		}

		if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
			os.Remove(sourceAudioFile)
		}
		fileLock.Release()
	}

	if (processedCount > 1 || totalFiles > 1) && !cli.Quiet {
		batchDuration := time.Since(batchStartTime)
		fmt.Printf("\nBatch Completed! Processed %d file(s) in %s.\n", processedCount, formatClock(batchDuration.Seconds()))
	}

	os.Stdout.Sync()
	os.Stderr.Sync()
}
