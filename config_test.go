package main

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestGetProfileCostLocal(t *testing.T) {
	c := getProfileCost(LLMProfile{Type: "ollama", URL: "http://localhost:11434", Model: "m"})
	if c.Type != "Local" {
		t.Error("expected Local")
	}
}

func TestLocalIP(t *testing.T) {
	if ip := localIP(); ip == "" {
		t.Error("empty IP")
	}
}

func TestDefaultConfig(t *testing.T) {
	if len(defaultConfig.Profiles) != 4 || defaultConfig.ActiveProfileID != 1 {
		t.Error("default config wrong")
	}
}

func TestFilterKeyTerms(t *testing.T) {
	r := filterKeyTerms([]string{"sonnet", "unknown", "flash"})
	if len(r) != 2 || r[0] != "sonnet" || r[1] != "flash" {
		t.Error("filterKeyTerms failed")
	}
}

func TestSplitModelTokens(t *testing.T) {
	r := splitModelTokens("a-b.c")
	if len(r) != 3 || r[0] != "a" || r[1] != "b" || r[2] != "c" {
		t.Error("splitModelTokens failed")
	}
}

func TestLocalIPError(t *testing.T) {
	if ip := localIP(); ip == "" {
		t.Error("should return an IP")
	}
}

func TestIsLocalHostError(t *testing.T) {
	orig := netInterfaceAddrs
	netInterfaceAddrs = func() ([]net.Addr, error) { return nil, fmt.Errorf("e") }
	defer func() { netInterfaceAddrs = orig }()
	if isLocalHost("1.2.3.4") {
		t.Error("should be false")
	}
}

func TestGetProfileCostOpenRouter(t *testing.T) {
	_ = getProfileCost(LLMProfile{Type: "openrouter", URL: "https://openrouter.ai", Model: "m"})
}

func TestFilterKeyTermsEmpty(t *testing.T) {
	if r := filterKeyTerms([]string{"x"}); len(r) != 0 {
		t.Error("should be empty")
	}
}

func TestSplitModelTokensEmpty(t *testing.T) {
	if r := splitModelTokens(""); len(r) != 0 {
		t.Error("should be empty")
	}
}

func TestGetProfileCostUnknown(t *testing.T) {
	c := getProfileCost(LLMProfile{Type: "unknown", URL: "http://custom:8080", Model: "m"})
	if c.Type != "Unknown" {
		t.Errorf("got %q", c.Type)
	}
}

func TestSetPodcastsDir(t *testing.T) {
	orig := testConfigPath
	testConfigPath = t.TempDir() + "/config.json"
	defer func() { testConfigPath = orig }()

	cfg := &Config{WhisperURL: "http://localhost:8088"}
	setPodcastsDir(cfg, "/tmp/my_podcasts")

	if cfg.PodcastsDir != "/tmp/my_podcasts" {
		t.Errorf("expected /tmp/my_podcasts, got %q", cfg.PodcastsDir)
	}

	data, err := os.ReadFile(testConfigPath)
	if err != nil || len(data) == 0 {
		t.Errorf("failed to read saved config file: %v", err)
	}
}

func TestPrintConfig(t *testing.T) {
	cfg := Config{
		PodcastsDir:            "/tmp/podcasts",
		WhisperURL:             "http://localhost:8088",
		WhisperSpeedFactor:     7.0,
		WhisperDockerContainer: "whisper",
		WhisperLanguage:        "en",
		ActiveProfileID:        1,
	}
	printConfig(cfg)

	cfgEmpty := Config{
		WhisperURL:         "http://localhost:8088",
		WhisperSpeedFactor: 7.0,
		ActiveProfileID:    1,
	}
	printConfig(cfgEmpty)
}
