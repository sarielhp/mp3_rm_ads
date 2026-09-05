package main

import (
	"path/filepath"
	"testing"
)

func TestInferWhisperEngine(t *testing.T) {
	cases := []struct {
		wp       WhisperProfile
		expected WhisperEngine
	}{
		{
			wp:       WhisperProfile{Engine: WhisperEngineLocal},
			expected: WhisperEngineLocal,
		},
		{
			wp:       WhisperProfile{CliBinary: "whisper-cli"},
			expected: WhisperEngineLocal,
		},
		{
			wp:       WhisperProfile{Model: "tiny.en"},
			expected: WhisperEngineLocal,
		},
		{
			wp:       WhisperProfile{Name: "Local GPU runner"},
			expected: WhisperEngineLocal,
		},
		{
			wp:       WhisperProfile{URL: "http://localhost:8088/inference"},
			expected: WhisperEngineDocker,
		},
		{
			wp:       WhisperProfile{URL: "http://192.168.1.50:8088/inference"},
			expected: WhisperEngineDocker,
		},
		{
			wp:       WhisperProfile{URL: "http://cloud8:8000/v1/audio/transcriptions"},
			expected: WhisperEngineRemote,
		},
	}

	for i, tc := range cases {
		got := inferWhisperEngine(tc.wp)
		if got != tc.expected {
			t.Errorf("case %d: expected %q, got %q", i, tc.expected, got)
		}
	}
}

func TestParseWhisperProfileSpec(t *testing.T) {
	wpLocal := parseWhisperProfileSpec("Local Engine|local|tiny.en|70.0|4|4|true", 1)
	if wpLocal.Engine != WhisperEngineLocal || wpLocal.Model != "tiny.en" || wpLocal.SpeedFactor != 70.0 {
		t.Errorf("local spec parse error: %+v", wpLocal)
	}
	if wpLocal.Processors != 4 || wpLocal.Threads != 4 || !wpLocal.Greedy {
		t.Errorf("local options parse error: %+v", wpLocal)
	}

	wpDocker := parseWhisperProfileSpec("Docker Engine|docker|http://localhost:8088/inference|7.0|whisper-container|en|prompt", 2)
	if wpDocker.Engine != WhisperEngineDocker || wpDocker.URL != "http://localhost:8088/inference" || wpDocker.DockerContainer != "whisper-container" {
		t.Errorf("docker spec parse error: %+v", wpDocker)
	}

	wpRemote := parseWhisperProfileSpec("Remote Engine|remote|http://cloud8:8000|7.0|wake_cmd|en|prompt", 3)
	if wpRemote.Engine != WhisperEngineRemote || wpRemote.URL != "http://cloud8:8000" || wpRemote.WakeCommand != "wake_cmd" {
		t.Errorf("remote spec parse error: %+v", wpRemote)
	}

	wpLegacy := parseWhisperProfileSpec("Legacy Server|http://10.0.0.1:8000|8.5|cont|he|prm|wake", 4)
	if wpLegacy.Engine != WhisperEngineRemote || wpLegacy.URL != "http://10.0.0.1:8000" || wpLegacy.SpeedFactor != 8.5 {
		t.Errorf("legacy spec parse error: %+v", wpLegacy)
	}
}

func TestAddAndRemoveWhisperProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := testConfigPath
	testConfigPath = filepath.Join(tmpDir, "config.json")
	defer func() { testConfigPath = origPath }()

	cfg := Config{
		ActiveWhisperID: 1,
		WhisperProfiles: []WhisperProfile{
			{ID: 1, Name: "Default", Engine: WhisperEngineLocal, Model: "tiny.en"},
		},
	}

	addWhisperProfile(&cfg, "Docker Test|docker|http://127.0.0.1:8088/inference|7.0")
	if len(cfg.WhisperProfiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.WhisperProfiles))
	}
	if cfg.WhisperProfiles[1].Engine != WhisperEngineDocker {
		t.Errorf("expected engine docker, got %s", cfg.WhisperProfiles[1].Engine)
	}

	setDefaultWhisperProfile(&cfg, 2)
	if cfg.ActiveWhisperID != 2 || cfg.WhisperEngine != WhisperEngineDocker {
		t.Errorf("expected active whisper id 2 with docker engine, got id %d engine %s", cfg.ActiveWhisperID, cfg.WhisperEngine)
	}

	removeWhisperProfile(&cfg, 2)
	if len(cfg.WhisperProfiles) != 1 {
		t.Fatalf("expected 1 profile remaining, got %d", len(cfg.WhisperProfiles))
	}
	if cfg.ActiveWhisperID != 1 {
		t.Errorf("expected active whisper fallback to 1, got %d", cfg.ActiveWhisperID)
	}
}

func TestListWhispersRuns(t *testing.T) {
	cfg := defaultConfig
	listWhispers(cfg)
	cfgEmpty := Config{}
	listWhispers(cfgEmpty)
}

func TestGetActiveWhisperProfile(t *testing.T) {
	cfg := Config{
		ActiveWhisperID: 2,
		WhisperProfiles: []WhisperProfile{
			{ID: 1, Name: "Local", Engine: WhisperEngineLocal, Model: "tiny.en"},
			{ID: 2, Name: "Docker", Engine: WhisperEngineDocker, URL: "http://localhost:8088"},
		},
	}

	wp := getActiveWhisperProfile(cfg)
	if wp.ID != 2 || wp.Engine != WhisperEngineDocker {
		t.Errorf("expected profile 2 docker, got %+v", wp)
	}

	cfgFallback := Config{
		WhisperURL:         "http://192.168.1.100:8088/inference",
		WhisperSpeedFactor: 7.0,
	}
	wpFallback := getActiveWhisperProfile(cfgFallback)
	if wpFallback.Engine != WhisperEngineDocker {
		t.Errorf("expected inferred docker engine, got %s", wpFallback.Engine)
	}
}
