package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const configDirName = ".config/abs"
const legacyConfigDirName = ".config/mp3_rm_ads"
const configFileName = "config.json"
const opencodeConfigFile = ".config/opencode/opencode.json"

var defaultWhisperProfiles = []WhisperProfile{
	{
		ID:          1,
		Name:        "Local whisper-cli (tiny.en)",
		Engine:      WhisperEngineLocal,
		Model:       "tiny.en",
		SpeedFactor: 70.0,
		CliBinary:   "whisper-cli",
		Processors:  4,
		Threads:     4,
		Greedy:      true,
	},
	{
		ID:              2,
		Name:            "Docker Daemon (localhost:8088)",
		Engine:          WhisperEngineDocker,
		URL:             "http://127.0.0.1:8088/inference",
		SpeedFactor:     7.0,
		DockerContainer: "whisper",
	},
	{
		ID:          3,
		Name:        "Remote cloud8 (faster-whisper)",
		Engine:      WhisperEngineRemote,
		URL:         "http://cloud8:8000/v1/audio/transcriptions",
		SpeedFactor: 7.0,
		WakeCommand: "wake_cloud8",
	},
	{
		ID:          4,
		Name:        "Gemini 1.5 Flash (Google Cloud)",
		Engine:      WhisperEngineGemini,
		Model:       "gemini-1.5-flash",
		SpeedFactor: 60.0,
	},
}

var (
	defaultKeyEnabled  = true
	defaultKeyDisabled = false
)

var defaultConfig = Config{
	Instructions:            "Configuration file for abs. Select profiles by ID or set active_profile_id.",
	DefaultDownloadPolicy:   "latest",
	DefaultDownloadK:        3,
	DefaultAdRemoval:        "all",
	DefaultProcessing:       "local",
	RemoteWorkDir:           "~/abs_remote",
	WhisperURL:              "http://127.0.0.1:8088/inference",
	WhisperSpeedFactor:      70.0,
	ChunkDurationSec:        0,
	ActiveProfileID:         1,
	ActiveWhisperID:         1,
	WhisperProfiles:         defaultWhisperProfiles,
	WhisperEngine:           WhisperEngineLocal,
	WhisperModel:            "tiny.en",
	WhisperCliBinary:        "whisper-cli",
	WhisperProcessors:       4,
	WhisperThreads:          4,
	WhisperGreedy:           true,
	GeminiProjectID:         "vm-on-cloud-sariel",
	GeminiStagingBucket:     "abs-audio-staging-sariel",
	GeminiLocation:          "us-central1",
	GeminiAPIKeyEnabled:     &defaultKeyEnabled,
	OpenRouterAPIKeyEnabled: &defaultKeyDisabled,
	Profiles: []LLMProfile{
		{ID: 1, Name: "Ollama Local (llama3.1:8b)", Type: "ollama", URL: "http://192.168.1.230:11434/v1/chat/completions", Model: "llama3.1:8b"},
		{ID: 2, Name: "OpenRouter - Claude 3.5 Sonnet", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "anthropic/claude-3.5-sonnet"},
		{ID: 3, Name: "OpenRouter - DeepSeek V4 Flash", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "deepseek/deepseek-v4-flash"},
		{ID: 4, Name: "OpenRouter - Gemini 2.5 Flash", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "google/gemini-2.5-flash"},
	},
	RemoteFFmpegHost: "cloud8",
}

func ensureConfigExists() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	if _, err := os.Stat(configPath()); os.IsNotExist(err) {
		cfg := defaultConfig
		ip := localIP()
		cfg.WhisperURL = fmt.Sprintf("http://%s:8088/inference", ip)
		for i := range cfg.Profiles {
			cfg.Profiles[i].URL = replaceIP(cfg.Profiles[i].URL, ip)
		}
		for i := range cfg.WhisperProfiles {
			if cfg.WhisperProfiles[i].Engine == WhisperEngineDocker {
				cfg.WhisperProfiles[i].URL = replaceIP(cfg.WhisperProfiles[i].URL, ip)
			}
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		_ = writeFileAtomic(configPath(), append(data, '\n'), 0600)
		fmt.Printf("Created default configuration file at: '%s'\n", configPath())
	}
}

