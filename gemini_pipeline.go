package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultGeminiChunkSec = 1800.0

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

func computeGeminiChunks(totalDuration, chunkDurSec float64) []geminiChunkInfo {
	if chunkDurSec <= 0 {
		chunkDurSec = defaultGeminiChunkSec
	}
	if totalDuration <= chunkDurSec {
		return []geminiChunkInfo{{index: 0, startSec: 0, durSec: totalDuration}}
	}
	numChunks := int(math.Ceil(totalDuration / chunkDurSec))
	chunks := make([]geminiChunkInfo, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		st := float64(i) * chunkDurSec
		dur := chunkDurSec
		if st+dur > totalDuration {
			dur = totalDuration - st
		}
		chunks = append(chunks, geminiChunkInfo{
			index:    i,
			startSec: st,
			durSec:   dur,
		})
	}
	return chunks
}

func splitAudioChunk(inputPath, outputPath string, startSec, durSec float64) error {
	verifyTempFile(outputPath)
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-t", fmt.Sprintf("%.3f", durSec),
		"-i", inputPath,
		"-c", "copy", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fbCmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
			"-ss", fmt.Sprintf("%.3f", startSec),
			"-t", fmt.Sprintf("%.3f", durSec),
			"-i", inputPath,
			"-c:a", "libmp3lame", "-b:a", "128k", outputPath)
		if fbOut, fbErr := fbCmd.CombinedOutput(); fbErr != nil {
			return fmt.Errorf("split audio chunk failed: %w (copy: %s, fallback: %s)", fbErr, string(out), string(fbOut))
		}
	}
	return nil
}

func prepareGeminiChunks(audioPath string, chunks []geminiChunkInfo) ([]geminiChunkInfo, func(), error) {
	if len(chunks) <= 1 {
		if len(chunks) == 1 {
			chunks[0].filePath = audioPath
		}
		return chunks, func() {}, nil
	}

	workDir := workDirFor(audioPath)
	_ = os.MkdirAll(workDir, 0755)
	ext := filepath.Ext(audioPath)
	if ext == "" {
		ext = ".mp3"
	}

	prepared := make([]geminiChunkInfo, len(chunks))
	cleanup := func() {
		for _, ch := range prepared {
			if ch.filePath != "" && ch.filePath != audioPath {
				_ = os.Remove(ch.filePath)
			}
		}
	}

	for i, ch := range chunks {
		chunkFile := filepath.Join(workDir, fmt.Sprintf("%s.gemini_chunk_%d_%d%s", filepath.Base(audioPath), i, time.Now().UnixNano(), ext))
		if err := splitAudioChunk(audioPath, chunkFile, ch.startSec, ch.durSec); err != nil {
			cleanup()
			return nil, nil, err
		}
		ch.filePath = chunkFile
		prepared[i] = ch
	}

	return prepared, cleanup, nil
}

func processSingleGeminiChunk(ctx context.Context, ch geminiChunkInfo, projectID, location, bucketName string) (*geminiChunkResult, error) {
	gcsURI, err := uploadAudioToGCS(ctx, bucketName, ch.filePath)
	if err != nil {
		return nil, fmt.Errorf("chunk %d upload failed: %w", ch.index, err)
	}
	defer deleteGCSObject(ctx, bucketName, gcsURI)

	payload, err := callGeminiAudioProcessor(ctx, projectID, location, gcsURI)
	if err != nil {
		return nil, fmt.Errorf("chunk %d processing failed: %w", ch.index, err)
	}

	return &geminiChunkResult{
		index:    ch.index,
		startSec: ch.startSec,
		payload:  payload,
	}, nil
}

func processGeminiChunksParallel(ctx context.Context, chunks []geminiChunkInfo, projectID, location, bucketName string) ([]*geminiChunkResult, error) {
	results := make([]*geminiChunkResult, len(chunks))
	var wg syncWG
	var mu syncMutex
	var firstErr error

	for i, ch := range chunks {
		wg.Add(1)
		go func(idx int, chunk geminiChunkInfo) {
			defer wg.Done()
			res, err := processSingleGeminiChunk(ctx, chunk, projectID, location, bucketName)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			results[idx] = res
		}(i, ch)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func mergeGeminiChunkResults(results []*geminiChunkResult) *geminiResponsePayload {
	merged := &geminiResponsePayload{}
	for _, res := range results {
		if res == nil || res.payload == nil {
			continue
		}
		offset := res.startSec
		for _, c := range res.payload.Cuts {
			merged.Cuts = append(merged.Cuts, geminiCutItem{
				Start:  c.Start + offset,
				End:    c.End + offset,
				Type:   c.Type,
				Reason: c.Reason,
			})
		}
		for _, s := range res.payload.Segments {
			merged.Segments = append(merged.Segments, geminiSegmentItem{
				Start: s.Start + offset,
				End:   s.End + offset,
				Text:  s.Text,
			})
		}
	}
	return merged
}

func ProcessWithGeminiFlash(ctx context.Context, audioPath, projectID, bucketName string) (*TranscriptionData, []AdSegment, error) {
	return ProcessWithGeminiFlashChunks(ctx, audioPath, projectID, bucketName, defaultGeminiChunkSec)
}

func ProcessWithGeminiFlashChunks(ctx context.Context, audioPath, projectID, bucketName string, chunkDurSec float64) (*TranscriptionData, []AdSegment, error) {
	totDur := getAudioDuration(audioPath)
	if totDur <= 0 {
		totDur = defaultGeminiChunkSec
	}
	chunks := computeGeminiChunks(totDur, chunkDurSec)
	prepared, cleanup, err := prepareGeminiChunks(audioPath, chunks)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	if len(prepared) > 1 {
		fmt.Printf("Splitting '%s' (%s) into %d parallel 30-min chunks for Gemini...\n",
			filepath.Base(audioPath), formatTime(totDur), len(prepared))
	} else {
		fmt.Printf("Uploading '%s' to GCS staging bucket...\n", filepath.Base(audioPath))
	}

	t0 := time.Now()
	location := os.Getenv("GEMINI_LOCATION")
	if location == "" {
		location = "us-central1"
	}
	results, err := processGeminiChunksParallel(ctx, prepared, projectID, location, bucketName)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("Gemini processing finished in %s across %d chunk(s)!\n",
		formatClock(time.Since(t0).Seconds()), len(prepared))

	merged := mergeGeminiChunkResults(results)
	td, ads := convertGeminiToAbsTypes(merged)
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

	chunkDur := defaultGeminiChunkSec
	if config.ChunkDurationSec > 0 {
		chunkDur = float64(config.ChunkDurationSec)
	}
	td, ads, err := ProcessWithGeminiFlashChunks(ctx, sourceAudioFile, projectID, bucketName, chunkDur)
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
