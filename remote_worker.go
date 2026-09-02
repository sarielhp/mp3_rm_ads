package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runBatchWorker(batchDir string, quiet, verbose bool) error {
	if batchDir == "" {
		return fmt.Errorf("batch directory argument --batch-dir is required")
	}

	manifestPath := filepath.Join(batchDir, "manifest.json")
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest from %s: %w", manifestPath, err)
	}

	outDir := filepath.Join(batchDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	manifest.Status = BatchStatusProcessing
	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_ = saveManifest(manifestPath, manifest)

	ensureConfigExists()
	config := loadConfig()
	selectedProfile := selectProfile(config, "")

	if !quiet {
		fmt.Printf("Worker started processing batch %s (%d items)...\n", manifest.BatchID, len(manifest.Items))
	}

	for i := range manifest.Items {
		item := &manifest.Items[i]
		if item.Status == BatchStatusCompleted {
			continue
		}

		item.Status = BatchStatusProcessing
		manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = saveManifest(manifestPath, manifest)

		if err := processBatchItem(item, batchDir, outDir, config, selectedProfile, quiet, verbose); err != nil {
			item.Status = BatchStatusFailed
			item.Error = err.Error()
		} else {
			item.Status = BatchStatusCompleted
			item.Error = ""
			if !quiet {
				fmt.Printf("Completed item %s (%s, saved %.1fs)\n", item.ID, item.AudioFileName, item.CutDurationSec)
			}
		}

		recalculateManifestStats(manifest)
		_ = saveManifest(manifestPath, manifest)
	}

	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	recalculateManifestStats(manifest)
	if err := saveManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to save final manifest: %w", err)
	}

	if !quiet {
		fmt.Printf("Batch %s worker finished: %d completed, %d failed.\n", manifest.BatchID, manifest.CompletedItems, manifest.FailedItems)
	}
	return nil
}

func processBatchItem(item *RemoteBatchJobItem, batchDir, outDir string, config Config, selectedProfile LLMProfile, quiet, verbose bool) error {
	inputFile := filepath.Join(batchDir, "in", item.AudioFileName)
	if !fileExists(inputFile) {
		return fmt.Errorf("input file %s not found", inputFile)
	}

	baseName := stripExt(item.AudioFileName)
	outMP3 := filepath.Join(outDir, item.AudioFileName)
	outTranscriptJSON := filepath.Join(outDir, baseName+".transcript.json")

	origDuration := getAudioDuration(inputFile)
	if origDuration <= 0 {
		origDuration = getMP3DiskDuration(inputFile)
	}

	whisperLanguage := config.WhisperLanguage
	whisperPrompt := config.WhisperPrompt
	id3Tags := map[string]string{}
	speedFactor := config.WhisperSpeedFactor
	if speedFactor <= 0 {
		speedFactor = 7.0
	}
	t0 := time.Now()
	isNewlyTranscribed := false
	cliOpts := CLIOptions{SaveTranscript: true, Quiet: quiet, Verbose: verbose}

	transcriptionData, err := loadOrTranscribe(inputFile, outTranscriptJSON, config, cliOpts, selectedProfile, origDuration, speedFactor, whisperLanguage, whisperPrompt, id3Tags, &isNewlyTranscribed, &t0)
	if err != nil {
		return fmt.Errorf("transcription error: %w", err)
	}

	saveJSONTranscript(outMP3, transcriptionData, outTranscriptJSON, quiet, id3Tags)

	formattedTranscript := formatTranscript(transcriptionData, origDuration)
	adSegments := detectAdsLLM(formattedTranscript, selectedProfile)
	if len(adSegments) > 0 {
		adSegments = mergeIntervals(adSegments)
	}

	cleanDuration := executeItemAudioCut(item, inputFile, outMP3, batchDir, origDuration, adSegments, selectedProfile, quiet)

	item.CleanedAudioFile = item.AudioFileName
	item.CutsJSONFile = baseName + ".cuts.json"
	item.TranscriptJSONFile = baseName + ".transcript.json"
	item.OriginalDurationSec = origDuration
	item.CleanedDurationSec = cleanDuration
	item.CutDurationSec = origDuration - cleanDuration
	return nil
}

func executeItemAudioCut(item *RemoteBatchJobItem, inputFile, outMP3, batchDir string, origDuration float64, adSegments []AdSegment, selectedProfile LLMProfile, quiet bool) float64 {
	cutsResult := saveCutsJSON(outMP3, origDuration, adSegments, &selectedProfile, quiet)
	keepSegments := cutsResult.KeepSegments

	cleanDuration := origDuration
	if len(adSegments) > 0 && len(keepSegments) > 0 {
		workDir := filepath.Join(batchDir, ".work")
		_ = os.MkdirAll(workDir, 0755)
		tempOut := filepath.Join(workDir, item.AudioFileName+".tmp.mp3")
		verifyTempFile(tempOut)

		if cutAudioFFmpeg(inputFile, keepSegments, tempOut) {
			if mvErr := safeMove(tempOut, outMP3); mvErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not install the cut audio for %s: %v\n", outMP3, mvErr)
				copyFile(inputFile, outMP3)
			} else {
				cleanDuration = getAudioDuration(outMP3)
			}
		} else {
			copyFile(inputFile, outMP3)
		}
		_ = os.RemoveAll(workDir)
	} else {
		copyFile(inputFile, outMP3)
	}
	return cleanDuration
}
