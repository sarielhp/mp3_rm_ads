package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

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

func TestConfigMigrate(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	pmDir := tempDir + "/.config/podcasts_manager"
	_ = os.MkdirAll(pmDir, 0755)
	pmCfg := `{"host":"http://migrated-host:8080","token":"tok123","sqlite_db_path":"/tmp/db.sqlite","podcasts_dir":"/tmp/pm_pods","post_processors":["/bin/true"]}`
	_ = os.WriteFile(pmDir+"/config.json", []byte(pmCfg), 0644)

	var cfg Config
	handleConfigMigrate(&cfg, "pm")

	if cfg.AudiobookshelfURL != "http://migrated-host:8080" {
		t.Errorf("expected migrated host, got %q", cfg.AudiobookshelfURL)
	}
	if cfg.AudiobookshelfToken != "tok123" {
		t.Errorf("expected tok123, got %q", cfg.AudiobookshelfToken)
	}
	if cfg.PodcastsDir != "/tmp/pm_pods" {
		t.Errorf("expected /tmp/pm_pods, got %q", cfg.PodcastsDir)
	}
	if len(cfg.PostProcessors) != 1 || cfg.PostProcessors[0] != "/bin/true" {
		t.Errorf("expected migrated post processors, got %v", cfg.PostProcessors)
	}
}

func TestConfigMigrateCLI(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	err := app.Execute([]string{"config", "migrate", "pm"})
	if err != nil {
		t.Fatalf("config migrate failed: %v", err)
	}
	if action != "config" || opts.ConfigCmd != "migrate" || opts.ConfigVal != "pm" {
		t.Errorf("unexpected action or opts: action=%q, cmd=%q, val=%q", action, opts.ConfigCmd, opts.ConfigVal)
	}
}

func TestPodFetchConfigSetGet(t *testing.T) {
	orig := testConfigPath
	testConfigPath = t.TempDir() + "/config.json"
	defer func() { testConfigPath = orig }()

	cfg := Config{}
	if err := handleConfigSet(&cfg, "backend-type", "podfetch"); err != nil {
		t.Fatalf("set backend-type failed: %v", err)
	}
	if err := handleConfigSet(&cfg, "podfetch-url", "http://localhost:8000"); err != nil {
		t.Fatalf("set podfetch-url failed: %v", err)
	}
	if err := handleConfigSet(&cfg, "podfetch-user", "pfuser"); err != nil {
		t.Fatalf("set podfetch-user failed: %v", err)
	}
	if err := handleConfigSet(&cfg, "podfetch-pass", "pfpass"); err != nil {
		t.Fatalf("set podfetch-pass failed: %v", err)
	}
	if err := handleConfigSet(&cfg, "podfetch-api-key", "pfkey123"); err != nil {
		t.Fatalf("set podfetch-api-key failed: %v", err)
	}
	if err := handleConfigSet(&cfg, "podfetch-db-path", "/path/to/podcast.db"); err != nil {
		t.Fatalf("set podfetch-db-path failed: %v", err)
	}

	if cfg.BackendType != "podfetch" || cfg.PodfetchURL != "http://localhost:8000" || cfg.PodfetchUser != "pfuser" || cfg.PodfetchPass != "pfpass" || cfg.PodfetchAPIKey != "pfkey123" || cfg.PodfetchDBPath != "/path/to/podcast.db" {
		t.Errorf("unexpected cfg values: %+v", cfg)
	}

	handleConfigGet(cfg, "backend-type")
	handleConfigGet(cfg, "podfetch-url")
	handleConfigGet(cfg, "podfetch-user")
	handleConfigGet(cfg, "podfetch-pass")
	handleConfigGet(cfg, "podfetch-api-key")
	handleConfigGet(cfg, "podfetch-db-path")
}

func TestApplyEnvOverridesPodFetch(t *testing.T) {
	origBackend := os.Getenv("BACKEND_TYPE")
	origURL := os.Getenv("PODFETCH_URL")
	origUser := os.Getenv("PODFETCH_USER")
	origPass := os.Getenv("PODFETCH_PASS")
	origKey := os.Getenv("PODFETCH_API_KEY")
	origDB := os.Getenv("PODFETCH_DB_PATH")
	defer func() {
		os.Setenv("BACKEND_TYPE", origBackend)
		os.Setenv("PODFETCH_URL", origURL)
		os.Setenv("PODFETCH_USER", origUser)
		os.Setenv("PODFETCH_PASS", origPass)
		os.Setenv("PODFETCH_API_KEY", origKey)
		os.Setenv("PODFETCH_DB_PATH", origDB)
	}()

	os.Setenv("BACKEND_TYPE", "podfetch")
	os.Setenv("PODFETCH_URL", "http://podfetch:8000")
	os.Setenv("PODFETCH_USER", "envuser")
	os.Setenv("PODFETCH_PASS", "envpass")
	os.Setenv("PODFETCH_API_KEY", "envkey")
	os.Setenv("PODFETCH_DB_PATH", "/env/podcast.db")

	cfg := Config{}
	applyEnvOverrides(&cfg)

	if cfg.BackendType != "podfetch" || cfg.PodfetchURL != "http://podfetch:8000" || cfg.PodfetchUser != "envuser" || cfg.PodfetchPass != "envpass" || cfg.PodfetchAPIKey != "envkey" || cfg.PodfetchDBPath != "/env/podcast.db" {
		t.Errorf("unexpected config after env overrides: %+v", cfg)
	}
}
