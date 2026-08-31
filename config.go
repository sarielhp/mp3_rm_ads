package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const configDirName = ".config/abs"
const legacyConfigDirName = ".config/mp3_rm_ads"
const configFileName = "config.json"
const opencodeConfigFile = ".config/opencode/opencode.json"

var defaultConfig = Config{
	Instructions:       "Configuration file for abs. Select profiles by ID or set active_profile_id.",
	WhisperURL:         "http://192.168.1.230:8088/inference",
	WhisperSpeedFactor: 7.0,
	ChunkDurationSec:   0,
	ActiveProfileID:    1,
	Profiles: []LLMProfile{
		{ID: 1, Name: "Ollama Local (llama3.1:8b)", Type: "ollama", URL: "http://192.168.1.230:11434/v1/chat/completions", Model: "llama3.1:8b"},
		{ID: 2, Name: "OpenRouter - Claude 3.5 Sonnet", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "anthropic/claude-3.5-sonnet"},
		{ID: 3, Name: "OpenRouter - DeepSeek V4 Flash", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "deepseek/deepseek-v4-flash"},
		{ID: 4, Name: "OpenRouter - Gemini 2.5 Flash", Type: "openrouter", URL: "https://openrouter.ai/api/v1/chat/completions", Model: "google/gemini-2.5-flash"},
	},
	RemoteFFmpegHost: "cloud8",
}

func userTmpDir() string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	if username == "" {
		username = "user"
	}
	dir := filepath.Join(os.TempDir(), username, "abs")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return userTmpDir()
	}
	return filepath.Join(home, configDirName)
}

func configPath() string {
	return filepath.Join(configDir(), configFileName)
}

func legacyConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, legacyConfigDirName, configFileName)
}

func opencodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, opencodeConfigFile)
}

func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ip := ipnet.IP.String()
			if !ipnet.IP.IsMulticast() && !ipnet.IP.IsLinkLocalUnicast() {
				return ip
			}
		}
	}
	return "127.0.0.1"
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

func replaceIP(url, ip string) string {
	return strings.Replace(url, "192.168.1.230", ip, 1)
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

func loadConfig() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		cfg := defaultConfig
		applyEnvOverrides(&cfg)
		return cfg
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = defaultConfig
		applyEnvOverrides(&cfg)
		return cfg
	}
	resolveActiveWhisperProfile(&cfg)
	applyEnvOverrides(&cfg)
	return cfg
}

var testConfigPath string

func saveConfig(cfg Config) {
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

func handleConfigSet(cfg *Config, key, val string) error {
	switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
	case "podcasts-dir", "podcasts.dir", "dir":
		cfg.PodcastsDir = val
	case "abs-url", "abs.url", "audiobookshelf-url", "url":
		cfg.AudiobookshelfURL = val
	case "abs-user", "abs.user", "audiobookshelf-user", "user":
		cfg.AudiobookshelfUser = val
	case "abs-pass", "abs.pass", "audiobookshelf-pass", "pass":
		cfg.AudiobookshelfPass = val
	case "abs-token", "abs.token", "audiobookshelf-token", "token":
		cfg.AudiobookshelfToken = val
	case "db-path", "abs.db", "db", "sqlite-db-path":
		cfg.AudiobookshelfDBPath = val
	case "remote-ffmpeg", "remote-ffmpeg-host", "ffmpeg-host", "rffmpeg":
		cfg.RemoteFFmpegHost = val
	case "whisper-url", "whisper.url":
		cfg.WhisperURL = val
	case "whisper-language", "whisper.language", "language", "lang":
		cfg.WhisperLanguage = val
	case "whisper-wake-command", "whisper.wake-command", "wake-command":
		cfg.WhisperWakeCommand = val
	case "whisper-speed-factor", "whisper.speed-factor", "speed-factor":
		if sf, err := strconv.ParseFloat(val, 64); err == nil && sf > 0 {
			cfg.WhisperSpeedFactor = sf
		} else {
			return fmt.Errorf("invalid speed factor: %s", val)
		}
	case "active-profile-id", "profile-id", "llm-id":
		if id, err := strconv.Atoi(val); err == nil && id > 0 {
			cfg.ActiveProfileID = id
		} else {
			return fmt.Errorf("invalid profile id: %s", val)
		}
	case "active-whisper-id", "whisper-id":
		if id, err := strconv.Atoi(val); err == nil && id > 0 {
			cfg.ActiveWhisperID = id
		} else {
			return fmt.Errorf("invalid whisper id: %s", val)
		}
	default:
		return fmt.Errorf("unknown configuration key: '%s'", key)
	}
	saveConfig(*cfg)
	fmt.Printf("Updated '%s' = '%s'\n", key, val)
	return nil
}

