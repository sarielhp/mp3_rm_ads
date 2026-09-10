package adremoval

import (
	"abs/pkg/audio"
	"abs/pkg/format"
	"abs/pkg/pipeline"
	"abs/pkg/remote"
	"abs/pkg/transcribe"
	"abs/pkg/util"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processSingleAudioFile(idx, totalFiles, processedCount int, inputFile string, opts ProcOptions, config Config, action string, batchStartTime time.Time, selectedProfile LLMProfile) (hasError bool, processed bool, stop bool) {
	fileStartTime := time.Now()

	if strings.HasSuffix(inputFile, ".json") {
		pipeline.ProcessJSONFile(inputFile, opts)
		return false, false, false
	}

	mainMP3File, precutFile, sourceAudioFile := resolveAudioFiles(inputFile, opts)
	baseName := util.StripExt(mainMP3File)
	jsonFile := opts.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	outputFile := resolveOutputFile(mainMP3File, opts, totalFiles)

	fileLock, ok, shouldStop := checkSkipOrLockAudioFile(mainMP3File, inputFile, idx, totalFiles, processedCount, opts)
	if !ok || shouldStop {
		return false, false, shouldStop
	}
	defer fileLock.Release()
	processed = true

	totalDuration := audio.GetAudioDuration(sourceAudioFile)
	_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateTranscribingLocally
		st.Original.DurationSec = totalDuration
		if fi, err := os.Stat(sourceAudioFile); err == nil {
			st.Original.SizeBytes = fi.Size()
		}
	})

	if opts.TranscribeMin != "" {
		totalDuration = handleTranscribeMin(&sourceAudioFile, totalDuration, opts)
	}
	if opts.Recut {
		err := pipeline.HandleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName, totalDuration, selectedProfile, config, opts, fileStartTime)
		return err != nil, processed, false
	}

	needsTranscription := !util.FileExists(jsonFile) || opts.ForceTranscribe
	if needsTranscription && canRunSpeculativeRace(config, opts) {
		success, handled := handleSpeculativeStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile, totalDuration, config, opts, selectedProfile, fileStartTime)
		if handled {
			if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
				os.Remove(sourceAudioFile)
			}
			return !success, processed, false
		}
	} else if needsTranscription && isGeminiEngine(config, opts) {
		success, handled := handleGeminiStepWithFallback(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile, totalDuration, config, opts, selectedProfile, fileStartTime)
		if handled {
			if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
				os.Remove(sourceAudioFile)
			}
			return !success, processed, false
		}
	}

	transData, t0Step1, ok, hasErr := runLocalTranscriptionStep(sourceAudioFile, jsonFile, mainMP3File, totalDuration, config, opts, selectedProfile, fileStartTime)
	if hasErr || !ok {
		return hasErr, processed, false
	}

	cutSuccess := runLocalAdDetectionAndCutStep(transData, sourceAudioFile, mainMP3File, precutFile, outputFile, totalDuration, config, opts, selectedProfile, fileStartTime, t0Step1)
	if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
		os.Remove(sourceAudioFile)
	}
	return !cutSuccess, processed, false
}

func canRunSpeculativeRace(config Config, opts ProcOptions) bool {
	if !config.IsSpeculativeTranscriptionEnabled() {
		return false
	}
	if !config.IsGeminiAPIKeyEnabled() || resolveGeminiAPIKey(config) == "" {
		return false
	}
	if opts.WhisperEngine != "" && opts.WhisperEngine != string(WhisperEngineGemini) {
		return false
	}
	return true
}

func handleSpeculativeStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, opts ProcOptions, selectedProfile LLMProfile, fileStartTime time.Time) (bool, bool) {
	t0Step1 := time.Now()
	speedFactor := config.WhisperSpeedFactor
	if speedFactor <= 0 {
		speedFactor = 7.0
	}
	id3Tags := audio.ExtractID3Tags(sourceAudioFile)
	isHebrew := isHebrewAudio(sourceAudioFile, id3Tags, config.WhisperLanguage)
	whisperPrompt := config.WhisperPrompt
	if whisperPrompt == "" {
		whisperPrompt = pipeline.ExtractMetadataPrompt(sourceAudioFile, id3Tags, selectedProfile, opts)
	}
	dockerContainer := config.WhisperDockerContainer
	if dockerContainer == "" {
		dockerContainer = transcribe.DetectWhisperDockerContainer(config.WhisperURL)
	}

	td, ads, geminiWon, err := runSpeculativeParallelRace(context.Background(), sourceAudioFile, config, opts, totalDuration, speedFactor, whisperPrompt, config.WhisperLanguage, dockerContainer, isHebrew)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Speculative transcription error: %v\n", err)
		return false, false
	}

	if opts.SaveTranscript {
		pipeline.SaveJSONTranscript(mainMP3File, td, jsonFile, opts.Quiet, id3Tags)
	}

	if handleExportOrPreviewReturns(td, totalDuration, fileStartTime, sourceAudioFile, jsonFile, opts) {
		return true, true
	}

	if geminiWon {
		return finalizeGeminiRaceWinner(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile, totalDuration, td, ads, selectedProfile, opts, fileStartTime, t0Step1)
	}

	detectAndSanitizeTranscriptLanguage(td, config.WhisperLanguage, true, opts.Quiet)
	if !validateTranscriptSanity(td, totalDuration, opts.Quiet) {
		return false, true
	}

	cutSuccess := runLocalAdDetectionAndCutStep(td, sourceAudioFile, mainMP3File, precutFile, outputFile, totalDuration, config, opts, selectedProfile, fileStartTime, t0Step1)
	return cutSuccess, true
}

