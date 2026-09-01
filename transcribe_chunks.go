package main

import (
	"fmt"
	"os"
	"path/filepath"
)

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
