package main

import (
	"fmt"
	"strconv"
	"strings"
)

func handleConfigSetAPIKey(cfg *Config, key, val string) (bool, error) {
	switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
	case "gemini-api-key-enabled":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return true, fmt.Errorf("invalid boolean value for %s: %s", key, val)
		}
		cfg.GeminiAPIKeyEnabled = &b
		return true, nil
	case "openrouter-api-key-enabled":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return true, fmt.Errorf("invalid boolean value for %s: %s", key, val)
		}
		cfg.OpenRouterAPIKeyEnabled = &b
		return true, nil
	case "gemini-api-key":
		cfg.GeminiAPIKey = val
		return true, nil
	case "gemini-model":
		cfg.GeminiModel = val
		return true, nil
	}
	return false, nil
}

func handleConfigSetBackend(cfg *Config, normKey, val string) bool {
	switch normKey {
	case "podcasts-dir", "podcasts.dir", "dir":
		cfg.PodcastsDir = val
	case "backend-type", "backend.type", "backend":
		cfg.BackendType = val
	case "podfetch-url", "podfetch.url":
		cfg.PodfetchURL = val
	case "podfetch-user", "podfetch.user":
		cfg.PodfetchUser = val
	case "podfetch-pass", "podfetch.pass":
		cfg.PodfetchPass = val
	case "podfetch-api-key", "podfetch.api-key", "podfetch-key", "podfetch-token":
		cfg.PodfetchAPIKey = val
	case "podfetch-db-path", "podfetch.db-path", "podfetch-db", "podfetch.db", "podfetch-sqlite-db-path":
		cfg.PodfetchDBPath = val
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
	default:
		return false
	}
	return true
}

func handleConfigSet(cfg *Config, key, val string) error {
	if handled, err := handleConfigSetAPIKey(cfg, key, val); handled {
		if err != nil {
			return err
		}
		saveConfig(*cfg)
		fmt.Printf("Updated '%s' = '%s'\n", key, val)
		return nil
	}
	normKey := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
	if handleConfigSetBackend(cfg, normKey, val) {
		saveConfig(*cfg)
		fmt.Printf("Updated '%s' = '%s'\n", key, val)
		return nil
	}
	switch normKey {
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
	case "default-download-policy", "default-download-mode", "download-policy":
		cfg.DefaultDownloadPolicy = normalizeDownloadPolicy(val)
	case "default-download-k", "default-k", "download-k":
		if k, err := strconv.Atoi(val); err == nil && k > 0 {
			cfg.DefaultDownloadK = k
		} else {
			return fmt.Errorf("invalid default download k: %s", val)
		}
	case "default-ad-policy", "default-ad-removal", "default-ad-mode", "ad-policy", "ad-removal":
		cfg.DefaultAdRemoval = normalizeAdRemovalMode(val)
	case "remote-host", "remote.host", "rhost":
		cfg.RemoteHost = val
	case "default-processing", "default.processing", "processing":
		norm := strings.ToLower(val)
		if norm != "local" && norm != "remote" {
			return fmt.Errorf("invalid default processing value: '%s' (must be 'local' or 'remote')", val)
		}
		cfg.DefaultProcessing = norm
	case "remote-work-dir", "remote.work-dir", "remote-workdir", "rworkdir":
		cfg.RemoteWorkDir = val
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
	case "backend-type", "backend.type", "backend":
		fmt.Println(cfg.BackendType)
	case "podfetch-url", "podfetch.url":
		fmt.Println(cfg.PodfetchURL)
	case "podfetch-user", "podfetch.user":
		fmt.Println(cfg.PodfetchUser)
	case "podfetch-pass", "podfetch.pass":
		fmt.Println(cfg.PodfetchPass)
	case "podfetch-api-key", "podfetch.api-key", "podfetch-key", "podfetch-token":
		fmt.Println(cfg.PodfetchAPIKey)
	case "podfetch-db-path", "podfetch.db-path", "podfetch-db", "podfetch.db", "podfetch-sqlite-db-path":
		fmt.Println(cfg.PodfetchDBPath)
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
	case "remote-host", "remote.host", "rhost":
		fmt.Println(cfg.RemoteHost)
	case "default-processing", "default.processing", "processing":
		fmt.Println(cfg.DefaultProcessing)
	case "remote-work-dir", "remote.work-dir", "remote-workdir", "rworkdir":
		fmt.Println(cfg.RemoteWorkDir)
	case "whisper-url", "whisper.url":
		fmt.Println(cfg.WhisperURL)
	case "whisper-language", "whisper.language", "lang":
		fmt.Println(cfg.WhisperLanguage)
	case "active-profile-id", "llm-id":
		fmt.Println(cfg.ActiveProfileID)
	case "active-whisper-id", "whisper-id":
		fmt.Println(cfg.ActiveWhisperID)
	case "default-download-policy", "default-download-mode", "download-policy":
		fmt.Println(cfg.DefaultDownloadPolicy)
	case "default-download-k", "default-k", "download-k":
		fmt.Println(cfg.DefaultDownloadK)
	case "default-ad-policy", "default-ad-removal", "default-ad-mode", "ad-policy", "ad-removal":
		fmt.Println(cfg.DefaultAdRemoval)
	case "gemini-api-key-enabled":
		fmt.Println(cfg.IsGeminiAPIKeyEnabled())
	case "openrouter-api-key-enabled":
		fmt.Println(cfg.IsOpenRouterAPIKeyEnabled())
	case "gemini-api-key":
		fmt.Println(cfg.GetGeminiAPIKey())
	case "gemini-model":
		fmt.Println(cfg.GetGeminiModel())
	default:
		fmt.Printf("Unknown configuration key: '%s'\n", key)
	}
}