func applyEnvOverrides(cfg *Config) {
	applyBackendEnvOverrides(cfg)
	applyWhisperAndRemoteEnvOverrides(cfg)
	applyGeminiEnvOverrides(cfg)
	applyAPIKeyEnvOverrides(cfg)
}

func applyBackendEnvOverrides(cfg *Config) {
	if v := os.Getenv("WHISPER_URL"); v != "" {
		cfg.WhisperURL = v
	}
	if v := os.Getenv("ABS_URL"); v != "" {
		cfg.AudiobookshelfURL = v
	} else if v := os.Getenv("AUDIOBOOKSHELF_URL"); v != "" {
		cfg.AudiobookshelfURL = v
	} else if v := os.Getenv("ABS_HOST"); v != "" {
		cfg.AudiobookshelfURL = v
	}
	if v := os.Getenv("ABS_USER"); v != "" {
		cfg.AudiobookshelfUser = v
	} else if v := os.Getenv("AUDIOBOOKSHELF_USER"); v != "" {
		cfg.AudiobookshelfUser = v
	}
	if v := os.Getenv("ABS_PASS"); v != "" {
		cfg.AudiobookshelfPass = v
	} else if v := os.Getenv("AUDIOBOOKSHELF_PASS"); v != "" {
		cfg.AudiobookshelfPass = v
	}
	if v := os.Getenv("ABS_TOKEN"); v != "" {
		cfg.AudiobookshelfToken = v
	}
	if v := os.Getenv("ABS_SQLITE_DB_PATH"); v != "" {
		cfg.AudiobookshelfDBPath = v
	}
	if v := os.Getenv("BACKEND_TYPE"); v != "" {
		cfg.BackendType = v
	} else if v := os.Getenv("ABS_BACKEND"); v != "" {
		cfg.BackendType = v
	} else if v := os.Getenv("BACKEND"); v != "" {
		cfg.BackendType = v
	}
	if v := os.Getenv("PODFETCH_URL"); v != "" {
		cfg.PodfetchURL = v
	} else if v := os.Getenv("PODFETCH_HOST"); v != "" {
		cfg.PodfetchURL = v
	}
	if v := os.Getenv("PODFETCH_USER"); v != "" {
		cfg.PodfetchUser = v
	}
	if v := os.Getenv("PODFETCH_PASS"); v != "" {
		cfg.PodfetchPass = v
	}
	if v := os.Getenv("PODFETCH_API_KEY"); v != "" {
		cfg.PodfetchAPIKey = v
	} else if v := os.Getenv("PODFETCH_KEY"); v != "" {
		cfg.PodfetchAPIKey = v
	} else if v := os.Getenv("PODFETCH_TOKEN"); v != "" {
		cfg.PodfetchAPIKey = v
	}
	if v := os.Getenv("PODFETCH_DB_PATH"); v != "" {
		cfg.PodfetchDBPath = v
	} else if v := os.Getenv("PODFETCH_SQLITE_DB_PATH"); v != "" {
		cfg.PodfetchDBPath = v
	} else if v := os.Getenv("PODFETCH_DB"); v != "" {
		cfg.PodfetchDBPath = v
	}
	if v := os.Getenv("PODCASTS_DIR"); v != "" {
		cfg.PodcastsDir = v
	}
}

func applyWhisperAndRemoteEnvOverrides(cfg *Config) {
	if v := os.Getenv("WHISPER_LANGUAGE"); v != "" {
		cfg.WhisperLanguage = v
	}
	if v := os.Getenv("WHISPER_DOCKER_CONTAINER"); v != "" {
		cfg.WhisperDockerContainer = v
	}
	if v := os.Getenv("WHISPER_WAKE_COMMAND"); v != "" {
		cfg.WhisperWakeCommand = v
	}
	if v := os.Getenv("REMOTE_FFMPEG_HOST"); v != "" {
		cfg.RemoteFFmpegHost = v
	} else if v := os.Getenv("ABS_REMOTE_FFMPEG"); v != "" {
		cfg.RemoteFFmpegHost = v
	}
	if v := os.Getenv("REMOTE_HOST"); v != "" {
		cfg.RemoteHost = v
	} else if v := os.Getenv("ABS_REMOTE_HOST"); v != "" {
		cfg.RemoteHost = v
	}
	if v := os.Getenv("DEFAULT_PROCESSING"); v != "" {
		cfg.DefaultProcessing = v
	} else if v := os.Getenv("ABS_DEFAULT_PROCESSING"); v != "" {
		cfg.DefaultProcessing = v
	}
	if v := os.Getenv("REMOTE_WORK_DIR"); v != "" {
		cfg.RemoteWorkDir = v
	} else if v := os.Getenv("ABS_REMOTE_WORK_DIR"); v != "" {
		cfg.RemoteWorkDir = v
	}
	if v := os.Getenv("DEFAULT_DOWNLOAD_POLICY"); v != "" {
		cfg.DefaultDownloadPolicy = normalizeDownloadPolicy(v)
	}
	if v := os.Getenv("DEFAULT_DOWNLOAD_K"); v != "" {
		if k, err := strconv.Atoi(v); err == nil && k > 0 {
			cfg.DefaultDownloadK = k
		}
	}
	if v := os.Getenv("DEFAULT_AD_REMOVAL"); v != "" {
		cfg.DefaultAdRemoval = normalizeAdRemovalMode(v)
	} else if v := os.Getenv("DEFAULT_AD_POLICY"); v != "" {
		cfg.DefaultAdRemoval = normalizeAdRemovalMode(v)
	}
}

