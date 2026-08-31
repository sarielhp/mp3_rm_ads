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

type WhisperProfile struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	URL             string  `json:"url"`
	SpeedFactor     float64 `json:"speed_factor"`
	DockerContainer string  `json:"docker_container,omitempty"`
	Language        string  `json:"language,omitempty"`
	Prompt          string  `json:"prompt,omitempty"`
	WakeCommand     string  `json:"wake_command,omitempty"`
}

type Config struct {
	Instructions           string           `json:"_instructions"`
	PodcastsDir            string           `json:"podcasts_dir"`
	WhisperURL             string           `json:"whisper_url"`
	WhisperSpeedFactor     float64          `json:"whisper_speed_factor"`
	WhisperDockerContainer string           `json:"whisper_docker_container"`
	WhisperLanguage        string           `json:"whisper_language"`
	WhisperPrompt          string           `json:"whisper_prompt"`
	WhisperWakeCommand     string           `json:"whisper_wake_command,omitempty"`
	ChunkDurationSec       int              `json:"chunk_duration_sec"`
	ActiveProfileID        int              `json:"active_profile_id"`
	Profiles               []LLMProfile     `json:"profiles"`
	ActiveWhisperID        int              `json:"active_whisper_id,omitempty"`
	WhisperProfiles        []WhisperProfile `json:"whisper_profiles,omitempty"`
	AudiobookshelfURL      string           `json:"audiobookshelf_url,omitempty"`
	AudiobookshelfUser     string           `json:"audiobookshelf_user,omitempty"`
	AudiobookshelfPass     string           `json:"audiobookshelf_pass,omitempty"`
	AudiobookshelfToken    string           `json:"audiobookshelf_token,omitempty"`
	AudiobookshelfDBPath   string           `json:"audiobookshelf_sqlite_db_path,omitempty"`
	PostProcessors         []string         `json:"post_processors,omitempty"`
	RemoteFFmpegHost       string           `json:"remote_ffmpeg_host,omitempty"`
	RemoteHost             string           `json:"remote_host,omitempty"`
	DefaultProcessing      string           `json:"default_processing,omitempty"`
	RemoteWorkDir          string           `json:"remote_work_dir,omitempty"`
	DefaultDownloadPolicy  string           `json:"default_download_policy,omitempty"`
	DefaultDownloadK       int              `json:"default_download_k,omitempty"`
	DefaultAdRemoval       string           `json:"default_ad_policy,omitempty"`
	TUIColor               *TUIColorConfig  `json:"tui_color,omitempty"`
}

type TUIColorConfig struct {
	Cyan     string `json:"cyan,omitempty"`
	Purple   string `json:"purple,omitempty"`
	Magenta  string `json:"magenta,omitempty"`
	Pink     string `json:"pink,omitempty"`
	Yellow   string `json:"yellow,omitempty"`
	Green    string `json:"green,omitempty"`
	Red      string `json:"red,omitempty"`
	Blue     string `json:"blue,omitempty"`
	Lavender string `json:"lavender,omitempty"`
	DarkBg   string `json:"dark_bg,omitempty"`
	CardBg   string `json:"card_bg,omitempty"`
	Border   string `json:"border,omitempty"`
	Subtext  string `json:"subtext,omitempty"`
	Dim      string `json:"dim,omitempty"`
}

type CLIOptions struct {
	Output               string
	TranscriptPath       string
	SaveTranscript       bool
	ExportSRT            bool
	ExportTXT            bool
	ExportFormat         string
	Recut                bool
	Force                string
	ForceLLM             bool
	ForceTranscribe      bool
	UseLLM               string
	ConfigCmd            string
	ConfigKey            string
	ConfigVal            string
	SetDefault           int
	PodcastsDir          string
	SetPodcastsDir       bool
	IsConfigCommand      bool
	IsDirCommand         bool
	IsFileCommand        bool
	IsTUICommand         bool
	IsTimelineCommand    bool
	IsTestCommand        bool
	IsScanCommand        bool
	IsStatusCommand      bool
	IsRemoteCommand      bool
	IsBatchWorkerCommand bool
	ListLLMs             bool
	CopyOpenCode         bool
	Quiet                bool
	Verbose              bool
	Debug                bool
	UseChunks            bool
	TranscribeMin        string
	ExtractKeywords      bool
	TestWhisper          bool
	TestABS              bool
	TestABSMap           bool
	TestABSDownload      bool
	TestKitty            bool
	ABSURL               string
	ABSUser              string
	ABSPass              string
	SetABS               bool
	IsCacheCommand       bool
	ResetCache           bool
	AddWhisper           string
	RemoveWhisper        int
	SetDefaultWhisper    int
	ListWhispers         bool

	Count               int
	CountGiven          bool
	Podcast             string
	Fill                bool
	DownloadAll         bool
	KeepCount           *int
	CheckNew            bool
	Oldest              bool
	NoWait              bool
	SqliteDBPath        string
	ProcessorCmd        string
	ProcessorValue      string
	DryRun              bool
	ConfigInfo          bool
	ABSToken            string
	Args                []string
	ServerSubcmd        string
	OPMLSubcmd          string
	OPMLFile            string
	PodcastsOnly        bool
	EpisodesOnly        bool
	RemoteFFmpegHost    string
	SetRemoteFFmpegHost bool
	RemoteSubcmd        string
	RemoteHost          string
	Remote              bool
	Local               bool
	BatchWorkerDir      string
}

type CostInfo struct {
	Type     string `json:"type"`
	In1M     float64
	Out1M    float64
	CostStr  string `json:"cost_str"`
	Est1HStr string `json:"est_1h_str"`
}
