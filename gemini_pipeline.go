package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func convertGeminiToAbsTypes(payload *geminiResponsePayload) (*TranscriptionData, []AdSegment) {
	td := &TranscriptionData{}
	if payload == nil {
		return td, nil
	}
	for _, s := range payload.Segments {
		td.Segments = append(td.Segments, TranscriptionSegment{
			Start: s.Start,
			End:   s.End,
			Text:  s.Text,
		})
		td.Text += s.Text + " "
	}
	td.Text = strings.TrimSpace(td.Text)

	var ads []AdSegment
	for _, c := range payload.Cuts {
		reason := c.Reason
		if c.Type != "" {
			if reason != "" {
				reason = fmt.Sprintf("[%s] %s", c.Type, c.Reason)
			} else {
				reason = fmt.Sprintf("[%s]", c.Type)
			}
		}
		ads = append(ads, AdSegment{
			Start:  c.Start,
			End:    c.End,
			Reason: reason,
		})
	}

	return td, ads
}

func ProcessWithGeminiFlash(ctx context.Context, audioPath, projectID, bucketName string) (*TranscriptionData, []AdSegment, error) {
	fmt.Printf("Uploading '%s' to GCS staging bucket...\n", filepath.Base(audioPath))
	gcsURI, err := uploadAudioToGCS(ctx, bucketName, audioPath)
	if err != nil {
		return nil, nil, err
	}
	defer deleteGCSObject(ctx, bucketName, gcsURI)

	fmt.Println("Analyzing audio with Gemini 1.5 Flash (transcription + ad/music detection)...")
	t0 := time.Now()
	location := os.Getenv("GEMINI_LOCATION")
	if location == "" {
		location = "us-central1"
	}
	payload, err := callGeminiAudioProcessor(ctx, projectID, location, gcsURI)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("Gemini processing finished in %s!\n", formatClock(time.Since(t0).Seconds()))

	td, ads := convertGeminiToAbsTypes(payload)
	return td, ads, nil
}

func isGeminiEngine(config Config, cli CLIOptions) bool {
	if cli.WhisperEngine == string(WhisperEngineGemini) {
		return true
	}
	wp := getActiveWhisperProfile(config)
	return wp.Engine == WhisperEngineGemini
}

func runGeminiPipelineStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, cli CLIOptions, selectedProfile LLMProfile, fileStartTime time.Time) bool {
	ctx := context.Background()
	t0Step1 := time.Now()
	projectID := config.GetGeminiProjectID()
	bucketName := config.GetGeminiStagingBucket()

	td, ads, err := ProcessWithGeminiFlash(ctx, sourceAudioFile, projectID, bucketName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing with Gemini Flash: %v\n", err)
		return false
	}

	if cli.SaveTranscript {
		saveJSONTranscript(mainMP3File, td, jsonFile, cli.Quiet, map[string]string{})
	}

	if handleExportOrPreviewReturns(td, totalDuration, fileStartTime, sourceAudioFile, jsonFile, cli) {
		return true
	}

	if len(ads) > 0 {
		ads = mergeIntervals(ads)
	}

	t0Step2 := time.Now()
	if len(ads) == 0 {
		handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile, totalDuration, selectedProfile, cli, fileStartTime, t0Step1, t0Step2)
		return true
	}

	cutsResult := saveCutsJSON(mainMP3File, totalDuration, ads, &selectedProfile, cli.Quiet)
	t0Step3 := time.Now()
	return executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile, cutsResult.KeepSegments, ads, totalDuration, config, cli, selectedProfile, fileStartTime, t0Step1, t0Step2, t0Step3)
}