func applyGeminiEnvOverrides(cfg *Config) {
	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := os.Getenv("GEMINI_MODEL"); v != "" {
		cfg.GeminiModel = v
	}
	if v := os.Getenv("GEMINI_PROJECT_ID"); v != "" {
		cfg.GeminiProjectID = v
	} else if v := os.Getenv("GCP_PROJECT"); v != "" {
		cfg.GeminiProjectID = v
	} else if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		cfg.GeminiProjectID = v
	}
	if v := os.Getenv("GEMINI_STAGING_BUCKET"); v != "" {
		cfg.GeminiStagingBucket = v
	} else if v := os.Getenv("GCS_STAGING_BUCKET"); v != "" {
		cfg.GeminiStagingBucket = v
	}
	if v := os.Getenv("GEMINI_LOCATION"); v != "" {
		cfg.GeminiLocation = v
	} else if v := os.Getenv("GCP_REGION"); v != "" {
		cfg.GeminiLocation = v
	}
}

const defaultGeminiModel = "gemini-flash-latest"

func (c *Config) GetGeminiAPIKey() string {
	if c == nil {
		return resolveGeminiAPIKey(Config{})
	}
	return resolveGeminiAPIKey(*c)
}

func (c *Config) GetGeminiModel() string {
	if c != nil && c.GeminiModel != "" {
		return c.GeminiModel
	}
	return defaultGeminiModel
}

func (c *Config) GetGeminiProjectID() string {
	if c != nil && c.GeminiProjectID != "" {
		return c.GeminiProjectID
	}
	return "vm-on-cloud-sariel"
}

func (c *Config) GetGeminiStagingBucket() string {
	if c != nil && c.GeminiStagingBucket != "" {
		return strings.TrimPrefix(c.GeminiStagingBucket, "gs://")
	}
	return "abs-audio-staging-sariel"
}

func (c *Config) GetGeminiLocation() string {
	if c != nil && c.GeminiLocation != "" {
		return c.GeminiLocation
	}
	return "us-central1"
}

func inferWhisperEngine(wp WhisperProfile) WhisperEngine {
	if wp.Engine != "" {
		return wp.Engine
	}
	if wp.URL == "" || wp.CliBinary != "" || strings.Contains(strings.ToLower(wp.Name), "local") || (wp.Model != "" && wp.URL == "") {
		return WhisperEngineLocal
	}
	return inferWhisperEngineFromURL(wp.URL)
}

func inferWhisperEngineFromURL(url string) WhisperEngine {
	if url == "" || strings.Contains(url, "whisper-cli") {
		return WhisperEngineLocal
	}
	lower := strings.ToLower(url)
	if strings.Contains(lower, "gemini") {
		return WhisperEngineGemini
	}
	if strings.Contains(lower, ":8088") || strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "192.168.") {
		return WhisperEngineDocker
	}
	return WhisperEngineRemote
}

