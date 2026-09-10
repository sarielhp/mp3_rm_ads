package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sariel/abs/pkg/audio"
	"github.com/sariel/abs/pkg/format"
	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

type ChunkInfo struct {
	Index        int
	ActualStart  float64
	ActualEnd    float64
	ExtractStart float64
	ExtractEnd   float64
	StartByte    int64
	DataSize     int
}

func TranscribeChunks(audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, chunkDuration int, dockerContainer string, prompt, language string) (*types.TranscriptionData, error) {
	return TranscribeChunksContext(context.Background(), audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, chunkDuration, dockerContainer, prompt, language)
}

func TranscribeChunksContext(ctx context.Context, audioPath, whisperURL string, quiet, verbose bool, totalDuration, speedFactor float64, chunkDuration int, dockerContainer string, prompt, language string) (*types.TranscriptionData, error) {
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
			format.FormatTime(totalDuration), numChunks, format.FormatTime(maxChunk))
	}

	workDir := util.WorkDirFor(audioPath)
	os.MkdirAll(workDir, 0755)
	wavPath := filepath.Join(workDir, filepath.Base(audioPath)+".wav")
	if err := util.VerifyTempFile(wavPath); err != nil {
		return nil, err
	}

	if !audio.ConvertToWAV(audioPath, wavPath) {
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
		return TranscribeWhisperContext(ctx, audioPath, whisperURL, quiet, verbose, totalDuration, speedFactor, dockerContainer, prompt, language, pcmData)
	}

	chunks := ComputeChunks(totalDuration, maxChunk, overlap, pcmSize, numChunks)
	var allSegments []types.TranscriptionSegment

	for _, ch := range chunks {
		if err := ctx.Err(); err != nil {
			os.Remove(wavPath)
			os.RemoveAll(workDir)
			return nil, err
		}
		segs, err := ProcessSingleChunkContext(ctx, wavPath, workDir, ch, numChunks, whisperURL, quiet, verbose, speedFactor, dockerContainer, prompt, language)
		if err != nil {
			os.Remove(wavPath)
			os.RemoveAll(workDir)
			return nil, err
		}
		allSegments = append(allSegments, segs...)
	}

	os.Remove(wavPath)
	os.RemoveAll(workDir)
	return AssembleTranscriptionResult(allSegments), nil
}

func ComputeChunks(totalDuration, maxChunk, overlap float64, pcmSize int64, numChunks int) []ChunkInfo {
	chunks := make([]ChunkInfo, numChunks)
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
		startByte := int64(extractStart * float64(WavBytesPerSec))
		dataSize := int((extractEnd - extractStart) * float64(WavBytesPerSec))
		if startByte+int64(dataSize) > pcmSize {
			dataSize = int(pcmSize - startByte)
		}
		if dataSize < 0 {
			dataSize = 0
		}

		chunks[i] = ChunkInfo{
			Index:        i,
			ActualStart:  actualStart,
			ActualEnd:    actualEnd,
			ExtractStart: extractStart,
			ExtractEnd:   extractEnd,
			StartByte:    startByte,
			DataSize:     dataSize,
		}
	}
	return chunks
}

func ProcessSingleChunk(wavPath, workDir string, ch ChunkInfo, numChunks int, whisperURL string, quiet, verbose bool, speedFactor float64, dockerContainer, prompt, language string) ([]types.TranscriptionSegment, error) {
	return ProcessSingleChunkContext(context.Background(), wavPath, workDir, ch, numChunks, whisperURL, quiet, verbose, speedFactor, dockerContainer, prompt, language)
}

func ProcessSingleChunkContext(ctx context.Context, wavPath, workDir string, ch ChunkInfo, numChunks int, whisperURL string, quiet, verbose bool, speedFactor float64, dockerContainer, prompt, language string) ([]types.TranscriptionSegment, error) {
	if !quiet {
		chunkLen := ch.ActualEnd - ch.ActualStart
		fmt.Printf("\nWorking on chunk %d/%d: %s -> %s (%s)\n",
			ch.Index+1, numChunks,
			format.FormatTime(ch.ActualStart), format.FormatTime(ch.ActualEnd), format.FormatTime(chunkLen))
	}

	chunkPath := filepath.Join(workDir, fmt.Sprintf("chunk_%04d.wav", ch.Index))
	pcmData := make([]byte, ch.DataSize)
	f, err := os.Open(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV for chunk %d: %w", ch.Index, err)
	}
	_, err = f.ReadAt(pcmData, 44+ch.StartByte)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM data for chunk %d: %w", ch.Index, err)
	}

	header := BuildWavHeader(len(pcmData))
	os.WriteFile(chunkPath, append(header, pcmData...), 0644)
	if !audio.ValidateWavFile(chunkPath) {
		return nil, fmt.Errorf("chunk %d failed WAV validation", ch.Index+1)
	}

	chunkData, err := TranscribeWhisperContext(
		ctx,
		chunkPath, whisperURL, quiet, verbose,
		ch.ActualEnd-ch.ActualStart, speedFactor,
		dockerContainer, prompt, language, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to transcribe chunk %d: %w", ch.Index+1, err)
	}
	os.Remove(chunkPath)

	isFirst := ch.Index == 0
	isLast := ch.Index == numChunks-1
	var adjusted []types.TranscriptionSegment
	for _, seg := range chunkData.Segments {
		if adj, ok := AdjustChunkSegment(seg, ch, isFirst, isLast); ok {
			adjusted = append(adjusted, adj)
		}
	}
	return adjusted, nil
}

func AdjustChunkSegment(seg types.TranscriptionSegment, ch ChunkInfo, isFirst, isLast bool) (types.TranscriptionSegment, bool) {
	cutStart := ch.ActualStart
	cutEnd := ch.ActualEnd
	segStart := seg.Start + ch.ExtractStart
	segEnd := seg.End + ch.ExtractStart

	if segEnd <= cutStart || segStart >= cutEnd {
		return seg, false
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

	if seg.End <= seg.Start {
		return seg, false
	}

	for i := range seg.Words {
		seg.Words[i].Start += ch.ExtractStart
		seg.Words[i].End += ch.ExtractStart
	}
	return seg, true
}

func AssembleTranscriptionResult(allSegments []types.TranscriptionSegment) *types.TranscriptionData {
	SortSegments(allSegments)
	mergedSegments := MergeSegments(allSegments)

	fullText := JoinSegmentText(mergedSegments)

	lang := "he"
	if len(allSegments) > 0 && allSegments[0].Language != "" {
		lang = allSegments[0].Language
	}

	return &types.TranscriptionData{
		Text:     fullText,
		Segments: mergedSegments,
		Language: lang,
	}
}
