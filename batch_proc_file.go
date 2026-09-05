package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processSingleAudioFile(idx, totalFiles, processedCount int, inputFile string, cli CLIOptions, config Config, action string, batchStartTime time.Time, selectedProfile LLMProfile) (hasError bool, processed bool, stop bool) {
	fileStartTime := time.Now()

	if strings.HasSuffix(inputFile, ".json") {
		processJSONFile(inputFile, cli)
		return false, false, false
	}

	mainMP3File, precutFile, sourceAudioFile := resolveAudioFiles(inputFile, cli)
	baseName := stripExt(mainMP3File)
	jsonFile := cli.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	outputFile := resolveOutputFile(mainMP3File, cli, totalFiles)

	fileLock, ok, shouldStop := checkSkipOrLockAudioFile(mainMP3File, inputFile, idx, totalFiles, processedCount, cli)
	if !ok || shouldStop {
		return false, false, shouldStop
	}
	defer fileLock.Release()
	processed = true

	totalDuration := getAudioDuration(sourceAudioFile)
	_ = updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
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
		return false, processed, false
	}

	if isGeminiEngine(config, cli) {
		success := runGeminiPipelineStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile, totalDuration, config, cli, selectedProfile, fileStartTime)
		if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
			os.Remove(sourceAudioFile)
		}
		return !success, processed, false
	}

	transData, t0Step1, ok, hasErr := runLocalTranscriptionStep(sourceAudioFile, jsonFile, mainMP3File, totalDuration, config, cli, selectedProfile, fileStartTime)
	if hasErr || !ok {
		return hasErr, processed, false
	}

	cutSuccess := runLocalAdDetectionAndCutStep(transData, sourceAudioFile, mainMP3File, precutFile, outputFile, totalDuration, config, cli, selectedProfile, fileStartTime, t0Step1)
	if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
		os.Remove(sourceAudioFile)
	}
	return !cutSuccess, processed, false
}

func runLocalTranscriptionStep(sourceAudioFile, jsonFile, mainMP3File string, totalDuration float64, config Config, cli CLIOptions, selectedProfile LLMProfile, fileStartTime time.Time) (*TranscriptionData, time.Time, bool, bool) {
	speedFactor := config.WhisperSpeedFactor
	if speedFactor <= 0 {
		speedFactor = 7.0
	}
	t0Step1 := time.Now()
	isNewlyTranscribed := false
	id3Tags := map[string]string{}

	transcriptionData, err := loadOrTranscribe(sourceAudioFile, jsonFile, config, cli, selectedProfile, totalDuration, speedFactor, config.WhisperLanguage, config.WhisperPrompt, id3Tags, &isNewlyTranscribed, &t0Step1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, t0Step1, false, true
	}

	detectAndSanitizeTranscriptLanguage(transcriptionData, config.WhisperLanguage, isNewlyTranscribed, cli.Quiet)
	if !validateTranscriptSanity(transcriptionData, totalDuration, cli.Quiet) {
		return nil, t0Step1, false, true
	}

	if isNewlyTranscribed && cli.SaveTranscript {
		saveJSONTranscript(mainMP3File, transcriptionData, jsonFile, cli.Quiet, id3Tags)
	}

	if handleExportOrPreviewReturns(transcriptionData, totalDuration, fileStartTime, sourceAudioFile, jsonFile, cli) {
		return nil, t0Step1, false, false
	}
	return transcriptionData, t0Step1, true, false
}

func runLocalAdDetectionAndCutStep(transcriptionData *TranscriptionData, sourceAudioFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, cli CLIOptions, selectedProfile LLMProfile, fileStartTime, t0Step1 time.Time) bool {
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
	if len(adSegments) == 0 {
		handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile, totalDuration, selectedProfile, cli, fileStartTime, t0Step1, t0Step2)
		return true
	}

	cutsResult := saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
	t0Step3 := time.Now()
	return executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile, cutsResult.KeepSegments, adSegments, totalDuration, config, cli, selectedProfile, fileStartTime, t0Step1, t0Step2, t0Step3)
}

func checkSkipOrLockAudioFile(mainMP3File, inputFile string, idx, totalFiles, processedCount int, cli CLIOptions) (*fileLockWrapper, bool, bool) {
	shortName := displayName(filepath.Base(inputFile))
	if !cli.ForceTranscribe && !cli.ForceLLM && !cli.Recut && isEpisodeCompleted(mainMP3File) {
		if cli.Verbose && !cli.Quiet {
			fmt.Printf("skipping: %s\n", shortName)
		}
		return nil, false, false
	}
	if cli.Count > 0 && processedCount >= cli.Count {
		if !cli.Quiet {
			fmt.Printf("\nReached maximum episode processing limit (%d). Done.\n", cli.Count)
		}
		return nil, false, true
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
		return nil, false, false
	}

	if !cli.Quiet {
		if cli.Count > 0 {
			printSeparator()
			fmt.Printf("Processing (%d/%d limit):\n  %s\n  %s\n", processedCount, cli.Count, filepath.Dir(inputFile), bold(filepath.Base(inputFile)))
		} else if totalFiles > 1 {
			printSeparator()
			fmt.Printf("Processing (%d/%d):\n  %s\n  %s\n", idx+1, totalFiles, filepath.Dir(inputFile), bold(filepath.Base(inputFile)))
		} else {
			fmt.Printf("Processing: %s\n", bold(shortName))
		}
	}
	return fileLock, true, false
}