func resolveActiveWhisperProfile(cfg *Config) {
	if cfg.ActiveWhisperID <= 0 && len(cfg.WhisperProfiles) > 0 {
		cfg.ActiveWhisperID = cfg.WhisperProfiles[0].ID
	}
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID == cfg.ActiveWhisperID {
			engine := wp.Engine
			if engine == "" {
				engine = inferWhisperEngine(wp)
			}
			cfg.WhisperEngine = engine
			cfg.WhisperModel = wp.Model
			cfg.WhisperCliBinary = wp.CliBinary
			cfg.WhisperProcessors = wp.Processors
			cfg.WhisperThreads = wp.Threads
			cfg.WhisperGreedy = wp.Greedy
			if wp.URL != "" {
				cfg.WhisperURL = wp.URL
			}
			if wp.SpeedFactor > 0 {
				cfg.WhisperSpeedFactor = wp.SpeedFactor
			}
			cfg.WhisperDockerContainer = wp.DockerContainer
			cfg.WhisperLanguage = wp.Language
			cfg.WhisperPrompt = wp.Prompt
			cfg.WhisperWakeCommand = wp.WakeCommand
			return
		}
	}
	if cfg.WhisperEngine == "" {
		cfg.WhisperEngine = inferWhisperEngineFromURL(cfg.WhisperURL)
	}
}

func getActiveWhisperProfile(cfg Config) WhisperProfile {
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID == cfg.ActiveWhisperID {
			return normalizeWhisperProfile(wp)
		}
	}
	if len(cfg.WhisperProfiles) > 0 {
		return normalizeWhisperProfile(cfg.WhisperProfiles[0])
	}
	return fallbackWhisperProfile(cfg)
}

func normalizeWhisperProfile(wp WhisperProfile) WhisperProfile {
	if wp.Engine == "" {
		wp.Engine = inferWhisperEngine(wp)
	}
	if wp.Engine == WhisperEngineLocal {
		if wp.Model == "" {
			wp.Model = "tiny.en"
		}
		if wp.Processors <= 0 {
			wp.Processors = 4
		}
		if wp.Threads <= 0 {
			wp.Threads = 4
		}
	}
	return wp
}

func fallbackWhisperProfile(cfg Config) WhisperProfile {
	engine := cfg.WhisperEngine
	if engine == "" {
		engine = inferWhisperEngineFromURL(cfg.WhisperURL)
	}
	model := cfg.WhisperModel
	if engine == WhisperEngineLocal && model == "" {
		model = "tiny.en"
	}
	procs := cfg.WhisperProcessors
	if engine == WhisperEngineLocal && procs <= 0 {
		procs = 4
	}
	threads := cfg.WhisperThreads
	if engine == WhisperEngineLocal && threads <= 0 {
		threads = 4
	}
	return WhisperProfile{
		ID:              0,
		Name:            "Default",
		Engine:          engine,
		URL:             cfg.WhisperURL,
		SpeedFactor:     cfg.WhisperSpeedFactor,
		DockerContainer: cfg.WhisperDockerContainer,
		Language:        cfg.WhisperLanguage,
		Prompt:          cfg.WhisperPrompt,
		WakeCommand:     cfg.WhisperWakeCommand,
		Model:           model,
		CliBinary:       cfg.WhisperCliBinary,
		Processors:      procs,
		Threads:         threads,
		Greedy:          cfg.WhisperGreedy,
	}
}

var configLoadMu syncMutex
var configLoadFailed bool

func setConfigLoadFailed(v bool) {
	configLoadMu.Lock()
	defer configLoadMu.Unlock()
	configLoadFailed = v
}

func configLoadDidFail() bool {
	configLoadMu.Lock()
	defer configLoadMu.Unlock()
	return configLoadFailed
}

var configFileSnapshot *Config

func setConfigFileSnapshot(c Config) {
	configLoadMu.Lock()
	defer configLoadMu.Unlock()
	snap := c
	configFileSnapshot = &snap
}

func getConfigFileSnapshot() (Config, bool) {
	configLoadMu.Lock()
	defer configLoadMu.Unlock()
	if configFileSnapshot == nil {
		return Config{}, false
	}
	return *configFileSnapshot, true
}

