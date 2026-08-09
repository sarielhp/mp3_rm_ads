package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const workDirName = ".work"

func workDirFor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return filepath.Join(filepath.Dir(abs), workDirName)
}

func verifyTempFile(filePath string) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Temp file '%s' is not in .work/ directory. Aborting.\n", filePath)
		os.Exit(1)
	}
	matched, _ := regexp.MatchString(`/\.work/`, abs)
	if !matched {
		fmt.Fprintf(os.Stderr, "ERROR: Temp file '%s' is not in .work/ directory. Aborting.\n", filePath)
		os.Exit(1)
	}
}

type AdSegment struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Reason string  `json:"reason,omitempty"`
}

type KeepSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type TranscriptionSegment struct {
	Start    float64             `json:"start"`
	End      float64             `json:"end"`
	Text     string              `json:"text"`
	Language string              `json:"language,omitempty"`
	Words    []TranscriptionWord `json:"words,omitempty"`
}

type TranscriptionWord struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Word  string  `json:"word"`
}

type TranscriptionData struct {
	Text     string                 `json:"text"`
	Segments []TranscriptionSegment `json:"segments"`
	Language string                 `json:"language,omitempty"`
}

type CutEntry struct {
	StartSec       float64 `json:"start_sec"`
	EndSec         float64 `json:"end_sec"`
	DurationSec    float64 `json:"duration_sec"`
	StartFormatted string  `json:"start_formatted"`
	EndFormatted   string  `json:"end_formatted"`
	Reason         string  `json:"reason,omitempty"`
}

type MergedCutInterval struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type CutsData struct {
	Version             int                 `json:"version"`
	Generator           string              `json:"generator"`
	LLMUsed             string              `json:"llm_used"`
	TargetFile          string              `json:"target_file"`
	OriginalDurationSec float64             `json:"original_duration_sec"`
	TotalCutDurationSec float64             `json:"total_cut_duration_sec"`
	CutIntervals        []CutEntry          `json:"cut_intervals"`
	MergedCutIntervals  []MergedCutInterval `json:"merged_cut_intervals"`
	KeepIntervals       []KeepSegment       `json:"keep_intervals"`
}

type CutsResult struct {
	CutsFile     string
	KeepSegments [][2]float64
	Changed      bool
}

type LLMProfile struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	Model  string `json:"model"`
	APIKey string `json:"api_key"`
}

type Config struct {
	Instructions           string       `json:"_instructions"`
	WhisperURL             string       `json:"whisper_url"`
	WhisperSpeedFactor     float64      `json:"whisper_speed_factor"`
	WhisperDockerContainer string       `json:"whisper_docker_container"`
	WhisperLanguage        string       `json:"whisper_language"`
	WhisperPrompt          string       `json:"whisper_prompt"`
	ChunkDurationSec       int          `json:"chunk_duration_sec"`
	ParallelChunks         int          `json:"parallel_chunks"`
	ActiveProfileID        int          `json:"active_profile_id"`
	Profiles               []LLMProfile `json:"profiles"`
}

type CLIOptions struct {
	Output          string
	TranscriptPath  string
	SaveTranscript  bool
	ExportSRT       bool
	ExportTXT       bool
	Recut           bool
	ForceLLM        bool
	ForceTranscribe bool
	UseLLM          string
	SetDefault      int
	ListLLMs        bool
	CopyOpenCode    bool
	Quiet           bool
	UseChunks       bool
	TranscribeMin   string
	ExtractKeywords bool
}

type CostInfo struct {
	Type     string `json:"type"`
	In1M     float64
	Out1M    float64
	CostStr  string `json:"cost_str"`
	Est1HStr string `json:"est_1h_str"`
}
