package main

import (
	"path/filepath"
	"regexp"

	"github.com/sariel/abs/pkg/types"
)

const workDirName = ".work"

func workDirFor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return filepath.Join(filepath.Dir(abs), workDirName, filepath.Base(abs))
}

func verifyTempFile(filePath string) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		fatalError("ERROR: Temp file '%s' is not in .work/ directory. Aborting.\n", filePath)
	}
	matched, _ := regexp.MatchString(`/\.work/`, abs)
	if !matched {
		fatalError("ERROR: Temp file '%s' is not in .work/ directory. Aborting.\n", filePath)
	}
}

type AdSegment = types.AdSegment
type KeepSegment = types.KeepSegment
type TranscriptionSegment = types.TranscriptionSegment
type TranscriptionWord = types.TranscriptionWord
type TranscriptionData = types.TranscriptionData
type CutEntry = types.CutEntry
type MergedCutInterval = types.MergedCutInterval
type CutsData = types.CutsData
type CutsResult = types.CutsResult
type LLMProfile = types.LLMProfile
type WhisperEngine = types.WhisperEngine

const (
	WhisperEngineLocal  = types.WhisperEngineLocal
	WhisperEngineDocker = types.WhisperEngineDocker
	WhisperEngineRemote = types.WhisperEngineRemote
	WhisperEngineGemini = types.WhisperEngineGemini
)

type WhisperProfile = types.WhisperProfile
type TUIColorConfig = types.TUIColorConfig
type CLIOptions = types.CLIOptions
type CostInfo = types.CostInfo

type Config struct {
	Instructions             string           `json:"_instructions"`
	PodcastsDir              string           `json:"podcasts_dir"`
	WhisperURL               string           `json:"whisper_url"`
	WhisperSpeedFactor       float64          `json:"whisper_speed_factor"`
	WhisperDockerContainer   string           `json:"whisper_docker_container"`
	WhisperLanguage          string           `json:"whisper_language"`
	WhisperPrompt            string           `json:"whisper_prompt"`
	WhisperWakeCommand       string           `json:"whisper_wake_command,omitempty"`
	WhisperEngine            WhisperEngine    `json:"whisper_engine,omitempty"`
	WhisperModel             string           `json:"whisper_model,omitempty"`
	WhisperCliBinary         string           `json:"whisper_cli_binary,omitempty"`
	WhisperProcessors        int              `json:"whisper_processors,omitempty"`
	WhisperThreads           int              `json:"whisper_threads,omitempty"`
	WhisperGreedy            bool             `json:"whisper_greedy,omitempty"`
	ChunkDurationSec         int              `json:"chunk_duration_sec"`
	ActiveProfileID          int              `json:"active_profile_id"`
	Profiles                 []LLMProfile     `json:"profiles"`
	ActiveWhisperID          int              `json:"active_whisper_id,omitempty"`
	WhisperProfiles          []WhisperProfile `json:"whisper_profiles,omitempty"`
	AudiobookshelfURL        string           `json:"audiobookshelf_url,omitempty"`
	AudiobookshelfUser       string           `json:"audiobookshelf_user,omitempty"`
	AudiobookshelfPass       string           `json:"audiobookshelf_pass,omitempty"`
	AudiobookshelfToken      string           `json:"audiobookshelf_token,omitempty"`
	AudiobookshelfDBPath     string           `json:"audiobookshelf_sqlite_db_path,omitempty"`
	BackendType              string           `json:"backend_type,omitempty"`
	PodfetchURL              string           `json:"podfetch_url,omitempty"`
	PodfetchUser             string           `json:"podfetch_user,omitempty"`
	PodfetchPass             string           `json:"podfetch_pass,omitempty"`
	PodfetchAPIKey           string           `json:"podfetch_api_key,omitempty"`
	PodfetchDBPath           string           `json:"podfetch_db_path,omitempty"`
	PostProcessors           []string         `json:"post_processors,omitempty"`
	RemoteFFmpegHost         string           `json:"remote_ffmpeg_host,omitempty"`
	RemoteHost               string           `json:"remote_host,omitempty"`
	DefaultProcessing        string           `json:"default_processing,omitempty"`
	RemoteWorkDir            string           `json:"remote_work_dir,omitempty"`
	DefaultDownloadPolicy    string           `json:"default_download_policy,omitempty"`
	DefaultDownloadK         int              `json:"default_download_k,omitempty"`
	DefaultAdRemoval         string           `json:"default_ad_policy,omitempty"`
	GeminiProjectID          string           `json:"gemini_project_id,omitempty"`
	GeminiStagingBucket      string           `json:"gemini_staging_bucket,omitempty"`
	GeminiLocation           string           `json:"gemini_location,omitempty"`
	GeminiAPIKey             string           `json:"gemini_api_key,omitempty"`
	GeminiModel              string           `json:"gemini_model,omitempty"`
	GeminiAPIKeyEnabled      *bool            `json:"gemini_api_key_enabled,omitempty"`
	OpenRouterAPIKeyEnabled  *bool            `json:"openrouter_api_key_enabled,omitempty"`
	SpeculativeTranscription *bool            `json:"speculative_transcription,omitempty"`
	TUIColor                 *TUIColorConfig  `json:"tui_color,omitempty"`
}

func (c *Config) IsGeminiAPIKeyEnabled() bool {
	if c != nil && c.GeminiAPIKeyEnabled != nil {
		return *c.GeminiAPIKeyEnabled
	}
	return true
}

func (c *Config) IsOpenRouterAPIKeyEnabled() bool {
	if c != nil && c.OpenRouterAPIKeyEnabled != nil {
		return *c.OpenRouterAPIKeyEnabled
	}
	return false
}

func (c *Config) IsSpeculativeTranscriptionEnabled() bool {
	if c != nil && c.SpeculativeTranscription != nil {
		return *c.SpeculativeTranscription
	}
	return true
}