func restoreBackendEnvOverriddenFields(cfg *Config, disk Config, anySet func(...string) bool) {
	if anySet("ABS_URL", "AUDIOBOOKSHELF_URL", "ABS_HOST") {
		cfg.AudiobookshelfURL = disk.AudiobookshelfURL
	}
	if anySet("ABS_USER", "AUDIOBOOKSHELF_USER") {
		cfg.AudiobookshelfUser = disk.AudiobookshelfUser
	}
	if anySet("ABS_PASS", "AUDIOBOOKSHELF_PASS") {
		cfg.AudiobookshelfPass = disk.AudiobookshelfPass
	} else if disk.AudiobookshelfPass == "" && (readAuthSecret("audiobookshelf_password") != "" || readAuthSecret("audiobookshelf_pass") != "") {
		cfg.AudiobookshelfPass = ""
	}
	if anySet("ABS_TOKEN") {
		cfg.AudiobookshelfToken = disk.AudiobookshelfToken
	} else if disk.AudiobookshelfToken == "" && (readAuthSecret("audiobookshelf_token") != "" || readAuthSecret("audiobookshelf_api_key") != "") {
		cfg.AudiobookshelfToken = ""
	}
	if anySet("ABS_SQLITE_DB_PATH") {
		cfg.AudiobookshelfDBPath = disk.AudiobookshelfDBPath
	}
	if anySet("BACKEND_TYPE", "ABS_BACKEND", "BACKEND") {
		cfg.BackendType = disk.BackendType
	}
	if anySet("PODFETCH_URL", "PODFETCH_HOST") {
		cfg.PodfetchURL = disk.PodfetchURL
	}
	if anySet("PODFETCH_USER") {
		cfg.PodfetchUser = disk.PodfetchUser
	}
	if anySet("PODFETCH_PASS") {
		cfg.PodfetchPass = disk.PodfetchPass
	} else if disk.PodfetchPass == "" && (readAuthSecret("podfetch_password") != "" || readAuthSecret("podfetch_pass") != "") {
		cfg.PodfetchPass = ""
	}
	if anySet("PODFETCH_API_KEY", "PODFETCH_KEY", "PODFETCH_TOKEN") {
		cfg.PodfetchAPIKey = disk.PodfetchAPIKey
	} else if disk.PodfetchAPIKey == "" && readAuthSecret("podfetch_api_key") != "" {
		cfg.PodfetchAPIKey = ""
	}
	if anySet("PODCASTS_DIR") {
		cfg.PodcastsDir = disk.PodcastsDir
	}
}

// envOverriddenFields mirrors applyEnvOverrides. Each entry restores the value
// that was on disk whenever the corresponding environment variable is set, so a
// per-invocation override is never persisted by a later saveConfig.
func restoreEnvOverriddenFields(cfg *Config, disk Config) {
	anySet := func(names ...string) bool {
		for _, n := range names {
			if os.Getenv(n) != "" {
				return true
			}
		}
		return false
	}
	restoreBackendEnvOverriddenFields(cfg, disk, anySet)
	if anySet("WHISPER_URL") {
		cfg.WhisperURL = disk.WhisperURL
	}
	if anySet("WHISPER_LANGUAGE") {
		cfg.WhisperLanguage = disk.WhisperLanguage
	}
	if anySet("WHISPER_DOCKER_CONTAINER") {
		cfg.WhisperDockerContainer = disk.WhisperDockerContainer
	}
	if anySet("WHISPER_WAKE_COMMAND") {
		cfg.WhisperWakeCommand = disk.WhisperWakeCommand
	}
	if anySet("REMOTE_FFMPEG_HOST", "ABS_REMOTE_FFMPEG") {
		cfg.RemoteFFmpegHost = disk.RemoteFFmpegHost
	}
	if anySet("REMOTE_HOST", "ABS_REMOTE_HOST") {
		cfg.RemoteHost = disk.RemoteHost
	}
	if anySet("DEFAULT_PROCESSING", "ABS_DEFAULT_PROCESSING") {
		cfg.DefaultProcessing = disk.DefaultProcessing
	}
	if anySet("REMOTE_WORK_DIR", "ABS_REMOTE_WORK_DIR") {
		cfg.RemoteWorkDir = disk.RemoteWorkDir
	}
	if anySet("DEFAULT_DOWNLOAD_POLICY") {
		cfg.DefaultDownloadPolicy = disk.DefaultDownloadPolicy
	}
	if anySet("DEFAULT_DOWNLOAD_K") {
		cfg.DefaultDownloadK = disk.DefaultDownloadK
	}
	if anySet("DEFAULT_AD_REMOVAL", "DEFAULT_AD_POLICY") {
		cfg.DefaultAdRemoval = disk.DefaultAdRemoval
	}
	if anySet("GEMINI_API_KEY_ENABLED") {
		cfg.GeminiAPIKeyEnabled = disk.GeminiAPIKeyEnabled
	}
	if anySet("OPENROUTER_API_KEY_ENABLED") {
		cfg.OpenRouterAPIKeyEnabled = disk.OpenRouterAPIKeyEnabled
	}
	for i := range cfg.Profiles {
		if i < len(disk.Profiles) && disk.Profiles[i].APIKey == "" && readAuthSecret("openrouter_api_key") != "" {
			cfg.Profiles[i].APIKey = ""
		}
	}
}

