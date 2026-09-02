package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const wavSampleRate = 16000
const wavBytesPerSec = wavSampleRate * 2

func transcribeWhisper(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*TranscriptionData, error) {
	bodyBytes, contentType, err := buildWhisperMultipartBody(audioPath, prompt, language, pcmData)
	if err != nil {
		return nil, err
	}

	maxRetries := 5
	retryDelay := 5
	readTimeout := int(totalDuration*1.5) + 600
	if readTimeout < 1800 {
		readTimeout = 1800
	}

	client := &http.Client{
		Timeout: time.Duration(readTimeout) * time.Second,
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		data, err := executeWhisperAttempt(client, whisperURL, contentType, bodyBytes, quiet, verbose)
		if err == nil {
			return data, nil
		}

		if attempt < maxRetries {
			if !quiet {
				fmt.Printf("\nWhisper server error (attempt %d/%d): %v\n", attempt, maxRetries, err)
				fmt.Printf("   Retrying in %d seconds...\n", retryDelay)
			}
			time.Sleep(time.Duration(retryDelay) * time.Second)
		} else {
			return nil, fmt.Errorf("failed to connect to Whisper GPU server at '%s' after %d attempts: %w", whisperURL, maxRetries, err)
		}
	}

	return nil, fmt.Errorf("whisper transcription failed after %d attempts", maxRetries)
}

func buildWhisperMultipartBody(audioPath, prompt, language string, pcmData []byte) ([]byte, string, error) {
	var audioContent []byte
	if pcmData != nil {
		header := buildWavHeader(len(pcmData))
		audioContent = append(header, pcmData...)
	} else {
		var err error
		audioContent, err = os.ReadFile(audioPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read audio file: %w", err)
		}
	}

	boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.SetBoundary(boundary)

	fw, err := w.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := fw.Write(audioContent); err != nil {
		return nil, "", fmt.Errorf("failed to write audio content: %w", err)
	}

	w.WriteField("response_format", "verbose_json")
	w.WriteField("temperature", "0.0")
	if language != "" && language != "auto" {
		w.WriteField("language", language)
	}
	if prompt != "" {
		w.WriteField("prompt", prompt)
	}
	w.Close()

	return buf.Bytes(), w.FormDataContentType(), nil
}

func executeWhisperAttempt(client *http.Client, uri, contentType string, bodyData []byte, quiet, verbose bool) (*TranscriptionData, error) {
	progressDone := make(chan struct{})
	defer close(progressDone)

	startTime := time.Now()
	req, err := http.NewRequest("POST", uri, bytes.NewReader(bodyData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	if !quiet {
		go startTranscriptionProgressTicker(startTime, progressDone)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !quiet && verbose {
		elapsed := time.Since(startTime)
		fmt.Printf("\rTranscription finished in %s!                                  \n", formatClock(elapsed.Seconds()))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var data TranscriptionData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse transcription JSON: %w", err)
	}
	return &data, nil
}

func startTranscriptionProgressTicker(startTime time.Time, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			fmt.Printf("\rTranscribing audio... Elapsed: %s   ", formatClock(elapsed.Seconds()))
		}
	}
}

func sortSegments(segs []TranscriptionSegment) {
	for i := 0; i < len(segs); i++ {
		for j := i + 1; j < len(segs); j++ {
			if segs[j].Start < segs[i].Start {
				segs[i], segs[j] = segs[j], segs[i]
			}
		}
	}
}

func mergeSegments(segs []TranscriptionSegment) []TranscriptionSegment {
	if len(segs) == 0 {
		return segs
	}

	merged := []TranscriptionSegment{segs[0]}
	for i := 1; i < len(segs); i++ {
		last := &merged[len(merged)-1]
		seg := segs[i]
		if seg.Start <= last.End+0.5 {
			if seg.End > last.End {
				last.End = seg.End
				last.Text = last.Text + " " + seg.Text
			}
		} else {
			merged = append(merged, seg)
		}
	}
	return merged
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
