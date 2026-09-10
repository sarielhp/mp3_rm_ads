package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigNoUsername(t *testing.T) {
	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	u := os.Getenv("USER")
	if u != "" && strings.Contains(strings.ToLower(string(data)), strings.ToLower(u)) {
		t.Errorf("DefaultConfig contains current username %q: %s", u, string(data))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ActiveProfileID != 1 {
		t.Errorf("expected ActiveProfileID 1, got %d", cfg.ActiveProfileID)
	}
	if len(cfg.Profiles) == 0 {
		t.Error("expected default profiles to not be empty")
	}
	if len(cfg.WhisperProfiles) == 0 {
		t.Error("expected default whisper profiles to not be empty")
	}
}

func TestEnsureConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.json")
	SetTestConfigPath(confPath)
	defer SetTestConfigPath("")

	cfg, err := EnsureConfigExists()
	if err != nil {
		t.Fatalf("EnsureConfigExists failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if _, err := os.Stat(confPath); err != nil {
		t.Fatalf("expected config file to be created at %s", confPath)
	}

	// Loading again should read existing
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.ActiveProfileID != cfg.ActiveProfileID {
		t.Errorf("expected ActiveProfileID %d, got %d", cfg.ActiveProfileID, loaded.ActiveProfileID)
	}
}

func TestPodcastConfigCycle(t *testing.T) {
	mode := AdRemovalNone
	mode = CycleAdRemovalMode(mode)
	if mode != AdRemovalLatest {
		t.Errorf("expected AdRemovalLatest, got %s", mode)
	}
	mode = CycleAdRemovalMode(mode)
	if mode != AdRemovalAll {
		t.Errorf("expected AdRemovalAll, got %s", mode)
	}
	mode = CycleAdRemovalMode(mode)
	if mode != AdRemovalNone {
		t.Errorf("expected AdRemovalNone, got %s", mode)
	}
}
