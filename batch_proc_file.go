package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processSingleAudioFile(idx, totalFiles, processedCount int, inputFile string, cli CLIOptions, config Config, action string, batchStartTime time.Time, selectedProfile LLMProfile) (hasError bool, processed bool, stop bool) {
	hasError = false
	processed = false
	fileStartTime := time.Now()

	if strings.HasSuffix(inputFile, ".json") {
		processJSONFile(inputFile, cli)
		return hasError, processed, false
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
		return hasError, processed, false
	}

	if cli.Count > 0 && processedCount >= cli.Count {
		if !cli.Quiet {
			fmt.Printf("\nReached maximum episode processing limit (%d). Done.\n", cli.Count)
		}
		return hasError, processed, true
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
		return hasError, processed, false
	}
	defer fileLock.Release()
	processed = true

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
		return hasError, processed, false
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
		return hasError, processed, false
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
		return hasError, processed, false
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
		return hasError, processed, false
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
		return hasError, processed, false
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
		return hasError, processed, false
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
			if mvErr := safeMove(mainMP3File, precutFile); mvErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not preserve the original: %v\n", mvErr)
				fmt.Fprintf(os.Stderr, "The episode was left unchanged; the cut audio is in %s\n", workDir)
				hasError = true
				return hasError, processed, false
			}
			if !cli.Quiet {
				fmt.Printf("Original file preserved at: '%s'\n", precutFile)
			}
		}

		if mvErr := safeMove(tempOutputFile, outputFile); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not install the cut audio: %v\n", mvErr)
			fmt.Fprintf(os.Stderr, "The original is at %s and the cut audio is in %s; neither was deleted.\n", precutFile, workDir)
			hasError = true
			return hasError, processed, false
		}
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
	return hasError, processed, false
}
