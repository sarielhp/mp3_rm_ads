package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func resolveGeminiAPIKey(config Config) string {
	if !config.IsGeminiAPIKeyEnabled() {
		if config.GeminiAPIKey != "" {
			_ = zeroWipeKey(config.GeminiAPIKey)
		}
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			_ = zeroWipeKey(envKey)
			os.Unsetenv("GEMINI_API_KEY")
		}
		wipeGeminiKeyFiles()
		return ""
	}

	if config.GeminiAPIKey != "" && !isZeroedKey(config.GeminiAPIKey) {
		return config.GeminiAPIKey
	}
	if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" && !isZeroedKey(envKey) {
		return envKey
	}
	return readGeminiKeyFiles()
}

func readGeminiKeyFiles() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, rel := range []string{
		filepath.Join(".config", "auth", "gemini_api_key"),
		filepath.Join(".config", "gemini", "api_key"),
		filepath.Join(".config", "gemini", "gemini_api_key"),
	} {
		path := filepath.Join(home, rel)
		if data, err := os.ReadFile(path); err == nil {
			if k := strings.TrimSpace(string(data)); k != "" && !isZeroedKey(k) {
				return k
			}
		}
	}
	return ""
}

func wipeGeminiKeyFiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, rel := range []string{
		filepath.Join(".config", "auth", "gemini_api_key"),
		filepath.Join(".config", "gemini", "api_key"),
		filepath.Join(".config", "gemini", "gemini_api_key"),
	} {
		path := filepath.Join(home, rel)
		if data, err := os.ReadFile(path); err == nil {
			k := strings.TrimSpace(string(data))
			if k != "" {
				_ = zeroWipeKey(k)
			}
		}
	}
}

func sanitizeDisabledAPIKeys(cfg *Config) {
	if cfg == nil {
		return
	}
	if !cfg.IsGeminiAPIKeyEnabled() {
		if cfg.GeminiAPIKey != "" {
			cfg.GeminiAPIKey = zeroWipeKey(cfg.GeminiAPIKey)
		}
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			_ = zeroWipeKey(envKey)
			os.Unsetenv("GEMINI_API_KEY")
		}
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		for i := range cfg.Profiles {
			p := &cfg.Profiles[i]
			if p.Type == "openrouter" || strings.Contains(p.URL, "openrouter") || strings.HasPrefix(p.APIKey, "sk-or-") {
				if p.APIKey != "" {
					p.APIKey = zeroWipeKey(p.APIKey)
				}
			}
		}
		if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
			_ = zeroWipeKey(envKey)
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && strings.HasPrefix(envKey, "sk-or-") {
			_ = zeroWipeKey(envKey)
			os.Unsetenv("OPENAI_API_KEY")
		}
	}
}

func validateOpenRouterKey(profile LLMProfile, apiKey string) (string, error) {
	cfg := loadConfig()
	isOpenRouter := profile.Type == "openrouter" || strings.Contains(profile.URL, "openrouter") || strings.HasPrefix(apiKey, "sk-or-")
	if isOpenRouter {
		if !cfg.IsOpenRouterAPIKeyEnabled() || isZeroedKey(apiKey) {
			if apiKey != "" {
				_ = zeroWipeKey(apiKey)
			}
			return "", fmt.Errorf("openrouter API key is disabled in configuration")
		}
	}
	return apiKey, nil
}

func validateGeminiKey(apiKey string) (string, error) {
	cfg := loadConfig()
	if !cfg.IsGeminiAPIKeyEnabled() || isZeroedKey(apiKey) {
		if apiKey != "" {
			_ = zeroWipeKey(apiKey)
		}
		return "", fmt.Errorf("gemini API key is disabled in configuration")
	}
	return apiKey, nil
}

func applyAPIKeyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GEMINI_API_KEY_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.GeminiAPIKeyEnabled = &b
		}
	}
	if v := os.Getenv("OPENROUTER_API_KEY_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.OpenRouterAPIKeyEnabled = &b
		}
	}
}

func readAuthSecret(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".config", "auth", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	k := strings.TrimSpace(string(data))
	if isZeroedKey(k) {
		return ""
	}
	return k
}

func resolveOpenRouterAPIKey(profile LLMProfile, cfg Config) string {
	isOpenRouter := profile.Type == "openrouter" || strings.Contains(profile.URL, "openrouter") || strings.HasPrefix(profile.APIKey, "sk-or-")
	if !isOpenRouter {
		return profile.APIKey
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		if profile.APIKey != "" {
			_ = zeroWipeKey(profile.APIKey)
		}
		if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
			_ = zeroWipeKey(envKey)
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		return ""
	}
	if profile.APIKey != "" && !isZeroedKey(profile.APIKey) {
		return profile.APIKey
	}
	if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" && !isZeroedKey(envKey) {
		return envKey
	}
	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && strings.HasPrefix(envKey, "sk-or-") && !isZeroedKey(envKey) {
		return envKey
	}
	return readAuthSecret("openrouter_api_key")
}

func resolveAuthFolderCredentials(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.PodfetchAPIKey == "" {
		cfg.PodfetchAPIKey = readAuthSecret("podfetch_api_key")
	}
	if cfg.PodfetchPass == "" {
		if pass := readAuthSecret("podfetch_password"); pass != "" {
			cfg.PodfetchPass = pass
		} else {
			cfg.PodfetchPass = readAuthSecret("podfetch_pass")
		}
	}
	if cfg.AudiobookshelfToken == "" {
		if tok := readAuthSecret("audiobookshelf_token"); tok != "" {
			cfg.AudiobookshelfToken = tok
		} else if tok := readAuthSecret("audiobookshelf_api_key"); tok != "" {
			cfg.AudiobookshelfToken = tok
		} else {
			cfg.AudiobookshelfToken = readAuthSecret("abs_token")
		}
	}
	if cfg.AudiobookshelfPass == "" {
		if pass := readAuthSecret("audiobookshelf_password"); pass != "" {
			cfg.AudiobookshelfPass = pass
		} else {
			cfg.AudiobookshelfPass = readAuthSecret("audiobookshelf_pass")
		}
	}
}
