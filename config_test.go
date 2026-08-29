package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
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

func TestUserTmpDir(t *testing.T) {
	dir := userTmpDir()
	if !strings.Contains(dir, "abs") {
		t.Errorf("expected dir to contain 'abs', got %q", dir)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Errorf("expected directory to exist, got err=%v", err)
	}

	origUser := os.Getenv("USER")
	origLogname := os.Getenv("LOGNAME")
	defer func() {
		os.Setenv("USER", origUser)
		os.Setenv("LOGNAME", origLogname)
	}()

	os.Setenv("USER", "testuser123")
	dirUser := userTmpDir()
	if !strings.Contains(dirUser, "testuser123") || !strings.Contains(dirUser, "abs") {
		t.Errorf("expected dir to contain 'testuser123' and 'abs', got %q", dirUser)
	}
	os.RemoveAll(dirUser)
}

func TestApplyEnvOverrides(t *testing.T) {
	origWhisper := os.Getenv("WHISPER_URL")
	origABS := os.Getenv("ABS_URL")
	origUser := os.Getenv("ABS_USER")
	origPass := os.Getenv("ABS_PASS")
	origDir := os.Getenv("PODCASTS_DIR")
	origLang := os.Getenv("WHISPER_LANGUAGE")
	origDocker := os.Getenv("WHISPER_DOCKER_CONTAINER")
	defer func() {
		os.Setenv("WHISPER_URL", origWhisper)
		os.Setenv("ABS_URL", origABS)
		os.Setenv("ABS_USER", origUser)
		os.Setenv("ABS_PASS", origPass)
		os.Setenv("PODCASTS_DIR", origDir)
		os.Setenv("WHISPER_LANGUAGE", origLang)
		os.Setenv("WHISPER_DOCKER_CONTAINER", origDocker)
	}()

	os.Setenv("WHISPER_URL", "http://custom-whisper:9000/inference")
	os.Setenv("ABS_URL", "http://custom-abs:8080")
	os.Setenv("ABS_USER", "customuser")
	os.Setenv("ABS_PASS", "custompass")
	os.Setenv("PODCASTS_DIR", "/custom/podcasts")
	os.Setenv("WHISPER_LANGUAGE", "es")
	os.Setenv("WHISPER_DOCKER_CONTAINER", "custom-docker")

	cfg := Config{}
	applyEnvOverrides(&cfg)

	if cfg.WhisperURL != "http://custom-whisper:9000/inference" {
		t.Errorf("expected custom whisper URL, got %q", cfg.WhisperURL)
	}
	if cfg.AudiobookshelfURL != "http://custom-abs:8080" {
		t.Errorf("expected custom ABS URL, got %q", cfg.AudiobookshelfURL)
	}
	if cfg.AudiobookshelfUser != "customuser" || cfg.AudiobookshelfPass != "custompass" {
		t.Errorf("expected custom user/pass, got %q / %q", cfg.AudiobookshelfUser, cfg.AudiobookshelfPass)
	}
	if cfg.PodcastsDir != "/custom/podcasts" {
		t.Errorf("expected custom podcasts dir, got %q", cfg.PodcastsDir)
	}
	if cfg.WhisperLanguage != "es" {
		t.Errorf("expected custom language, got %q", cfg.WhisperLanguage)
	}
	if cfg.WhisperDockerContainer != "custom-docker" {
		t.Errorf("expected custom docker container, got %q", cfg.WhisperDockerContainer)
	}
}

func TestLegacyConfigPath(t *testing.T) {
	p := legacyConfigPath()
	if p != "" && !strings.Contains(p, "mp3_rm_ads") {
		t.Errorf("expected legacy config path to contain mp3_rm_ads, got %q", p)
	}
}

func TestConfigCompletion(t *testing.T) {
	tempDir := t.TempDir()
	origXDG := os.Getenv("XDG_DATA_HOME")
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_DATA_HOME", tempDir+"/data")
	os.Setenv("XDG_CONFIG_HOME", tempDir+"/config")
	defer func() {
		os.Setenv("XDG_DATA_HOME", origXDG)
		os.Setenv("XDG_CONFIG_HOME", origConfig)
	}()

	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	var buf bytes.Buffer
	app.Stdout = &buf
	err := app.Execute([]string{"config", "completion", "bash"})
	if err != nil {
		t.Fatalf("config completion bash failed: %v", err)
	}
	if !strings.Contains(buf.String(), "_abs_complete") {
		t.Errorf("bash completion output unexpected: %s", buf.String())
	}

	buf.Reset()
	err = app.Execute([]string{"config", "completion", "zsh"})
	if err != nil {
		t.Fatalf("config completion zsh failed: %v", err)
	}
	if !strings.Contains(buf.String(), "#compdef abs") {
		t.Errorf("zsh completion output unexpected: %s", buf.String())
	}

	buf.Reset()
	err = app.Execute([]string{"config", "completion", "fish"})
	if err != nil {
		t.Fatalf("config completion fish failed: %v", err)
	}
	if !strings.Contains(buf.String(), "fish completion for abs") {
		t.Errorf("fish completion output unexpected: %s", buf.String())
	}

	buf.Reset()
	err = app.Execute([]string{"config", "completion", "install", "bash"})
	if err != nil {
		t.Fatalf("config completion install bash failed: %v", err)
	}
	installedBash := tempDir + "/data/bash-completion/completions/abs"
	if !fileExists(installedBash) {
		t.Errorf("expected %s to exist", installedBash)
	}
}
