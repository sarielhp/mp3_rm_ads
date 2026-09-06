package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIKeyEnabledDefaults(t *testing.T) {
	var cfg Config
	if !cfg.IsGeminiAPIKeyEnabled() {
		t.Errorf("expected Gemini API key to be enabled by default")
	}
	if cfg.IsOpenRouterAPIKeyEnabled() {
		t.Errorf("expected OpenRouter API key to be disabled by default")
	}

	f := false
	tr := true
	cfg.GeminiAPIKeyEnabled = &f
	cfg.OpenRouterAPIKeyEnabled = &tr

	if cfg.IsGeminiAPIKeyEnabled() {
		t.Errorf("expected Gemini API key to be disabled when explicitly set to false")
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		t.Errorf("expected OpenRouter API key to be enabled when explicitly set to true")
	}
}

func TestZeroWipeKeyAndIsZeroedKey(t *testing.T) {
	orig := "sk-or-v1-abcdef1234567890"
	wiped := zeroWipeKey(orig)

	if len(wiped) != len(orig) {
		t.Fatalf("expected wiped length %d, got %d", len(orig), len(wiped))
	}
	if wiped != strings.Repeat("0", len(orig)) {
		t.Errorf("expected all zeroes, got %s", wiped)
	}
	if !isZeroedKey(wiped) {
		t.Errorf("expected isZeroedKey to be true for wiped key")
	}
	if isZeroedKey(orig) {
		t.Errorf("expected isZeroedKey to be false for original key")
	}
	if !isZeroedKey("") {
		t.Errorf("expected isZeroedKey to be true for empty string")
	}
	if zeroWipeKey("") != "" {
		t.Errorf("expected zeroWipeKey empty string to return empty string")
	}
}

func TestResolveGeminiAPIKeyWhenDisabled(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origKey := os.Getenv("GEMINI_API_KEY")
	defer func() {
		os.Setenv("HOME", origHome)
		if origKey != "" {
			os.Setenv("GEMINI_API_KEY", origKey)
		} else {
			os.Unsetenv("GEMINI_API_KEY")
		}
	}()

	os.Setenv("HOME", tempDir)
	authDir := filepath.Join(tempDir, ".config", "auth")
	_ = os.MkdirAll(authDir, 0755)
	_ = os.WriteFile(filepath.Join(authDir, "gemini_api_key"), []byte("auth-file-key\n"), 0600)

	f := false
	cfg := Config{
		GeminiAPIKey:        "my-secret-key",
		GeminiAPIKeyEnabled: &f,
	}

	key := resolveGeminiAPIKey(cfg)
	if key != "" {
		t.Errorf("expected empty key when disabled, got %s", key)
	}
}

func TestSanitizeDisabledAPIKeys(t *testing.T) {
	f := false
	origKey := "sk-or-v1-secret12345"
	cfg := Config{
		GeminiAPIKey:            "gemini-secret-6789",
		GeminiAPIKeyEnabled:     &f,
		OpenRouterAPIKeyEnabled: &f,
		Profiles: []LLMProfile{
			{
				ID:     3,
				Type:   "openrouter",
				URL:    "https://openrouter.ai/api/v1/chat/completions",
				APIKey: origKey,
			},
		},
	}

	sanitizeDisabledAPIKeys(&cfg)

	if !isZeroedKey(cfg.GeminiAPIKey) {
		t.Errorf("expected Gemini API key to be zero-wiped, got %s", cfg.GeminiAPIKey)
	}
	if len(cfg.GeminiAPIKey) != len("gemini-secret-6789") {
		t.Errorf("expected zeroed key of length %d, got %d", len("gemini-secret-6789"), len(cfg.GeminiAPIKey))
	}

	profileKey := cfg.Profiles[0].APIKey
	if !isZeroedKey(profileKey) {
		t.Errorf("expected OpenRouter profile key to be zero-wiped, got %s", profileKey)
	}
	if len(profileKey) != len(origKey) {
		t.Errorf("expected zeroed key of length %d, got %d", len(origKey), len(profileKey))
	}
}

func TestValidateOpenRouterKey(t *testing.T) {
	prof := LLMProfile{
		Type: "openrouter",
		URL:  "https://openrouter.ai/api/v1/chat/completions",
	}

	origTestConfig := testConfigPath
	defer func() { testConfigPath = origTestConfig }()

	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "config.json")
	testConfigPath = cfgFile

	f := false
	cfg := Config{OpenRouterAPIKeyEnabled: &f}
	saveConfig(cfg)

	key := "sk-or-v1-secret-test-key"
	_, err := validateOpenRouterKey(prof, key)
	if err == nil {
		t.Fatalf("expected error validating OpenRouter key when disabled")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled error message, got: %v", err)
	}
}

func TestValidateGeminiKey(t *testing.T) {
	origTestConfig := testConfigPath
	defer func() { testConfigPath = origTestConfig }()

	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "config.json")
	testConfigPath = cfgFile

	f := false
	cfg := Config{GeminiAPIKeyEnabled: &f}
	saveConfig(cfg)

	key := "AIzaSySecretGeminiKey"
	_, err := validateGeminiKey(key)
	if err == nil {
		t.Fatalf("expected error validating Gemini key when disabled")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled error message, got: %v", err)
	}
}

func TestApplyAPIKeyEnvOverrides(t *testing.T) {
	origGemini := os.Getenv("GEMINI_API_KEY_ENABLED")
	origOR := os.Getenv("OPENROUTER_API_KEY_ENABLED")
	defer func() {
		if origGemini != "" {
			os.Setenv("GEMINI_API_KEY_ENABLED", origGemini)
		} else {
			os.Unsetenv("GEMINI_API_KEY_ENABLED")
		}
		if origOR != "" {
			os.Setenv("OPENROUTER_API_KEY_ENABLED", origOR)
		} else {
			os.Unsetenv("OPENROUTER_API_KEY_ENABLED")
		}
	}()

	os.Setenv("GEMINI_API_KEY_ENABLED", "false")
	os.Setenv("OPENROUTER_API_KEY_ENABLED", "true")

	var cfg Config
	applyAPIKeyEnvOverrides(&cfg)

	if cfg.IsGeminiAPIKeyEnabled() {
		t.Errorf("expected GEMINI_API_KEY_ENABLED=false override")
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		t.Errorf("expected OPENROUTER_API_KEY_ENABLED=true override")
	}
}
