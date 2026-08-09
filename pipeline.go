package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName string, totalDuration float64, selectedProfile LLMProfile, cli CLIOptions, fileStartTime time.Time) {
	cutsFile := baseName + ".cuts.json"
	if !fileExists(cutsFile) {
		fmt.Fprintf(os.Stderr, "Error: Cut metadata JSON file '%s' not found for recutting.\n", cutsFile)
		return
	}

	jsonFile := cli.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	if fileExists(jsonFile) && !cli.ForceTranscribe && !cli.ForceLLM {
		if !cli.Quiet {
			fmt.Printf("Skipping '%s' (.transcript.json exists). Use --force-transcribe or --force-llm to reprocess.\n", mainMP3File)
		}
		return
	}

	if !cli.Quiet {
		fmt.Printf("Recutting audio using existing cut metadata: '%s'\n", cutsFile)
	}

	data, err := readFile(cutsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading cuts file: %v\n", err)
		return
	}
	var cutsData CutsData
	if err := jsonUnmarshal(data, &cutsData); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cuts file: %v\n", err)
		return
	}

	var existingAds []AdSegment
	for _, c := range cutsData.CutIntervals {
		st := c.StartSec
		en := c.EndSec
		existingAds = append(existingAds, AdSegment{Start: st, End: en, Reason: c.Reason})
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
		return
	}

	if !cli.Quiet {
		mergedIntervals := cutsData.MergedCutIntervals
		if len(mergedIntervals) == 0 {
			fmt.Println("No cut intervals specified in metadata!")
		} else {
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Printf("CUT INTERVALS TO REMOVE (%d segment(s)):\n", len(mergedIntervals))
			fmt.Printf("%s\n", repeatStr("=", 65))
			for _, m := range mergedIntervals {
				duration := m.End - m.Start
				fmt.Printf("  - [%s -> %s] (%.1fs)\n", formatTime(m.Start), formatTime(m.End), duration)
			}
			fmt.Printf("%s\n\n", repeatStr("=", 65))
		}
	}

	t0Recut := time.Now()
	if !cli.Quiet {
		fmt.Printf("Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
	}

	workDir := workDirFor(outputFile)
	os.MkdirAll(workDir, 0755)
	tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
	verifyTempFile(tempOutputFile)

	if cutAudioFFmpeg(sourceAudioFile, keepSegments, tempOutputFile) {
		recutDuration := time.Since(t0Recut)
		if !cli.Quiet {
			fmt.Printf("Audio Recutting finished in %s\n", formatClock(recutDuration.Seconds()))
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
		pctCut := actualCut / totalDuration * 100
		fileTotalDuration := time.Since(fileStartTime)

		if !cli.Quiet {
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Println("DURATION & TIME SAVED SUMMARY (RECUT):")
			fmt.Printf("%s\n", repeatStr("=", 65))
			fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
			fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs)\n", formatTime(actualCut), actualCut)
			fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
			fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
			fmt.Printf("  - Total Recut Time:        %s\n", formatClock(fileTotalDuration.Seconds()))
			fmt.Printf("%s\n\n", repeatStr("=", 65))
			fmt.Printf("Success! Recut ad-free episode saved to: '%s'\n", outputFile)
		}
	} else {
		os.Remove(tempOutputFile)
		os.RemoveAll(workDir)
	}
}

func loadOrTranscribe(sourceAudioFile, jsonFile string, config Config, cli CLIOptions, selectedProfile LLMProfile, totalDuration, speedFactor float64, whisperLanguage, whisperPrompt string, id3TagsOut map[string]string, isNewlyTranscribed *bool, t0Step1 *time.Time) (*TranscriptionData, error) {
	if fileExists(jsonFile) {
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
		step1Duration := time.Since(*t0Step1)
		if !cli.Quiet {
			fmt.Printf("Step 1/3 (Transcript Loaded) finished in %s\n", formatClock(step1Duration.Seconds()))
		}
		return &td, nil
	}

	if !cli.Quiet {
		fmt.Println("Step 1/3: Transcribing audio via AMD GPU Whisper server...")
	}

	if whisperPrompt == "" {
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
		if tagText != "" {
			if !cli.Quiet {
				keys := make([]string, 0, len(id3Tags))
				for k := range id3Tags {
					keys = append(keys, k)
				}
				fmt.Printf("   Extracted ID3 metadata: %s\n", strings.Join(keys, ", "))
				fmt.Println("   Extracting keywords from metadata to improve transcription accuracy...")
			}
			extracted := extractKeywordsLLM(tagText, selectedProfile, cli.Quiet)
			if extracted != "" {
				whisperPrompt = extracted
				if !cli.Quiet {
					fmt.Printf("   Using keywords: %s\n", whisperPrompt)
				}
			}
		} else if !cli.Quiet {
			fmt.Println("   No ID3 metadata found in file for keyword extraction.")
		}
	}

	chunkDuration := config.ChunkDurationSec
	useChunks := cli.UseChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)

	dockerContainer := config.WhisperDockerContainer
	if dockerContainer == "" {
		dockerContainer = detectWhisperDockerContainer(config.WhisperURL)
		if !cli.Quiet && dockerContainer != "" {
			fmt.Printf("   Auto-detected whisper Docker container: '%s'\n", dockerContainer)
		}
	}

	whisperLangArg := whisperLanguage
	whisperPromptArg := whisperPrompt
	if whisperPromptArg == "" {
		whisperPromptArg = ""
	}

	var transcriptionData *TranscriptionData
	var err error

	if useChunks {
		parallelChunks := config.ParallelChunks
		if parallelChunks < 1 {
			parallelChunks = 1
		}
		if !cli.Quiet {
			numChunks := int(totalDuration / float64(chunkDuration))
			if numChunks < 1 {
				numChunks = 1
			}
			fmt.Printf("   Audio is %s long - splitting into %d chunks of %s for reliability...\n",
				formatTime(totalDuration), numChunks, formatTime(float64(chunkDuration)))
			if parallelChunks > 1 {
				fmt.Printf("   Parallel chunks: %d\n", parallelChunks)
			}
		}
		transcriptionData, err = transcribeChunks(
			sourceAudioFile, config.WhisperURL, cli.Quiet,
			totalDuration, speedFactor, chunkDuration, parallelChunks,
			dockerContainer, whisperPromptArg, whisperLangArg,
		)
	} else {
		transcriptionData, err = transcribeWhisper(
			sourceAudioFile, config.WhisperURL, cli.Quiet,
			totalDuration, speedFactor, dockerContainer,
			whisperPromptArg, whisperLangArg, nil,
		)
		if err != nil && strings.Contains(err.Error(), "failed to") && totalDuration > 300 {
			if !cli.Quiet {
				fmt.Println("\nFull-file transcription failed - retrying in chunks...")
			}
			chunkDur := config.ChunkDurationSec
			if chunkDur <= 0 {
				chunkDur = 900
			}
			transcriptionData, err = transcribeChunks(
				sourceAudioFile, config.WhisperURL, cli.Quiet,
				totalDuration, speedFactor, chunkDur, 1,
				dockerContainer, whisperPromptArg, whisperLangArg,
			)
		}
	}

	if err != nil {
		return nil, err
	}

	step1Duration := time.Since(*t0Step1)
	if !cli.Quiet {
		fmt.Printf("Step 1/3 (Transcription) finished in %s\n", formatClock(step1Duration.Seconds()))
	}
	*isNewlyTranscribed = true
	return transcriptionData, nil
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
