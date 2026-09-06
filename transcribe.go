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
	"sort"
	"strings"
	"time"
)

const wavSampleRate = 16000
const wavBytesPerSec = wavSampleRate * 2

func transcribeWhisper(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*TranscriptionData, error) {
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
		bodyReader, contentType, err := buildWhisperMultipartBody(audioPath, prompt, language, pcmData)
		if err != nil {
			return nil, err
		}

		data, err := executeWhisperAttempt(client, whisperURL, contentType, bodyReader, quiet, verbose)
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

func buildWhisperMultipartBody(audioPath, prompt, language string, pcmData []byte) (io.ReadCloser, string, error) {
	var audioSource io.Reader
	var closeSrc func() error

	if pcmData != nil {
		header := buildWavHeader(len(pcmData))
		audioSource = io.MultiReader(bytes.NewReader(header), bytes.NewReader(pcmData))
	} else {
		f, err := os.Open(audioPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open audio file: %w", err)
		}
		audioSource = f
		closeSrc = f.Close
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		if closeSrc != nil {
			defer closeSrc()
		}
		err := writeWhisperMultipartFields(mw, audioSource, filepath.Base(audioPath), prompt, language)
		if closeErr := mw.Close(); err == nil {
			err = closeErr
		}
		_ = pw.CloseWithError(err)
	}()

	return pr, mw.FormDataContentType(), nil
}

func writeWhisperMultipartFields(mw *multipart.Writer, audioSource io.Reader, filename, prompt, language string) error {
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, audioSource); err != nil {
		return err
	}
	if err := mw.WriteField("response_format", "verbose_json"); err != nil {
		return err
	}
	if err := mw.WriteField("temperature", "0.0"); err != nil {
		return err
	}
	if language != "" && language != "auto" {
		if err := mw.WriteField("language", language); err != nil {
			return err
		}
	}
	if prompt != "" {
		if err := mw.WriteField("prompt", prompt); err != nil {
			return err
		}
	}
	return nil
}

const maxWhisperResponseBytes int64 = 128 << 20

func readLimitedBody(r io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes limit", maxBytes)
	}
	return body, nil
}

func executeWhisperAttempt(client *http.Client, uri, contentType string, bodyReader io.ReadCloser, quiet, verbose bool) (*TranscriptionData, error) {
	defer bodyReader.Close()

	progressDone := make(chan struct{})
	defer close(progressDone)

	startTime := time.Now()
	req, err := http.NewRequest("POST", uri, bodyReader)
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

	body, err := readLimitedBody(resp.Body, maxWhisperResponseBytes)
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
	sort.Slice(segs, func(i, j int) bool {
		return segs[i].Start < segs[j].Start
	})
}

func joinSegmentText(segs []TranscriptionSegment) string {
	var b strings.Builder
	total := 0
	for _, seg := range segs {
		total += len(seg.Text) + 1
	}
	b.Grow(total)
	for i, seg := range segs {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(seg.Text)
	}
	return strings.TrimSpace(b.String())
}

func mergeSegments(segs []TranscriptionSegment) []TranscriptionSegment {
	if len(segs) == 0 {
		return segs
	}

	merged := make([]TranscriptionSegment, 0, len(segs))
	current := segs[0]
	currentParts := []string{segs[0].Text}

	for i := 1; i < len(segs); i++ {
		seg := segs[i]
		if seg.Start <= current.End+0.5 {
			if seg.End > current.End {
				current.End = seg.End
				currentParts = append(currentParts, seg.Text)
			}
		} else {
			current.Text = strings.Join(currentParts, " ")
			merged = append(merged, current)
			current = seg
			currentParts = []string{seg.Text}
		}
	}
	current.Text = strings.Join(currentParts, " ")
	merged = append(merged, current)
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