func finalizeGeminiRaceWinner(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile string, totalDuration float64, td *TranscriptionData, ads []AdSegment, selectedProfile LLMProfile, opts ProcOptions, fileStartTime, t0Step1 time.Time) (bool, bool) {
	if len(ads) > 0 {
		ads = format.MergeIntervals(ads)
	}
	t0Step2 := time.Now()
	_ = updateTranscriptAdDetectionStatus(jsonFile, true, "completed", "gemini-flash", "", len(ads))
	updateStatusAdDetection(mainMP3File, true, "completed", "gemini-flash", "")
	if len(ads) == 0 {
		handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile, totalDuration, selectedProfile, opts, fileStartTime, t0Step1, t0Step2)
		return true, true
	}
	cutsResult := format.SaveCutsJSON(mainMP3File, totalDuration, ads, &selectedProfile, opts.Quiet)
	t0Step3 := time.Now()
	cutSuccess := executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile, cutsResult.KeepSegments, ads, totalDuration, Config{}, opts, selectedProfile, fileStartTime, t0Step1, t0Step2, t0Step3)
	return cutSuccess, true
}

func handleGeminiStepWithFallback(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, opts ProcOptions, selectedProfile LLMProfile, fileStartTime time.Time) (bool, bool) {
	if runGeminiPipelineStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile, totalDuration, config, opts, selectedProfile, fileStartTime) {
		return true, true
	}
	if !opts.Quiet {
		fmt.Println()
		fmt.Println(util.BoldYellow("Warning: Gemini processing failed. Falling back to local Whisper transcription..."))
	}
	fallbackCfg := prepareWhisperFallbackConfig(config)
	fallbackOpts := opts
	fallbackOpts.WhisperEngine = string(fallbackCfg.WhisperEngine)
	transData, t0Step1, ok, hasErr := runLocalTranscriptionStep(sourceAudioFile, jsonFile, mainMP3File, totalDuration, fallbackCfg, fallbackOpts, selectedProfile, fileStartTime)
	if hasErr || !ok {
		return false, true
	}
	cutSuccess := runLocalAdDetectionAndCutStep(transData, sourceAudioFile, mainMP3File, precutFile, outputFile, totalDuration, fallbackCfg, fallbackOpts, selectedProfile, fileStartTime, t0Step1)
	return cutSuccess, true
}

func runLocalTranscriptionStep(sourceAudioFile, jsonFile, mainMP3File string, totalDuration float64, config Config, opts ProcOptions, selectedProfile LLMProfile, fileStartTime time.Time) (*TranscriptionData, time.Time, bool, bool) {
	speedFactor := config.WhisperSpeedFactor
	if speedFactor <= 0 {
		speedFactor = 7.0
	}
	t0Step1 := time.Now()
	isNewlyTranscribed := false
	id3Tags := map[string]string{}

	transcriptionData, err := pipeline.LoadOrTranscribe(sourceAudioFile, jsonFile, config, opts, selectedProfile, totalDuration, speedFactor, config.WhisperLanguage, config.WhisperPrompt, id3Tags, &isNewlyTranscribed, &t0Step1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, t0Step1, false, true
	}

	detectAndSanitizeTranscriptLanguage(transcriptionData, config.WhisperLanguage, isNewlyTranscribed, opts.Quiet)
	if !validateTranscriptSanity(transcriptionData, totalDuration, opts.Quiet) {
		return nil, t0Step1, false, true
	}

	if isNewlyTranscribed && opts.SaveTranscript {
		pipeline.SaveJSONTranscript(mainMP3File, transcriptionData, jsonFile, opts.Quiet, id3Tags)
	}

	if handleExportOrPreviewReturns(transcriptionData, totalDuration, fileStartTime, sourceAudioFile, jsonFile, opts) {
		return nil, t0Step1, false, false
	}
	return transcriptionData, t0Step1, true, false
}

