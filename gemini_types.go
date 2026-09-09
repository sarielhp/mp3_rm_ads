package main

import "github.com/sariel/abs/pkg/types"

const geminiAdRemovalPrompt = types.GeminiAdRemovalPrompt

type geminiCutItem = types.GeminiCutItem
type geminiSegmentItem = types.GeminiSegmentItem
type geminiResponsePayload = types.GeminiResponsePayload

type geminiChunkInfo struct {
	index    int
	startSec float64
	durSec   float64
	filePath string
}

type geminiChunkResult struct {
	index    int
	startSec float64
	payload  *geminiResponsePayload
}