func loadConfig() Config {
	setConfigLoadFailed(false)
	data, err := os.ReadFile(configPath())
	if err != nil {
		if !os.IsNotExist(err) {
			setConfigLoadFailed(true)
			fmt.Fprintf(os.Stderr, "Error: cannot read configuration file '%s': %v\n", configPath(), err)
			fmt.Fprintf(os.Stderr, "Running with defaults. Configuration changes will be refused until this is resolved.\n")
		}
		cfg := defaultConfig
		applyEnvOverrides(&cfg)
		resolveAuthFolderCredentials(&cfg)
		sanitizeDisabledAPIKeys(&cfg)
		return cfg
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		setConfigLoadFailed(true)
		fmt.Fprintf(os.Stderr, "Error: configuration file '%s' is not valid JSON: %v\n", configPath(), err)
		fmt.Fprintf(os.Stderr, "Running with defaults. Configuration changes will be refused so your\n")
		fmt.Fprintf(os.Stderr, "existing settings and credentials are not overwritten. Fix or remove the file.\n")
		cfg = defaultConfig
		applyEnvOverrides(&cfg)
		resolveAuthFolderCredentials(&cfg)
		sanitizeDisabledAPIKeys(&cfg)
		return cfg
	}
	if cfg.DefaultDownloadPolicy == "" {
		cfg.DefaultDownloadPolicy = "latest"
	}
	if cfg.DefaultDownloadK <= 0 {
		cfg.DefaultDownloadK = 3
	}
	if cfg.DefaultAdRemoval == "" {
		cfg.DefaultAdRemoval = "all"
	}
	if cfg.DefaultProcessing == "" {
		cfg.DefaultProcessing = "local"
	}
	if cfg.RemoteWorkDir == "" {
		cfg.RemoteWorkDir = "~/abs_remote"
	}
	for i := range cfg.WhisperProfiles {
		if cfg.WhisperProfiles[i].Engine == "" {
			cfg.WhisperProfiles[i].Engine = inferWhisperEngine(cfg.WhisperProfiles[i])
		}
	}
	if len(cfg.WhisperProfiles) == 0 && cfg.WhisperURL == "" {
		cfg.WhisperProfiles = defaultWhisperProfiles
		cfg.ActiveWhisperID = 1
	}
	resolveActiveWhisperProfile(&cfg)
	if cfg.PodfetchDBPath == "" && testConfigPath == "" {
		defaultDB := "/media/dockers/podfetch/db/podcast.db"
		if _, err := os.Stat(defaultDB); err == nil {
			cfg.PodfetchDBPath = defaultDB
		}
	}
	setConfigFileSnapshot(cfg)
	applyEnvOverrides(&cfg)
	resolveAuthFolderCredentials(&cfg)
	sanitizeDisabledAPIKeys(&cfg)
	return cfg
}

var testConfigPath string

func saveConfig(cfg Config) {
	if configLoadDidFail() {
		fmt.Fprintf(os.Stderr, "Refusing to write '%s': the existing file could not be read or parsed.\n", configPath())
		fmt.Fprintf(os.Stderr, "Writing now would replace your settings and credentials with defaults.\n")
		return
	}
	dir := configDir()
	os.MkdirAll(dir, 0755)
	if disk, ok := getConfigFileSnapshot(); ok {
		restoreEnvOverriddenFields(&cfg, disk)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not serialise configuration: %v\n", err)
		return
	}
	path := configPath()
	if testConfigPath != "" {
		path = testConfigPath
	}
	if err := writeFileAtomic(path, append(data, '\n'), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not write '%s': %v\n", path, err)
	}
}

func setPodcastsDir(cfg *Config, dir string) {
	cfg.PodcastsDir = dir
	saveConfig(*cfg)
	fmt.Printf("Default podcasts directory updated to: '%s'\n", dir)
}

