package transcribe

import (
	"bytes"
	"context"
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

	"abs/pkg/format"
	"abs/pkg/types"
)

const WavBytesPerSec = WavSampleRate * 2
const maxWhisperResponseBytes int64 = 128 << 20

func TranscribeWhisper(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*types.TranscriptionData, error) {
	return TranscribeWhisperContext(context.Background(), audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, dockerContainer, prompt, language, pcmData)
}

func TranscribeWhisperContext(ctx context.Context, audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*types.TranscriptionData, error) {
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
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		bodyReader, contentType, err := BuildWhisperMultipartBody(audioPath, prompt, language, pcmData)
		if err != nil {
			return nil, err
		}

		data, err := ExecuteWhisperAttemptContext(ctx, client, whisperURL, contentType, bodyReader, quiet, verbose)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt < maxRetries {
			if !quiet {
				fmt.Printf("\nWhisper server error (attempt %d/%d): %v\n\n", attempt, maxRetries, err)
				fmt.Printf("   Retrying in %d seconds...\n", retryDelay)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(retryDelay) * time.Second):
			}
		} else {
			return nil, fmt.Errorf("failed to connect to Whisper GPU server at '%s' after %d attempts: %w", whisperURL, maxRetries, err)
		}
	}

	return nil, fmt.Errorf("whisper transcription failed after %d attempts", maxRetries)
}

func BuildWhisperMultipartBody(audioPath, prompt, language string, pcmData []byte) (io.ReadCloser, string, error) {
	var audioSource io.Reader
	var closeSrc func() error

	if pcmData != nil {
		header := BuildWavHeader(len(pcmData))
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
		err := WriteWhisperMultipartFields(mw, audioSource, filepath.Base(audioPath), prompt, language)
		if closeErr := mw.Close(); err == nil {
			err = closeErr
		}
		_ = pw.CloseWithError(err)
	}()

	return pr, mw.FormDataContentType(), nil
}

func WriteWhisperMultipartFields(mw *multipart.Writer, audioSource io.Reader, filename, prompt, language string) error {
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

func ReadLimitedBody(r io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes limit", maxBytes)
	}
	return body, nil
}

func ExecuteWhisperAttemptContext(ctx context.Context, client *http.Client, uri, contentType string, bodyReader io.ReadCloser, quiet, verbose bool) (*types.TranscriptionData, error) {
	defer bodyReader.Close()

	progressDone := make(chan struct{})
	defer close(progressDone)

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "POST", uri, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	if !quiet {
		go StartTranscriptionProgressTicker(startTime, progressDone)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !quiet && verbose {
		elapsed := time.Since(startTime)
		fmt.Printf("\rTranscription finished in %s!                                  \n", format.FormatClock(elapsed.Seconds()))
	}

	body, err := ReadLimitedBody(resp.Body, maxWhisperResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var data types.TranscriptionData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse transcription JSON: %w", err)
	}
	return &data, nil
}

func StartTranscriptionProgressTicker(startTime time.Time, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			fmt.Printf("\rTranscribing audio... Elapsed: %s   ", format.FormatClock(elapsed.Seconds()))
		}
	}
}

func SortSegments(segs []types.TranscriptionSegment) {
	sort.Slice(segs, func(i, j int) bool {
		return segs[i].Start < segs[j].Start
	})
}

func JoinSegmentText(segs []types.TranscriptionSegment) string {
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

func MergeSegments(segs []types.TranscriptionSegment) []types.TranscriptionSegment {
	if len(segs) == 0 {
		return segs
	}

	merged := make([]types.TranscriptionSegment, 0, len(segs))
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

func TrimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
