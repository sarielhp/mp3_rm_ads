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

func buildWavHeader(dataSize int) []byte {
	channels := uint16(1)
	bitsPerSample := uint16(16)
	sampleRate := uint32(wavSampleRate)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	blockAlign := channels * bitsPerSample / 8
	riffSize := uint32(36 + dataSize)

	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	putUint32(header[4:8], riffSize)
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	putUint32(header[16:20], 16)
	putUint16(header[20:22], 1)
	putUint16(header[22:24], channels)
	putUint32(header[24:28], sampleRate)
	putUint32(header[28:32], byteRate)
	putUint16(header[32:34], blockAlign)
	putUint16(header[34:36], bitsPerSample)
	copy(header[36:40], []byte("data"))
	putUint32(header[40:44], uint32(dataSize))

	return header
}

func putUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func transcribeWhisper(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*TranscriptionData, error) {
	uri := whisperURL
	boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())

	var audioContent []byte

	if pcmData != nil {
		header := buildWavHeader(len(pcmData))
		audioContent = append(header, pcmData...)
	} else {
		var err error
		audioContent, err = os.ReadFile(audioPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read audio file: %w", err)
		}
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.SetBoundary(boundary)

	fw, err := w.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := fw.Write(audioContent); err != nil {
		return nil, fmt.Errorf("failed to write audio content: %w", err)
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

	maxRetries := 5
	retryDelay := 5

	readTimeout := int(totalDuration*1.5) + 600
	if readTimeout < 1800 {
		readTimeout = 1800
	}

	var progressDone chan struct{}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if progressDone != nil {
			close(progressDone)
		}
		progressDone = make(chan struct{})

		startTime := time.Now()

		client := &http.Client{
			Timeout: time.Duration(readTimeout) * time.Second,
		}

		req, err := http.NewRequest("POST", uri, bytes.NewReader(buf.Bytes()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		if !quiet {
			go func(done chan struct{}) {
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
			}(progressDone)
		}

		resp, err := client.Do(req)
		if err != nil {
			if progressDone != nil {
				close(progressDone)
				progressDone = nil
			}
			if attempt < maxRetries {
				if !quiet {
					fmt.Printf("\nWhisper server connection error (attempt %d/%d): %v\n", attempt, maxRetries, err)
					fmt.Printf("   Retrying in %d seconds...\n", retryDelay)
				}
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to connect to Whisper GPU server at '%s' after %d attempts: %w", whisperURL, maxRetries, err)
		}

		if !quiet && verbose {
			elapsed := time.Since(startTime)
			fmt.Printf("\rTranscription finished in %s!                                  \n", formatClock(elapsed.Seconds()))
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				if progressDone != nil {
					close(progressDone)
				}
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
			var data TranscriptionData
			if err := json.Unmarshal(body, &data); err != nil {
				if progressDone != nil {
					close(progressDone)
				}
				return nil, fmt.Errorf("failed to parse transcription JSON: %w", err)
			}
			if progressDone != nil {
				close(progressDone)
			}
			return &data, nil
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !quiet {
			fmt.Printf("\nWhisper server returned status %d: %s\n", resp.StatusCode, string(body))
		}
		if progressDone != nil {
			close(progressDone)
			progressDone = nil
		}
		if attempt < maxRetries {
			if !quiet {
				fmt.Printf("   Retrying in %d seconds (attempt %d/%d)...\n", retryDelay, attempt, maxRetries)
			}
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}
	}

	if progressDone != nil {
		close(progressDone)
	}
	return nil, fmt.Errorf("whisper transcription failed after %d attempts", maxRetries)
}

func transcribeChunks(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, chunkDuration int, dockerContainer string, prompt, language string) (*TranscriptionData, error) {
	overlap := 30.0
	maxChunk := float64(chunkDuration)
	if maxChunk > 1200.0 {
		maxChunk = 1200.0
	}
	numChunks := int(totalDuration / maxChunk)
	if numChunks < 1 {
		numChunks = 1
	}
	if totalDuration/float64(numChunks) > maxChunk {
		numChunks++
	}

	if !quiet {
		fmt.Printf("   Converting to WAV and splitting %s audio into %d chunks of %s...\n",
			formatTime(totalDuration), numChunks, formatTime(maxChunk))
	}

	workDir := workDirFor(audioPath)
	os.MkdirAll(workDir, 0755)
	wavPath := filepath.Join(workDir, filepath.Base(audioPath)+".wav")
	verifyTempFile(wavPath)

	if !convertToWAV(audioPath, wavPath) {
		return nil, fmt.Errorf("failed to convert audio to WAV")
	}

	wavInfo, _ := os.Stat(wavPath)
	wavSize := wavInfo.Size()
	pcmSize := wavSize - 44

	if numChunks <= 1 {
		pcmData := make([]byte, pcmSize)
		f, _ := os.Open(wavPath)
		f.ReadAt(pcmData, 44)
		f.Close()
		os.Remove(wavPath)
		os.RemoveAll(workDir)
		return transcribeWhisper(audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, dockerContainer, prompt, language, pcmData)
	}

	type chunkInfo struct {
		index        int
		actualStart  float64
		actualEnd    float64
		extractStart float64
		extractEnd   float64
		startByte    int64
		dataSize     int
	}

	chunks := make([]chunkInfo, numChunks)
	for i := 0; i < numChunks; i++ {
		actualStart := float64(i) * maxChunk
		actualEnd := float64(i+1) * maxChunk
		if actualEnd > totalDuration {
			actualEnd = totalDuration
		}
		extractStart := actualStart - overlap
		if extractStart < 0 {
			extractStart = 0
		}
		extractEnd := actualEnd + overlap
		if extractEnd > totalDuration {
			extractEnd = totalDuration
		}
		startByte := int64(extractStart * float64(wavBytesPerSec))
		dataSize := int((extractEnd - extractStart) * float64(wavBytesPerSec))
		if startByte+int64(dataSize) > pcmSize {
			dataSize = int(pcmSize - startByte)
		}
		if dataSize < 0 {
			dataSize = 0
		}

		chunks[i] = chunkInfo{
			index:        i,
			actualStart:  actualStart,
			actualEnd:    actualEnd,
			extractStart: extractStart,
			extractEnd:   extractEnd,
			startByte:    startByte,
			dataSize:     dataSize,
		}
	}

	type chunkResult struct {
		data  *TranscriptionData
		chunk chunkInfo
	}

	var allResults []chunkResult

	for _, ch := range chunks {
		if !quiet {
			chunkLen := ch.actualEnd - ch.actualStart
			fmt.Printf("\nWorking on chunk %d/%d: %s -> %s (%s)\n",
				ch.index+1, numChunks,
				formatTime(ch.actualStart), formatTime(ch.actualEnd), formatTime(chunkLen))
		}

		chunkPath := filepath.Join(workDir, fmt.Sprintf("chunk_%04d.wav", ch.index))
		pcmData := make([]byte, ch.dataSize)
		f, err := os.Open(wavPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open WAV for chunk %d: %w", ch.index, err)
		}
		_, err = f.ReadAt(pcmData, 44+ch.startByte)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read PCM data for chunk %d: %w", ch.index, err)
		}

		header := buildWavHeader(len(pcmData))
		os.WriteFile(chunkPath, append(header, pcmData...), 0644)

		if !validateWavFile(chunkPath) {
			return nil, fmt.Errorf("chunk %d failed WAV validation", ch.index+1)
		}

		chunkData, err := transcribeWhisper(
			chunkPath, whisperURL, quiet, verbose,
			ch.actualEnd-ch.actualStart, speedFactor,
			dockerContainer, prompt, language, nil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to transcribe chunk %d: %w", ch.index+1, err)
		}

		os.Remove(chunkPath)

		isFirst := ch.index == 0
		isLast := ch.index == numChunks-1
		cutStart := ch.actualStart
		cutEnd := ch.actualEnd

		for _, seg := range chunkData.Segments {
			segStart := seg.Start + ch.extractStart
			segEnd := seg.End + ch.extractStart

			if isFirst {
				if segEnd <= cutStart {
					continue
				}
				if segStart < cutStart {
					seg.Start = cutStart
				} else {
					seg.Start = segStart
				}
				if segEnd > cutEnd {
					seg.End = cutEnd
				} else {
					seg.End = segEnd
				}
			} else if isLast {
				if segStart >= cutEnd {
					continue
				}
				if segStart < cutStart {
					seg.Start = cutStart
				} else {
					seg.Start = segStart
				}
				if segEnd > cutEnd {
					seg.End = cutEnd
				} else {
					seg.End = segEnd
				}
			} else {
				if segStart >= cutEnd {
					continue
				}
				if segStart < cutStart {
					seg.Start = cutStart
				} else {
					seg.Start = segStart
				}
				if segEnd > cutEnd {
					seg.End = cutEnd
				} else {
					seg.End = segEnd
				}
			}

			for i := range seg.Words {
				seg.Words[i].Start += ch.extractStart
				seg.Words[i].End += ch.extractStart
			}

			allResults = append(allResults, chunkResult{data: chunkData, chunk: ch})
		}
	}

	os.Remove(wavPath)
	os.RemoveAll(workDir)

	allSegments := make([]TranscriptionSegment, 0, len(allResults))
	for _, r := range allResults {
		if r.data != nil {
			allSegments = append(allSegments, r.data.Segments...)
		}
	}

	sortSegments(allSegments)

	mergedSegments := mergeSegments(allSegments)

	fullText := ""
	for _, seg := range mergedSegments {
		fullText += seg.Text + " "
	}
	fullText = trimSpace(fullText)

	lang := "he"
	if len(allSegments) > 0 && allSegments[0].Language != "" {
		lang = allSegments[0].Language
	}

	return &TranscriptionData{
		Text:     fullText,
		Segments: mergedSegments,
		Language: lang,
	}, nil
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
