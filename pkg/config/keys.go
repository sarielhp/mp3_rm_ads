package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"abs/pkg/types"
	"abs/pkg/util"
)

func ResolveGeminiAPIKey(cfg *types.Config) string {
	if cfg == nil || !cfg.IsGeminiAPIKeyEnabled() {
		if cfg != nil && cfg.GeminiAPIKey != "" {
			_ = util.ZeroWipeKey(cfg.GeminiAPIKey)
		}
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			_ = util.ZeroWipeKey(envKey)
			os.Unsetenv("GEMINI_API_KEY")
		}
		WipeGeminiKeyFiles()
		return ""
	}

	if cfg.GeminiAPIKey != "" && !util.IsZeroedKey(cfg.GeminiAPIKey) {
		return cfg.GeminiAPIKey
	}
	if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" && !util.IsZeroedKey(envKey) {
		return envKey
	}
	return ReadGeminiKeyFiles()
}

func ReadGeminiKeyFiles() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, rel := range []string{
		filepath.Join(".config", "auth", "gemini_ai_studio_key_free"),
		filepath.Join(".config", "auth", "gemini_api_key_free"),
		filepath.Join(".config", "auth", "gemini_ai_studio_key"),
		filepath.Join(".config", "auth", "gemini_api_key"),
		filepath.Join(".config", "auth", "gemini_ai_studio_key_paid"),
		filepath.Join(".config", "auth", "gemini_api_key_paid"),
		filepath.Join(".config", "gemini", "api_key"),
		filepath.Join(".config", "gemini", "gemini_api_key"),
	} {
		path := filepath.Join(home, rel)
		if data, err := os.ReadFile(path); err == nil {
			if k := strings.TrimSpace(string(data)); k != "" && !util.IsZeroedKey(k) {
				return k
			}
		}
	}
	return ""
}

func WipeGeminiKeyFiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, rel := range []string{
		filepath.Join(".config", "auth", "gemini_ai_studio_key_free"),
		filepath.Join(".config", "auth", "gemini_api_key_free"),
		filepath.Join(".config", "auth", "gemini_ai_studio_key"),
		filepath.Join(".config", "auth", "gemini_api_key"),
		filepath.Join(".config", "auth", "gemini_ai_studio_key_paid"),
		filepath.Join(".config", "auth", "gemini_api_key_paid"),
		filepath.Join(".config", "gemini", "api_key"),
		filepath.Join(".config", "gemini", "gemini_api_key"),
	} {
		path := filepath.Join(home, rel)
		if data, err := os.ReadFile(path); err == nil {
			k := strings.TrimSpace(string(data))
			if k != "" {
				_ = util.ZeroWipeKey(k)
			}
		}
	}
}

func SanitizeDisabledAPIKeys(cfg *types.Config) {
	if cfg == nil {
		return
	}
	if !cfg.IsGeminiAPIKeyEnabled() {
		if cfg.GeminiAPIKey != "" {
			cfg.GeminiAPIKey = util.ZeroWipeKey(cfg.GeminiAPIKey)
		}
		if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
			_ = util.ZeroWipeKey(envKey)
			os.Unsetenv("GEMINI_API_KEY")
		}
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		for i := range cfg.Profiles {
			p := &cfg.Profiles[i]
			if p.Type == "openrouter" || strings.Contains(p.URL, "openrouter") || strings.HasPrefix(p.APIKey, "sk-or-") {
				if p.APIKey != "" {
					p.APIKey = util.ZeroWipeKey(p.APIKey)
				}
			}
		}
		if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
			_ = util.ZeroWipeKey(envKey)
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && strings.HasPrefix(envKey, "sk-or-") {
			_ = util.ZeroWipeKey(envKey)
			os.Unsetenv("OPENAI_API_KEY")
		}
	}
}

func ValidateOpenRouterKey(profile types.LLMProfile, apiKey string, enabled bool) (string, error) {
	isOpenRouter := profile.Type == "openrouter" || strings.Contains(profile.URL, "openrouter") || strings.HasPrefix(apiKey, "sk-or-")
	if isOpenRouter {
		if !enabled || util.IsZeroedKey(apiKey) {
			if apiKey != "" {
				_ = util.ZeroWipeKey(apiKey)
			}
			return "", fmt.Errorf("openrouter API key is disabled in configuration")
		}
	}
	return apiKey, nil
}

func ValidateGeminiKey(apiKey string, enabled bool) (string, error) {
	if !enabled || util.IsZeroedKey(apiKey) {
		if apiKey != "" {
			_ = util.ZeroWipeKey(apiKey)
		}
		return "", fmt.Errorf("gemini API key is disabled in configuration")
	}
	return apiKey, nil
}

func ApplyAPIKeyEnvOverrides(cfg *types.Config) {
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

func ReadAuthSecret(name string) string {
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
	if util.IsZeroedKey(k) {
		return ""
	}
	return k
}

func ResolveOpenRouterAPIKey(profile types.LLMProfile, cfg *types.Config) string {
	isOpenRouter := profile.Type == "openrouter" || strings.Contains(profile.URL, "openrouter") || strings.HasPrefix(profile.APIKey, "sk-or-")
	if !isOpenRouter {
		return profile.APIKey
	}
	if cfg == nil || !cfg.IsOpenRouterAPIKeyEnabled() {
		if profile.APIKey != "" {
			_ = util.ZeroWipeKey(profile.APIKey)
		}
		if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
			_ = util.ZeroWipeKey(envKey)
			os.Unsetenv("OPENROUTER_API_KEY")
		}
		return ""
	}
	if profile.APIKey != "" && !util.IsZeroedKey(profile.APIKey) {
		return profile.APIKey
	}
	if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" && !util.IsZeroedKey(envKey) {
		return envKey
	}
	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && strings.HasPrefix(envKey, "sk-or-") && !util.IsZeroedKey(envKey) {
		return envKey
	}
	return ReadAuthSecret("openrouter_api_key")
}

func ResolveAuthFolderCredentials(cfg *types.Config) {
	if cfg == nil {
		return
	}
	if cfg.PodfetchAPIKey == "" {
		cfg.PodfetchAPIKey = ReadAuthSecret("podfetch_api_key")
	}
	if cfg.PodfetchPass == "" {
		if pass := ReadAuthSecret("podfetch_password"); pass != "" {
			cfg.PodfetchPass = pass
		} else {
			cfg.PodfetchPass = ReadAuthSecret("podfetch_pass")
		}
	}
	if cfg.AudiobookshelfToken == "" {
		if tok := ReadAuthSecret("audiobookshelf_token"); tok != "" {
			cfg.AudiobookshelfToken = tok
		} else if tok := ReadAuthSecret("audiobookshelf_api_key"); tok != "" {
			cfg.AudiobookshelfToken = tok
		} else {
			cfg.AudiobookshelfToken = ReadAuthSecret("abs_token")
		}
	}
	if cfg.AudiobookshelfPass == "" {
		if pass := ReadAuthSecret("audiobookshelf_password"); pass != "" {
			cfg.AudiobookshelfPass = pass
		} else {
			cfg.AudiobookshelfPass = ReadAuthSecret("audiobookshelf_pass")
		}
	}
}