func handleConfigGet(cfg Config, key string) {
	switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
	case "podcasts-dir", "podcasts.dir", "dir":
		fmt.Println(cfg.PodcastsDir)
	case "abs-url", "abs.url", "audiobookshelf-url", "url":
		fmt.Println(cfg.AudiobookshelfURL)
	case "abs-user", "abs.user", "audiobookshelf-user", "user":
		fmt.Println(cfg.AudiobookshelfUser)
	case "abs-token", "abs.token", "token":
		fmt.Println(cfg.AudiobookshelfToken)
	case "db-path", "abs.db", "db":
		fmt.Println(cfg.AudiobookshelfDBPath)
	case "remote-ffmpeg", "remote-ffmpeg-host", "rffmpeg":
		fmt.Println(cfg.RemoteFFmpegHost)
	case "whisper-url", "whisper.url":
		fmt.Println(cfg.WhisperURL)
	case "whisper-language", "whisper.language", "lang":
		fmt.Println(cfg.WhisperLanguage)
	case "active-profile-id", "llm-id":
		fmt.Println(cfg.ActiveProfileID)
	case "active-whisper-id", "whisper-id":
		fmt.Println(cfg.ActiveWhisperID)
	default:
		fmt.Printf("Unknown configuration key: '%s'\n", key)
	}
}

func getProfileCost(profile LLMProfile) CostInfo {
	t := profile.Type
	u := profile.URL
	m := profile.Model

	if t == "ollama" || containsAny(u, []string{"11434", "localhost", "127.0.0.1"}) {
		return CostInfo{
			Type:     "Local",
			CostStr:  "Free ($0.00 / Local GPU)",
			Est1HStr: "$0.00",
		}
	}

	if containsAny(u, []string{"openrouter.ai"}) || t == "openrouter" {
		models := fetchOpenRouterModels()
		clean := cleanModelName(m)
		tokens := splitModelTokens(clean)

		var found *OpenRouterModel
		for i := range models {
			mid := cleanModelName(models[i].ID)
			if mid == clean || mid == m {
				found = &models[i]
				break
			}
		}
		if found == nil {
			for i := range models {
				mid := cleanModelName(models[i].ID)
				if allTokensMatch(mid, tokens) {
					found = &models[i]
					break
				}
			}
		}
		if found == nil {
			keyTerms := filterKeyTerms(tokens)
			if len(keyTerms) > 0 {
				for i := range models {
					mid := cleanModelName(models[i].ID)
					if allTokensMatch(mid, keyTerms) {
						found = &models[i]
						break
					}
				}
			}
		}

		if found != nil && found.Pricing.Prompt != "" {
			promptPrice := parseFloat(found.Pricing.Prompt)
			completionPrice := parseFloat(found.Pricing.Completion)
			in1M := roundFloat(promptPrice*1_000_000, 4)
			out1M := roundFloat(completionPrice*1_000_000, 4)
			est1H := roundFloat(13000*promptPrice+300*completionPrice, 4)
			return CostInfo{
				Type:     fmt.Sprintf("OpenRouter (%s)", found.ID),
				In1M:     in1M,
				Out1M:    out1M,
				CostStr:  fmt.Sprintf("In: $%.4f/1M, Out: $%.4f/1M", in1M, out1M),
				Est1HStr: fmt.Sprintf("~$%.4f / 1-hr episode", est1H),
			}
		}
	}

	return CostInfo{
		Type:     "Unknown",
		CostStr:  "Dynamic pricing unavailable (Check OpenRouter API)",
		Est1HStr: "N/A",
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func cleanModelName(name string) string {
	result := name
	for _, prefix := range []string{"openrouter/", "~"} {
		if len(result) >= len(prefix) && result[:len(prefix)] == prefix {
			result = result[len(prefix):]
		}
	}
	for i := 0; i < len(result); i++ {
		if result[i] == ':' {
			result = result[:i]
			break
		}
	}
	return result
}

func splitModelTokens(name string) []string {
	var tokens []string
	current := ""
	for _, ch := range name {
		if ch == '/' || ch == '.' || ch == '_' || ch == '-' {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}

func allTokensMatch(id string, tokens []string) bool {
	for _, tok := range tokens {
		if !contains(id, tok) {
			return false
		}
	}
	return true
}

func filterKeyTerms(tokens []string) []string {
	keySet := map[string]bool{
		"sonnet": true, "haiku": true, "opus": true, "flash": true,
		"deepseek": true, "llama": true, "gemini": true, "qwen": true, "mistral": true,
	}
	var result []string
	for _, t := range tokens {
		if keySet[t] {
			result = append(result, t)
		}
	}
	return result
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func roundFloat(f float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	return float64(int64(f*pow+0.5)) / pow
}
