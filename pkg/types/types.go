package types

import (
	"os"
)

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
	// Backend and Model record which engine produced this transcript, so a
	// saved transcript can be traced back to the backend that made it.
	Backend string `json:"whisper_backend,omitempty"`
	Model   string `json:"whisper_model,omitempty"`
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

type WhisperEngine string

const (
	WhisperEngineLocal  WhisperEngine = "local"
	WhisperEngineDocker WhisperEngine = "docker"
	WhisperEngineRemote WhisperEngine = "remote"
	WhisperEngineGemini WhisperEngine = "gemini"
)

type WhisperProfile struct {
	ID              int           `json:"id"`
	Name            string        `json:"name"`
	URL             string        `json:"url,omitempty"`
	SpeedFactor     float64       `json:"speed_factor"`
	DockerContainer string        `json:"docker_container,omitempty"`
	Language        string        `json:"language,omitempty"`
	Languages       []string      `json:"languages,omitempty"`
	Prompt          string        `json:"prompt,omitempty"`
	WakeCommand     string        `json:"wake_command,omitempty"`
	Engine          WhisperEngine `json:"engine"`
	Model           string        `json:"model,omitempty"`
	CliBinary       string        `json:"cli_binary,omitempty"`
	Processors      int           `json:"processors,omitempty"`
	Threads         int           `json:"threads,omitempty"`
	Greedy          bool          `json:"greedy,omitempty"`
}

