package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

const configDirName = ".config/abs"
const legacyConfigDirName = ".config/mp3_rm_ads"
const configFileName = "config.json"
const opencodeConfigFile = ".config/opencode/opencode.json"

var defaultConfig = Config{
	Instructions:          "Configuration file for abs. Select profiles by ID or set active_profile_id.",
	DefaultDownloadPolicy: "latest",
	DefaultDownloadK:      3,
	DefaultAdRemoval:      "all",
	DefaultProcessing:     "local",
	RemoteWorkDir:         "~/abs_remote",
	WhisperURL:            "http://192.168.1.230:8088/inference",
	WhisperSpeedFactor:    7.0,
	ChunkDurationSec:      0,
	ActiveProfileID:       1,
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
		data, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile(configPath(), append(data, '\n'), 0644)
		fmt.Printf("Created default configuration file at: '%s'\n", configPath())
	}
}

func applyEnvOverrides(cfg *Config) {
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

func resolveActiveWhisperProfile(cfg *Config) {
	if cfg.ActiveWhisperID <= 0 {
		return
	}
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID == cfg.ActiveWhisperID {
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
	resolveActiveWhisperProfile(&cfg)
	applyEnvOverrides(&cfg)
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
	data, _ := json.MarshalIndent(cfg, "", "  ")
	path := configPath()
	if testConfigPath != "" {
		path = testConfigPath
	}
	os.WriteFile(path, append(data, '\n'), 0644)
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
