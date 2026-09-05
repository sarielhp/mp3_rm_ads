package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName string, totalDuration float64, selectedProfile LLMProfile, config Config, cli CLIOptions, fileStartTime time.Time) {
	cutsFile := baseName + ".cuts.json"
	if !fileExists(cutsFile) {
		fmt.Fprintf(os.Stderr, "Error: Cut metadata JSON file '%s' not found for recutting.\n", cutsFile)
		return
	}

	keepSegments, _, ok := loadRecutKeepSegments(cutsFile, mainMP3File, totalDuration, selectedProfile, cli)
	if !ok || len(keepSegments) == 0 {
		return
	}

	workDir := workDirFor(outputFile)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating work directory '%s': %v\n", workDir, err)
		return
	}
	tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
	verifyTempFile(tempOutputFile)

	executeRecutAudio(sourceAudioFile, precutFile, outputFile, tempOutputFile, mainMP3File, workDir, keepSegments, totalDuration, config, cli, fileStartTime)
}

func loadRecutKeepSegments(cutsFile, mainMP3File string, totalDuration float64, selectedProfile LLMProfile, cli CLIOptions) ([][2]float64, CutsData, bool) {
	if !cli.Quiet {
		fmt.Printf("Recutting audio using existing cut metadata: '%s'\n", cutsFile)
	}

	data, err := readFile(cutsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading cuts file: %v\n", err)
		return nil, CutsData{}, false
	}
	var cutsData CutsData
	if err := jsonUnmarshal(data, &cutsData); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cuts file: %v\n", err)
		return nil, CutsData{}, false
	}

	var existingAds []AdSegment
	for _, c := range cutsData.CutIntervals {
		existingAds = append(existingAds, AdSegment{Start: c.StartSec, End: c.EndSec, Reason: c.Reason})
	}
	if len(existingAds) > 0 {
		existingAds = mergeIntervals(existingAds)
	}

	cutsResult := saveCutsJSON(mainMP3File, totalDuration, existingAds, &selectedProfile, cli.Quiet)
	keepSegments := cutsResult.KeepSegments
	if len(keepSegments) == 0 {
		if !cli.Quiet {
			fmt.Println("No keep segments found in cut metadata.")
		}
		return nil, cutsData, false
	}

	if cli.Verbose && !cli.Quiet && len(cutsData.MergedCutIntervals) > 0 {
		fmt.Println("\nCUT INTERVALS TO REMOVE:")
		for _, m := range cutsData.MergedCutIntervals {
			fmt.Printf("  - [%s -> %s] (%.1fs)\n", formatTime(m.Start), formatTime(m.End), m.End-m.Start)
		}
		fmt.Println()
	}
	return keepSegments, cutsData, true
}

func executeRecutAudio(sourceAudioFile, precutFile, outputFile, tempOutputFile, mainMP3File, workDir string, keepSegments [][2]float64, totalDuration float64, config Config, cli CLIOptions, fileStartTime time.Time) {
	t0Recut := time.Now()
	if !cli.Quiet {
		fmt.Printf("Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
	}

	remoteHost := config.RemoteFFmpegHost
	if cli.RemoteFFmpegHost != "" {
		remoteHost = cli.RemoteFFmpegHost
	}
	if !cutAudioFFmpegWithHost(sourceAudioFile, keepSegments, tempOutputFile, remoteHost) {
		os.Remove(tempOutputFile)
		os.RemoveAll(workDir)
		return
	}

	if !cli.Quiet && cli.Verbose {
		fmt.Printf("Audio Recutting finished in %s\n", formatClock(time.Since(t0Recut).Seconds()))
	}

	if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
		checkPrecutSymlink(precutFile)
		if mvErr := safeMove(mainMP3File, precutFile); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not preserve the original: %v\n", mvErr)
			return
		}
		if !cli.Quiet {
			fmt.Printf("Original file preserved at: '%s'\n", precutFile)
		}
	}

	if mvErr := safeMove(tempOutputFile, outputFile); mvErr != nil {
		fmt.Fprintf(os.Stderr, "Error: could not install the recut audio: %v\n", mvErr)
		return
	}
	os.RemoveAll(workDir)

	finishRecutStatusAndSummary(mainMP3File, precutFile, outputFile, totalDuration, config, cli, fileStartTime)
}