type CostInfo struct {
	Type     string `json:"type"`
	In1M     float64
	Out1M    float64
	CostStr  string `json:"cost_str"`
	Est1HStr string `json:"est_1h_str"`
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

// RemoteOptions holds the remote-execution flags the command line acts on
// itself. The flags the engine acts on — Remote, Local, RemoteHost,
// RemoteFFmpegHost, NoCollect, Priority — live in ProcOptions.
type RemoteOptions struct {
	SetRemoteFFmpegHost bool
	RemoteSubcmd        string
	RemoteWorkDir       string
	Daemon              bool
	IfDirty             bool
	BatchWorkerDir      string
}

type PolicyOptions struct {
	AutoDownloadStr string
	DownloadPolicy  string
	DownloadK       int
	AutoCleanupStr  string
	CleanupDays     int
	AdRemovalMode   string
}

type BackendOptions struct {
	ABSURL        string
	ABSUser       string
	ABSPass       string
	SetABS        bool
	ABSToken      string
	SqliteDBPath  string
	ServerSubcmd  string
	ForceDelete   bool
	Refresh       bool
	DisableHourly bool
	OPMLSubcmd    string
	OPMLFile      string
	PodcastsOnly  bool
	EpisodesOnly  bool

	// FeedJobs caps how many podcast feeds are fetched concurrently when
	// checking for new episodes. Zero selects the default.
	FeedJobs int
}

type CLIOptions struct {
	ProcOptions
	RemoteOptions
	PolicyOptions
	BackendOptions

	ConfigCmd         string
	ConfigKey         string
	ConfigVal         string
	SetDefault        int
	PodcastsDir       string
	SetPodcastsDir    bool
	ListLLMs          bool
	CopyOpenCode      bool
	Debug             bool
	TestWhisper       bool
	TestABS           bool
	TestABSMap        bool
	TestABSDownload   bool
	TestKitty         bool
	ResetCache        bool
	AddWhisper        string
	RemoveWhisper     int
	SetDefaultWhisper int
	ListWhispers      bool

	CountGiven     bool
	Podcast        string
	Fill           bool
	DownloadAll    bool
	KeepCount      *int
	CheckNew       bool
	Oldest         bool
	NoWait         bool
	ProcessorCmd   string
	ProcessorValue string
	ConfigInfo     bool
	Args           []string
	ProcSubcmd     string
	ExportFormat   string
	ShowCuts       bool
	ShowTranscript bool
	LsSubcmd       string
	JSON           bool
	InfoSubcmd     string
	QueueSubcmd    string
	PlayerSubcmd   string
	SyncSubcmd     string
	StatusSubcmd   string
	Latest         bool
	ShowExamples   bool
}

type WhisperConfig struct {
	WhisperURL             string           `json:"whisper_url"`
	WhisperSpeedFactor     float64          `json:"whisper_speed_factor"`
	WhisperDockerContainer string           `json:"whisper_docker_container"`
	WhisperLanguage        string           `json:"whisper_language"`
	WhisperPrompt          string           `json:"whisper_prompt"`
	WhisperWakeCommand     string           `json:"whisper_wake_command,omitempty"`
	WhisperEngine          WhisperEngine    `json:"whisper_engine,omitempty"`
	WhisperModel           string           `json:"whisper_model,omitempty"`
	WhisperCliBinary       string           `json:"whisper_cli_binary,omitempty"`
	WhisperProcessors      int              `json:"whisper_processors,omitempty"`
	WhisperThreads         int              `json:"whisper_threads,omitempty"`
	WhisperGreedy          bool             `json:"whisper_greedy,omitempty"`
	ActiveWhisperID        int              `json:"active_whisper_id,omitempty"`
	WhisperProfiles        []WhisperProfile `json:"whisper_profiles,omitempty"`
}

type BackendConfig struct {
	AudiobookshelfURL    string `json:"audiobookshelf_url,omitempty"`
	AudiobookshelfUser   string `json:"audiobookshelf_user,omitempty"`
	AudiobookshelfPass   string `json:"audiobookshelf_pass,omitempty"`
	AudiobookshelfToken  string `json:"audiobookshelf_token,omitempty"`
	AudiobookshelfDBPath string `json:"audiobookshelf_sqlite_db_path,omitempty"`
	BackendType          string `json:"backend_type,omitempty"`
	PodfetchURL          string `json:"podfetch_url,omitempty"`
	PodfetchUser         string `json:"podfetch_user,omitempty"`
	PodfetchPass         string `json:"podfetch_pass,omitempty"`
	PodfetchAPIKey       string `json:"podfetch_api_key,omitempty"`
	PodfetchDBPath       string `json:"podfetch_db_path,omitempty"`
}

type RemoteConfig struct {
	RemoteFFmpegHost  string `json:"remote_ffmpeg_host,omitempty"`
	RemoteHost        string `json:"remote_host,omitempty"`
	DefaultProcessing string `json:"default_processing,omitempty"`
	RemoteWorkDir     string `json:"remote_work_dir,omitempty"`
}

type PolicyConfig struct {
	DefaultDownloadPolicy string `json:"default_download_policy,omitempty"`
	DefaultDownloadK      int    `json:"default_download_k,omitempty"`
	DefaultAdRemoval      string `json:"default_ad_policy,omitempty"`
}

type GeminiConfig struct {
	GeminiProjectID          string `json:"gemini_project_id,omitempty"`
	GeminiStagingBucket      string `json:"gemini_staging_bucket,omitempty"`
	GeminiLocation           string `json:"gemini_location,omitempty"`
	GeminiAPIKey             string `json:"gemini_api_key,omitempty"`
	GeminiModel              string `json:"gemini_model,omitempty"`
	GeminiAPIKeyEnabled      *bool  `json:"gemini_api_key_enabled,omitempty"`
	OpenRouterAPIKeyEnabled  *bool  `json:"openrouter_api_key_enabled,omitempty"`
	SpeculativeTranscription *bool  `json:"speculative_transcription,omitempty"`
}

type Config struct {
	Instructions     string          `json:"_instructions"`
	PodcastsDir      string          `json:"podcasts_dir"`
	ChunkDurationSec int             `json:"chunk_duration_sec"`
	ActiveProfileID  int             `json:"active_profile_id"`
	Profiles         []LLMProfile    `json:"profiles"`
	PostProcessors   []string        `json:"post_processors,omitempty"`
	TUIColor         *TUIColorConfig `json:"tui_color,omitempty"`

	WhisperConfig
	BackendConfig
	RemoteConfig
	PolicyConfig
	GeminiConfig
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

func (c *Config) GetGeminiModel() string {
	if c != nil && c.GeminiModel != "" {
		return c.GeminiModel
	}
	return "gemini-flash-latest"
}

func (c *Config) GetGeminiProjectID() string {
	if c != nil && c.GeminiProjectID != "" {
		return c.GeminiProjectID
	}
	return os.Getenv("GEMINI_PROJECT_ID")
}

func (c *Config) GetGeminiStagingBucket() string {
	if c != nil && c.GeminiStagingBucket != "" {
		s := c.GeminiStagingBucket
		if len(s) >= 5 && s[:5] == "gs://" {
			return s[5:]
		}
		return s
	}
	s := os.Getenv("GEMINI_STAGING_BUCKET")
	if len(s) >= 5 && s[:5] == "gs://" {
		return s[5:]
	}
	return s
}

func (c *Config) GetGeminiLocation() string {
	if c != nil && c.GeminiLocation != "" {
		return c.GeminiLocation
	}
	return "us-central1"
}