func printConfig(cfg Config) {
	fmt.Printf("Configuration file: '%s'\n", configPath())
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "(not set)"
	}
	fmt.Printf("  podcasts_dir:             %s\n", podcastsDir)
	if cfg.DefaultDownloadPolicy != "" {
		fmt.Printf("  default_download_policy:  %s\n", cfg.DefaultDownloadPolicy)
	}
	if cfg.DefaultDownloadK > 0 {
		fmt.Printf("  default_download_k:       %d\n", cfg.DefaultDownloadK)
	}
	if cfg.DefaultAdRemoval != "" {
		fmt.Printf("  default_ad_policy:        %s\n", cfg.DefaultAdRemoval)
	}
	if cfg.DefaultProcessing != "" {
		fmt.Printf("  default_processing:       %s\n", cfg.DefaultProcessing)
	}
	if cfg.RemoteHost != "" {
		fmt.Printf("  remote_host:              %s\n", cfg.RemoteHost)
	}
	if cfg.RemoteWorkDir != "" {
		fmt.Printf("  remote_work_dir:          %s\n", cfg.RemoteWorkDir)
	}
	fmt.Printf("  whisper_url:              %s\n", cfg.WhisperURL)
	fmt.Printf("  whisper_speed_factor:     %.1f\n", cfg.WhisperSpeedFactor)
	if cfg.WhisperDockerContainer != "" {
		fmt.Printf("  whisper_docker_container: %s\n", cfg.WhisperDockerContainer)
	}
	if cfg.WhisperWakeCommand != "" {
		fmt.Printf("  whisper_wake_command:     %s\n", cfg.WhisperWakeCommand)
	}
	if cfg.WhisperLanguage != "" {
		fmt.Printf("  whisper_language:         %s\n", cfg.WhisperLanguage)
	}
	fmt.Printf("  active_profile_id:        %d\n", cfg.ActiveProfileID)
	if cfg.ActiveWhisperID > 0 {
		fmt.Printf("  active_whisper_id:        %d\n", cfg.ActiveWhisperID)
	}
	if cfg.AudiobookshelfURL != "" {
		fmt.Printf("  audiobookshelf_url:       %s\n", cfg.AudiobookshelfURL)
	}
	if cfg.AudiobookshelfUser != "" {
		fmt.Printf("  audiobookshelf_user:      %s\n", cfg.AudiobookshelfUser)
	}
	if cfg.BackendType != "" {
		fmt.Printf("  backend_type:             %s\n", cfg.BackendType)
	}
	if cfg.PodfetchURL != "" {
		fmt.Printf("  podfetch_url:             %s\n", cfg.PodfetchURL)
	}
	if cfg.PodfetchUser != "" {
		fmt.Printf("  podfetch_user:            %s\n", cfg.PodfetchUser)
	}
	if cfg.PodfetchDBPath != "" {
		fmt.Printf("  podfetch_db_path:         %s\n", cfg.PodfetchDBPath)
	}
	if cfg.RemoteFFmpegHost != "" {
		fmt.Printf("  remote_ffmpeg_host:       %s\n", cfg.RemoteFFmpegHost)
	}
	fmt.Printf("  gemini_api_key_enabled:   %v\n", cfg.IsGeminiAPIKeyEnabled())
	fmt.Printf("  openrouter_api_key_enabled: %v\n", cfg.IsOpenRouterAPIKeyEnabled())
}

func setRemoteFFmpegHost(cfg *Config, host string) {
	cfg.RemoteFFmpegHost = host
	saveConfig(*cfg)
	if host == "" {
		fmt.Println("Remote FFmpeg host disabled (local cutting enabled).")
	} else {
		fmt.Printf("Remote FFmpeg host updated to: '%s'\n", host)
	}
}

func setAudiobookshelf(cfg *Config, url, user, pass string) {
	if url != "" {
		cfg.AudiobookshelfURL = url
	}
	if user != "" {
		cfg.AudiobookshelfUser = user
	}
	if pass != "" {
		cfg.AudiobookshelfPass = pass
	}
	saveConfig(*cfg)
}

func setConfigFileSnapshotNil() {
	configLoadMu.Lock()
	defer configLoadMu.Unlock()
	configFileSnapshot = nil
}