func finishRecutStatusAndSummary(mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, cli CLIOptions, fileStartTime time.Time) {
	newDuration := getAudioDuration(outputFile)
	actualCut := totalDuration - newDuration
	pctCut := 0.0
	if totalDuration > 0 {
		pctCut = actualCut / totalDuration * 100
	}

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
	})
	syncAudiobookshelfDuration(&config, outputFile, newDuration)

	if !cli.Quiet {
		fmt.Println()
		fmt.Println("DURATION & TIME SAVED SUMMARY (RECUT):")
		fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
		fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs)\n", formatTime(actualCut), actualCut)
		fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
		fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
		fmt.Printf("  - Total Recut Time:        %s\n", formatClock(time.Since(fileStartTime).Seconds()))
		fmt.Printf("Success! Recut ad-free episode saved to: '%s'\n", outputFile)
	}
}

func loadOrTranscribe(sourceAudioFile, jsonFile string, config Config, cli CLIOptions, selectedProfile LLMProfile, totalDuration, speedFactor float64, whisperLanguage, whisperPrompt string, id3TagsOut map[string]string, isNewlyTranscribed *bool, t0Step1 *time.Time) (*TranscriptionData, error) {
	if fileExists(jsonFile) && !cli.ForceTranscribe {
		if !cli.Quiet {
			fmt.Printf("Found existing transcript JSON file: '%s'. Reusing transcript...\n", jsonFile)
		}
		data, err := readFile(jsonFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read transcript file: %w", err)
		}
		var td TranscriptionData
		if err := jsonUnmarshal(data, &td); err != nil {
			return nil, fmt.Errorf("failed to parse transcript JSON: %w", err)
		}
		if !cli.Quiet && cli.Verbose {
			fmt.Printf("\nStep 1/3 (Transcript Loaded) finished in %s\n", formatClock(time.Since(*t0Step1).Seconds()))
		}
		return &td, nil
	}

	if !cli.Quiet {
		fmt.Println()
		fmt.Println(boldYellow("Step 1/3: Transcribing audio via AMD GPU Whisper server..."))
	}

	if whisperPrompt == "" {
		whisperPrompt = extractMetadataPrompt(sourceAudioFile, id3TagsOut, selectedProfile, cli)
	}

	dockerContainer := config.WhisperDockerContainer
	if dockerContainer == "" {
		dockerContainer = detectWhisperDockerContainer(config.WhisperURL)
		if cli.Verbose && dockerContainer != "" {
			fmt.Printf("   Auto-detected whisper Docker container: '%s'\n", dockerContainer)
		}
	}

	transcriptionData, err := runWhisperTranscription(sourceAudioFile, config, cli, totalDuration, speedFactor, whisperPrompt, whisperLanguage, dockerContainer)
	if err != nil {
		return nil, err
	}

	if !cli.Quiet && cli.Verbose {
		fmt.Printf("Step 1/3 (Transcription) finished in %s\n", formatClock(time.Since(*t0Step1).Seconds()))
	}
	*isNewlyTranscribed = true
	return transcriptionData, nil
}

func extractMetadataPrompt(sourceAudioFile string, id3TagsOut map[string]string, selectedProfile LLMProfile, cli CLIOptions) string {
	id3Tags := extractID3Tags(sourceAudioFile)
	for k, v := range id3Tags {
		id3TagsOut[k] = v
	}
	var tagTexts []string
	for _, key := range []string{"title", "artist", "album", "genre", "comment", "description", "synopsis", "purl", "encodedby", "copyright"} {
		if val, ok := id3TagsOut[key]; ok && val != "" {
			tagTexts = append(tagTexts, val)
		}
	}
	tagText := strings.Join(tagTexts, "\n")
	if tagText == "" {
		if !cli.Quiet {
			fmt.Println("   No ID3 metadata found in file for keyword extraction.")
		}
		return ""
	}

	if !cli.Quiet {
		if cli.Verbose {
			keys := make([]string, 0, len(id3Tags))
			for k := range id3Tags {
				keys = append(keys, k)
			}
			fmt.Printf("   Extracted ID3 metadata: %s\n", strings.Join(keys, ", "))
		}
		fmt.Println("   Extracting keywords from metadata to improve transcription accuracy...")
	}
	extracted := extractKeywordsLLM(tagText, selectedProfile, cli.Quiet)
	if extracted != "" && cli.Verbose {
		fmt.Printf("   Using keywords: %s\n", extracted)
	}
	return extracted
}