func detectAndSanitizeTranscriptLanguage(transcriptionData *TranscriptionData, whisperLanguage string, isNewlyTranscribed, quiet bool) {
	detectedLang := transcriptionData.Language
	if detectedLang == "" && len(transcriptionData.Segments) > 0 {
		detectedLang = transcriptionData.Segments[0].Language
	}
	if !quiet && detectedLang != "" {
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
			if !quiet {
				fmt.Printf("   Corrected language from %s to %s (detected from script)\n", strings.ToUpper(detectedLang), strings.ToUpper(scriptLang))
			}
		}
	}
}

func handleExportOrPreviewReturns(transcriptionData *TranscriptionData, totalDuration float64, fileStartTime time.Time, sourceAudioFile, jsonFile string, cli CLIOptions) bool {
	if cli.ExportSRT {
		convertJSONToSRT(jsonFile, transcriptionData, cli.TranscriptPath, cli.Quiet)
	}
	if cli.ExportTXT {
		convertJSONToTXT(jsonFile, transcriptionData, totalDuration, cli.TranscriptPath, cli.Quiet)
	}
	if cli.ExportSRT || cli.ExportTXT {
		if !cli.Quiet {
			fmt.Printf("Export completed in %s\n", formatClock(time.Since(fileStartTime).Seconds()))
		}
		return true
	}
	if cli.TranscribeMin != "" {
		if !cli.Quiet {
			fmt.Printf("Preview transcription completed in %s\n   Transcript saved - original file was not modified.\n", formatClock(time.Since(fileStartTime).Seconds()))
		}
		if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
			os.Remove(sourceAudioFile)
		}
		return true
	}
	return false
}

func handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile string, totalDuration float64, selectedProfile LLMProfile, cli CLIOptions, fileStartTime, t0Step1, t0Step2 time.Time) {
	saveCutsJSON(mainMP3File, totalDuration, nil, &selectedProfile, cli.Quiet)
	_ = updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateDone
		st.Cleaned = EpisodeAudioMeta{Filename: filepath.Base(outputFile), DurationSec: totalDuration}
		st.Ads = nil
	})
	if !cli.Quiet {
		fmt.Println("No ad segments detected by LLM!")
		printTimingSummary(cli.Verbose, totalDuration, totalDuration, 0, 0, 0, step1Duration(t0Step1), time.Since(t0Step2), 0, time.Since(fileStartTime))
	}
	if sourceAudioFile != outputFile {
		copyFile(sourceAudioFile, outputFile)
	}
	fmt.Printf("Result saved to: '%s'\n", outputFile)
}

func executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile string, keepSegments [][2]float64, adSegments []AdSegment, totalDuration float64, config Config, cli CLIOptions, selectedProfile LLMProfile, fileStartTime, t0Step1, t0Step2, t0Step3 time.Time) bool {
	_ = updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateCuttingLocally
	})
	if !cli.Quiet {
		fmt.Println()
		fmt.Printf("Step 3/3: Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
	}

	workDir := workDirFor(outputFile)
	_ = os.MkdirAll(workDir, 0755)
	tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
	verifyTempFile(tempOutputFile)

	remoteHost := config.RemoteFFmpegHost
	if cli.RemoteFFmpegHost != "" {
		remoteHost = cli.RemoteFFmpegHost
	}

	if !cutAudioFFmpegWithHost(sourceAudioFile, keepSegments, tempOutputFile, remoteHost) {
		_ = os.Remove(tempOutputFile)
		_ = os.RemoveAll(workDir)
		fmt.Fprintf(os.Stderr, "Failed to output ad-free audio for '%s'.\n", mainMP3File)
		return false
	}

	if !installCutAudioAndPreserveOriginal(sourceAudioFile, mainMP3File, precutFile, outputFile, tempOutputFile, workDir, cli.Quiet) {
		return false
	}
	_ = os.RemoveAll(workDir)

	newDuration := getAudioDuration(outputFile)
	actualCut := totalDuration - newDuration
	pctCut := 0.0
	if totalDuration > 0 {
		pctCut = actualCut / totalDuration * 100
	}

	updateEpisodeStatusAfterCut(mainMP3File, precutFile, outputFile, adSegments, newDuration, actualCut)

	if !cli.Quiet {
		printFullSummary(cli.Verbose, totalDuration, newDuration, actualCut, pctCut, len(adSegments), step1Duration(t0Step1), time.Since(t0Step2), time.Since(t0Step3), time.Since(fileStartTime))
		fmt.Printf("\nSuccess! Ad-free episode saved to: '%s'\n", outputFile)
	}
	syncAudiobookshelfDuration(&config, outputFile, newDuration)
	return true
}

func installCutAudioAndPreserveOriginal(sourceAudioFile, mainMP3File, precutFile, outputFile, tempOutputFile, workDir string, quiet bool) bool {
	if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
		checkPrecutSymlink(precutFile)
		if mvErr := safeMove(mainMP3File, precutFile); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not preserve the original: %v\n", mvErr)
			return false
		}
		if !quiet {
			fmt.Printf("Original file preserved at: '%s'\n", precutFile)
		}
	}

	if mvErr := safeMove(tempOutputFile, outputFile); mvErr != nil {
		fmt.Fprintf(os.Stderr, "Error: could not install the cut audio: %v\n", mvErr)
		return false
	}
	return true
}

func updateEpisodeStatusAfterCut(mainMP3File, precutFile, outputFile string, adSegments []AdSegment, newDuration, actualCut float64) {
	_ = updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
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
			st.Ads = append(st.Ads, EpisodeAdCut(ad))
		}
	})
}