func runLocalAdDetectionAndCutStep(transcriptionData *TranscriptionData, sourceAudioFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, opts ProcOptions, selectedProfile LLMProfile, fileStartTime, t0Step1 time.Time) bool {
	formattedTranscript := pipeline.FormatTranscript(transcriptionData, totalDuration)
	t0Step2 := time.Now()
	if !opts.Quiet {
		fmt.Println()
		fmt.Println(util.BoldYellow("Step 2/3: Detecting ad/sponsor segments via LLM (" + selectedProfile.Model + ")..."))
	}
	jsonFile := opts.TranscriptPath
	if jsonFile == "" {
		jsonFile = util.StripExt(mainMP3File) + ".transcript.json"
	}
	adSegments, err := detectAdsLLM(formattedTranscript, selectedProfile)
	if err != nil {
		if !opts.Quiet {
			fmt.Fprintf(os.Stderr, "Error during LLM ad detection: %v\n", err)
		}
		_ = updateTranscriptAdDetectionStatus(jsonFile, false, "failed", selectedProfile.Model, err.Error(), 0)
		updateStatusAdDetection(mainMP3File, false, "failed", selectedProfile.Model, err.Error())
		_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
			st.Status = StateFailed
		})
		return false
	}
	_ = updateTranscriptAdDetectionStatus(jsonFile, true, "completed", selectedProfile.Model, "", len(adSegments))
	updateStatusAdDetection(mainMP3File, true, "completed", selectedProfile.Model, "")
	if len(adSegments) > 0 {
		adSegments = format.MergeIntervals(adSegments)
	}
	if len(adSegments) == 0 {
		handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile, totalDuration, selectedProfile, opts, fileStartTime, t0Step1, t0Step2)
		return true
	}

	cutsResult := format.SaveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, opts.Quiet)
	t0Step3 := time.Now()
	return executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile, cutsResult.KeepSegments, adSegments, totalDuration, config, opts, selectedProfile, fileStartTime, t0Step1, t0Step2, t0Step3)
}

func checkSkipOrLockAudioFile(mainMP3File, inputFile string, idx, totalFiles, processedCount int, opts ProcOptions) (*fileLockWrapper, bool, bool) {
	shortName := util.DisplayName(filepath.Base(inputFile))
	if !opts.ForceTranscribe && !opts.ForceLLM && !opts.Recut && pipeline.IsEpisodeClean(mainMP3File) {
		if opts.Verbose && !opts.Quiet {
			fmt.Printf("skipping: %s\n", shortName)
		}
		return nil, false, false
	}
	if opts.Count > 0 && processedCount >= opts.Count {
		if !opts.Quiet {
			fmt.Printf("\nReached maximum episode processing limit (%d). Done.\n", opts.Count)
		}
		return nil, false, true
	}

	fileLock, err := util.AcquireFileLock(mainMP3File)
	if err != nil {
		if !opts.Quiet {
			fmt.Fprintf(os.Stderr, "Cannot safely process %s: %v\n", shortName, err)
		}
		return nil, false, false
	}
	if fileLock == nil {
		if !opts.Quiet {
			fmt.Printf("⏭️  Skipping '%s' (currently being processed by another instance)\n", shortName)
		}
		return nil, false, false
	}

	if !opts.Quiet {
		if opts.Count > 0 {
			fmt.Println()
			fmt.Printf("Processing (%d/%d limit):\n  %s\n  %s\n", processedCount, opts.Count, filepath.Dir(inputFile), util.Bold(filepath.Base(inputFile)))
		} else if totalFiles > 1 {
			fmt.Println()
			fmt.Printf("Processing (%d/%d):\n  %s\n  %s\n", idx+1, totalFiles, filepath.Dir(inputFile), util.Bold(filepath.Base(inputFile)))
		} else {
			fmt.Printf("Processing: %s\n", util.Bold(shortName))
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

func handleExportOrPreviewReturns(transcriptionData *TranscriptionData, totalDuration float64, fileStartTime time.Time, sourceAudioFile, jsonFile string, opts ProcOptions) bool {
	if opts.ExportSRT {
		convertJSONToSRT(jsonFile, transcriptionData, opts.TranscriptPath, opts.Quiet)
	}
	if opts.ExportTXT {
		convertJSONToTXT(jsonFile, transcriptionData, totalDuration, opts.TranscriptPath, opts.Quiet)
	}
	if opts.ExportSRT || opts.ExportTXT {
		if !opts.Quiet {
			fmt.Printf("Export completed in %s\n", format.FormatClock(time.Since(fileStartTime).Seconds()))
		}
		return true
	}
	if opts.TranscribeMin != "" {
		if !opts.Quiet {
			fmt.Printf("Preview transcription completed in %s\n   Transcript saved - original file was not modified.\n", format.FormatClock(time.Since(fileStartTime).Seconds()))
		}
		if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
			os.Remove(sourceAudioFile)
		}
		return true
	}
	return false
}

func installNoAdsOutput(source, output string) error {
	if source == output {
		return nil
	}
	workDir := util.WorkDirFor(output)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(workDir, filepath.Base(output)+".tmp"+filepath.Ext(output))
	util.VerifyTempFile(tmp)

	if err := util.CopyFileErr(source, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := util.SafeMove(tmp, output); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.RemoveAll(workDir)
	return nil
}

func handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile string, totalDuration float64, selectedProfile LLMProfile, opts ProcOptions, fileStartTime, t0Step1, t0Step2 time.Time) {
	if sourceAudioFile != outputFile {
		if err := installNoAdsOutput(sourceAudioFile, outputFile); err != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error installing output file: %v\n", err)
			}
			return
		}
	}
	format.SaveCutsJSON(mainMP3File, totalDuration, nil, &selectedProfile, opts.Quiet)
	_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateDone
		st.Cleaned = EpisodeAudioMeta{Filename: filepath.Base(outputFile), DurationSec: totalDuration}
		st.Ads = nil
	})
	if !opts.Quiet {
		fmt.Println("No ad segments detected by LLM!")
		printTimingSummary(opts.Verbose, totalDuration, totalDuration, 0, 0, 0, time.Since(t0Step1), time.Since(t0Step2), 0, time.Since(fileStartTime))
	}
	fmt.Printf("Result saved to: '%s'\n", outputFile)
}

func executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile string, keepSegments [][2]float64, adSegments []AdSegment, totalDuration float64, config Config, opts ProcOptions, selectedProfile LLMProfile, fileStartTime, t0Step1, t0Step2, t0Step3 time.Time) bool {
	_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateCuttingLocally
	})
	if !opts.Quiet {
		fmt.Println()
		fmt.Printf("Step 3/3: Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
	}

	workDir := util.WorkDirFor(outputFile)
	_ = os.MkdirAll(workDir, 0755)
	tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
	util.VerifyTempFile(tempOutputFile)

	remoteHost := config.RemoteFFmpegHost
	if opts.RemoteFFmpegHost != "" {
		remoteHost = opts.RemoteFFmpegHost
	}
	if opts.Local {
		remoteHost = ""
	}

	if !cutAudioFFmpegWithHost(sourceAudioFile, keepSegments, tempOutputFile, remoteHost) {
		_ = os.Remove(tempOutputFile)
		_ = os.RemoveAll(workDir)
		fmt.Fprintf(os.Stderr, "Failed to output ad-free audio for '%s'.\n", mainMP3File)
		return false
	}

	if !installCutAudioAndPreserveOriginal(sourceAudioFile, mainMP3File, precutFile, outputFile, tempOutputFile, workDir, opts.Quiet) {
		return false
	}
	_ = os.RemoveAll(workDir)

	newDuration := audio.GetAudioDuration(outputFile)
	actualCut := totalDuration - newDuration
	pctCut := 0.0
	if totalDuration > 0 {
		pctCut = actualCut / totalDuration * 100
	}

	updateEpisodeStatusAfterCut(mainMP3File, precutFile, outputFile, adSegments, newDuration, actualCut)

	if !opts.Quiet {
		printFullSummary(opts.Verbose, totalDuration, newDuration, actualCut, pctCut, len(adSegments), time.Since(t0Step1), time.Since(t0Step2), time.Since(t0Step3), time.Since(fileStartTime))
		fmt.Printf("\nSuccess! Ad-free episode saved to: '%s'\n", outputFile)
	}
	remote.SyncAudiobookshelfDuration(&config, outputFile, newDuration)
	return true
}

func installCutAudioAndPreserveOriginal(sourceAudioFile, mainMP3File, precutFile, outputFile, tempOutputFile, workDir string, quiet bool) bool {
	preserved := false
	if sourceAudioFile == mainMP3File && util.FileExists(mainMP3File) {
		checkPrecutSymlink(precutFile)
		if err := os.Link(mainMP3File, precutFile); err != nil {
			if cpErr := util.CopyFileErr(mainMP3File, precutFile); cpErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not preserve the original: %v\n", cpErr)
				return false
			}
		}
		preserved = true
		if !quiet {
			fmt.Printf("Original file preserved at: '%s'\n", precutFile)
		}
	}

	if mvErr := util.SafeMove(tempOutputFile, outputFile); mvErr != nil {
		if preserved {
			_ = os.Remove(precutFile)
		}
		fmt.Fprintf(os.Stderr, "Error: could not install the cut audio: %v\n", mvErr)
		return false
	}
	return true
}

func updateEpisodeStatusAfterCut(mainMP3File, precutFile, outputFile string, adSegments []AdSegment, newDuration, actualCut float64) {
	_ = pipeline.UpdateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
		st.Status = StateDone
		if util.FileExists(precutFile) {
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