func runWhisperTranscription(sourceAudioFile string, config Config, cli CLIOptions, totalDuration, speedFactor float64, whisperPrompt, whisperLang, dockerContainer string) (*TranscriptionData, error) {
	wp := getActiveWhisperProfile(config)
	if cli.WhisperEngine != "" {
		wp.Engine = WhisperEngine(cli.WhisperEngine)
	}
	if cli.WhisperModel != "" {
		wp.Model = cli.WhisperModel
	}

	if wp.Engine == WhisperEngineLocal {
		return runWhisperCLITranscription(sourceAudioFile, wp, cli.Quiet, cli.Verbose, whisperPrompt, whisperLang)
	}
	if wp.Engine == WhisperEngineGemini {
		td, _, err := ProcessWithGeminiFlash(context.Background(), sourceAudioFile, config.GetGeminiProjectID(), config.GetGeminiStagingBucket())
		return td, err
	}

	chunkDuration := config.ChunkDurationSec
	useChunks := cli.UseChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)

	if useChunks {
		if !cli.Quiet {
			numChunks := int(totalDuration / float64(chunkDuration))
			if numChunks < 1 {
				numChunks = 1
			}
			fmt.Printf("   Audio is %s long - splitting into %d chunks of %s for reliability...\n",
				formatTime(totalDuration), numChunks, formatTime(float64(chunkDuration)))
		}
		return transcribeChunks(
			sourceAudioFile, config.WhisperURL, cli.Quiet, cli.Verbose,
			totalDuration, speedFactor, chunkDuration,
			dockerContainer, whisperPrompt, whisperLang,
		)
	}

	data, err := transcribeWhisper(
		sourceAudioFile, config.WhisperURL, cli.Quiet, cli.Verbose,
		totalDuration, speedFactor, dockerContainer,
		whisperPrompt, whisperLang, nil,
	)
	if err != nil && strings.Contains(err.Error(), "failed to") && totalDuration > 300 {
		if !cli.Quiet {
			fmt.Println("\nFull-file transcription failed - retrying in chunks...")
		}
		chunkDur := config.ChunkDurationSec
		if chunkDur <= 0 {
			chunkDur = 900
		}
		return transcribeChunks(
			sourceAudioFile, config.WhisperURL, cli.Quiet, cli.Verbose,
			totalDuration, speedFactor, chunkDur,
			dockerContainer, whisperPrompt, whisperLang,
		)
	}
	return data, err
}

func formatTranscript(data *TranscriptionData, totalDuration float64) string {
	segments := data.Segments
	if len(segments) == 0 && data.Text != "" {
		return fmt.Sprintf("[0.0s -> %.1fs] %s", totalDuration, data.Text)
	}

	var lines []string
	for _, seg := range segments {
		lines = append(lines, fmt.Sprintf("[%.1fs -> %.1fs] %s", seg.Start, seg.End, seg.Text))
	}
	return strings.Join(lines, "\n")
}

func processJSONFile(inputFile string, cli CLIOptions) {
	if !fileExists(inputFile) {
		fmt.Fprintf(os.Stderr, "Error: Transcript JSON file '%s' not found.\n", inputFile)
		return
	}

	if !cli.Quiet {
		fmt.Printf("Processing transcript JSON file: '%s'\n", inputFile)
	}

	if !cli.ExportSRT && !cli.ExportTXT {
		cli.ExportSRT = true
		cli.ExportTXT = true
	}

	if cli.ExportSRT {
		convertJSONToSRT(inputFile, nil, cli.TranscriptPath, cli.Quiet)
	}
	if cli.ExportTXT {
		convertJSONToTXT(inputFile, nil, 0, cli.TranscriptPath, cli.Quiet)
	}
}
