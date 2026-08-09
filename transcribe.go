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
	putUint16(header[20:22], 1) // PCM
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

func transcribeWhisper(audioPath, whisperURL string, quiet bool, totalDuration, speedFactor float64, dockerContainer string, prompt, language string, pcmData []byte) (*TranscriptionData, error) {
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
	w.WriteField("language", language)

	if prompt != "" {
		w.WriteField("prompt", prompt)
	}

	w.Close()

	maxRetries := 5
	retryDelay := 5

	estTranscribeSeconds := 900.0
	if totalDuration > 0 && speedFactor > 0 {
		estTranscribeSeconds = totalDuration / speedFactor
	}
	readTimeout := int(estTranscribeSeconds * 1.5)
	if readTimeout < 600 {
		readTimeout = 600
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		startTime := time.Now()

		client := &http.Client{
			Timeout: time.Duration(readTimeout) * time.Second,
		}

		req, err := http.NewRequest("POST", uri, &buf)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		if !quiet {
			go func() {
				for {
					elapsed := time.Since(startTime)

					dockerProgress := pollWhisperDockerProgress(dockerContainer)
					if dp, ok := dockerProgress.(*failedToDecodeType); ok && dp == failedToDecodeSentinel {
						fmt.Print("\rWhisper failed to decode audio (file too long or corrupted).\n")
						return
					} else if pct, ok := dockerProgress.(float64); ok && pct <= 1.0 {
						remaining := (elapsed.Seconds() / pct) - elapsed.Seconds()
						if remaining < 0 {
							remaining = 0
						}
						remStr := formatClock(remaining)
						fmt.Printf("\rTranscribing audio... %5.1f%% | Elapsed: %s | Est. remaining: %s   ", pct*100, formatClock(elapsed.Seconds()), remStr)
					} else if pos, ok := dockerProgress.(float64); ok && pos > 0 && totalDuration > 0 {
						pct := pos / totalDuration
						if pct > 0.99 {
							pct = 0.99
						}
						remaining := (elapsed.Seconds() / pct) - elapsed.Seconds()
						if remaining < 0 {
							remaining = 0
						}
						fmt.Printf("\rTranscribing audio... %5.1f%% | Elapsed: %s | Est. remaining: %s   ", pct*100, formatClock(elapsed.Seconds()), formatClock(remaining))
					} else if estTranscribeSeconds > 0 {
						remaining := estTranscribeSeconds - elapsed.Seconds()
						if remaining < 0 {
							remaining = 0
						}
						fmt.Printf("\rTranscribing audio... Elapsed: %s | Est. remaining: %s   ", formatClock(elapsed.Seconds()), formatClock(remaining))
					} else {
						fmt.Printf("\rTranscribing audio... Elapsed: %s   ", formatClock(elapsed.Seconds()))
					}

					time.Sleep(2 * time.Second)
				}
			}()
		}

		resp, err := client.Do(req)
		if err != nil {
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
		defer resp.Body.Close()

		if !quiet {
			elapsed := time.Since(startTime)
			fmt.Printf("\rTranscription finished in %s!                                  \n", formatClock(elapsed.Seconds()))
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
			var data TranscriptionData
			if err := json.Unmarshal(body, &data); err != nil {
				return nil, fmt.Errorf("failed to parse transcription JSON: %w", err)
			}
			return &data, nil
		}

		body, _ := io.ReadAll(resp.Body)
		if !quiet {
			fmt.Printf("\nWhisper server returned status %d: %s\n", resp.StatusCode, string(body))
		}
		if attempt < maxRetries {
			if !quiet {
				fmt.Printf("   Retrying in %d seconds (attempt %d/%d)...\n", retryDelay, attempt, maxRetries)
			}
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}
	}

	return nil, fmt.Errorf("whisper transcription failed after %d attempts", maxRetries)
}

func transcribeChunks(audioPath, whisperURL string, quiet bool, totalDuration, speedFactor float64, chunkDuration int, parallel int, dockerContainer string, prompt, language string) (*TranscriptionData, error) {
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
		return transcribeWhisper(audioPath, whisperURL, quiet, totalDuration, speedFactor, dockerContainer, prompt, language, pcmData)
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
	var mu syncMu
	workers := parallel
	if workers > len(chunks) {
		workers = len(chunks)
	}

	for i := 0; i < len(chunks); i += workers {
		end := i + workers
		if end > len(chunks) {
			end = len(chunks)
		}

		type batchResult struct {
			data  *TranscriptionData
			chunk chunkInfo
		}
		var batchResults []batchResult
		var batchMu syncMu

		var wg syncWG
		for _, ch := range chunks[i:end] {
			ch := ch
			wg.Add(1)
			go func() {
				defer wg.Done()

				if !quiet {
					mu.Lock()
					chunkLen := ch.actualEnd - ch.actualStart
					fmt.Printf("\nWorking on chunk %d/%d: %s -> %s (%s)\n",
						ch.index+1, numChunks,
						formatTime(ch.actualStart), formatTime(ch.actualEnd), formatTime(chunkLen))
					mu.Unlock()
				}

				chunkPath := filepath.Join(workDir, fmt.Sprintf("chunk_%04d.wav", ch.index))
				pcmData := make([]byte, ch.dataSize)
				f, err := os.Open(wavPath)
				if err != nil {
					return
				}
				_, err = f.ReadAt(pcmData, 44+ch.startByte)
				f.Close()
				if err != nil {
					return
				}

				header := buildWavHeader(len(pcmData))
				os.WriteFile(chunkPath, append(header, pcmData...), 0644)

				if !validateWavFile(chunkPath) {
					mu.Lock()
					fmt.Printf("Chunk %d failed WAV validation\n", ch.index+1)
					mu.Unlock()
					return
				}

				chunkData, err := transcribeWhisper(
					chunkPath, whisperURL, quiet,
					ch.actualEnd-ch.actualStart, speedFactor,
					dockerContainer, prompt, language, nil,
				)
				if err != nil {
					return
				}

				os.Remove(chunkPath)

				batchMu.Lock()
				batchResults = append(batchResults, batchResult{data: chunkData, chunk: ch})
				batchMu.Unlock()
			}()
		}
		wg.Wait()

		for _, br := range batchResults {
			chunkData := br.data
			ch := br.chunk
			if chunkData == nil {
				continue
			}

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
					mid := cutEnd
					if segStart >= mid {
						continue
					}
					if segStart < cutStart {
						seg.Start = cutStart
					} else {
						seg.Start = segStart
					}
					if segEnd > mid {
						seg.End = mid
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
				last.Text = seg.Text
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

type syncMu struct {
	mu syncMutex
}

func (m *syncMu) Lock() {
	m.mu.Lock()
}

func (m *syncMu) Unlock() {
	m.mu.Unlock()
}

type syncMutex struct {
	ch chan struct{}
}

func (m *syncMutex) Lock() {
	if m.ch == nil {
		m.ch = make(chan struct{}, 1)
	}
	m.ch <- struct{}{}
}

func (m *syncMutex) Unlock() {
	<-m.ch
}

type syncWG struct {
	ch chan struct{}
}

func (wg *syncWG) Add(n int) {
	if wg.ch == nil {
		wg.ch = make(chan struct{}, 1)
	}
}

func (wg *syncWG) Done() {
	wg.ch <- struct{}{}
}

func (wg *syncWG) Wait() {
	if wg.ch == nil {
		return
	}
	<-wg.ch
}
