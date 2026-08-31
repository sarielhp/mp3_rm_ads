package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalDefaultPoliciesConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	testConfigPath = cfgPath
	defer func() { testConfigPath = "" }()

	if defaultConfig.DefaultDownloadPolicy != "latest" {
		t.Errorf("expected defaultConfig.DefaultDownloadPolicy = 'latest', got %q", defaultConfig.DefaultDownloadPolicy)
	}
	if defaultConfig.DefaultDownloadK != 3 {
		t.Errorf("expected defaultConfig.DefaultDownloadK = 3, got %d", defaultConfig.DefaultDownloadK)
	}
	if defaultConfig.DefaultAdRemoval != "all" {
		t.Errorf("expected defaultConfig.DefaultAdRemoval = 'all', got %q", defaultConfig.DefaultAdRemoval)
	}

	cfg := Config{}
	saveConfig(cfg)

	loaded := loadConfig()
	if loaded.DefaultDownloadPolicy != "latest" {
		t.Errorf("expected loaded default download policy 'latest', got %q", loaded.DefaultDownloadPolicy)
	}
	if loaded.DefaultDownloadK != 3 {
		t.Errorf("expected loaded default download k 3, got %d", loaded.DefaultDownloadK)
	}
	if loaded.DefaultAdRemoval != "all" {
		t.Errorf("expected loaded default ad removal 'all', got %q", loaded.DefaultAdRemoval)
	}
}

func TestGlobalPolicyHandleConfigSetAndGet(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	testConfigPath = cfgPath
	defer func() { testConfigPath = "" }()

	cfg := defaultConfig

	if err := handleConfigSet(&cfg, "default-download-policy", "latest_k"); err != nil {
		t.Fatalf("handleConfigSet default-download-policy failed: %v", err)
	}
	if cfg.DefaultDownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected policy latest_k, got %q", cfg.DefaultDownloadPolicy)
	}

	if err := handleConfigSet(&cfg, "default-download-k", "5"); err != nil {
		t.Fatalf("handleConfigSet default-download-k failed: %v", err)
	}
	if cfg.DefaultDownloadK != 5 {
		t.Errorf("expected download k 5, got %d", cfg.DefaultDownloadK)
	}

	if err := handleConfigSet(&cfg, "default-ad-policy", "latest"); err != nil {
		t.Fatalf("handleConfigSet default-ad-policy failed: %v", err)
	}
	if cfg.DefaultAdRemoval != AdRemovalLatest {
		t.Errorf("expected ad policy latest, got %q", cfg.DefaultAdRemoval)
	}

	loaded := loadConfig()
	if loaded.DefaultDownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected persisted policy latest_k, got %q", loaded.DefaultDownloadPolicy)
	}
	if loaded.DefaultDownloadK != 5 {
		t.Errorf("expected persisted download k 5, got %d", loaded.DefaultDownloadK)
	}
	if loaded.DefaultAdRemoval != AdRemovalLatest {
		t.Errorf("expected persisted ad policy latest, got %q", loaded.DefaultAdRemoval)
	}
}

func TestPodcastConfigInheritanceFromGlobalDefaults(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	testConfigPath = cfgPath
	defer func() { testConfigPath = "" }()

	globalCfg := defaultConfig
	globalCfg.DefaultDownloadPolicy = DownloadPolicyLatestK
	globalCfg.DefaultDownloadK = 8
	globalCfg.DefaultAdRemoval = AdRemovalNone
	saveConfig(globalCfg)

	podDirNoConfig := filepath.Join(tempDir, "podcast_none")
	_ = os.MkdirAll(podDirNoConfig, 0755)

	podCfg := loadPodcastConfig(podDirNoConfig)
	if podCfg.DownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected inherited DownloadPolicy to be %q, got %q", DownloadPolicyLatestK, podCfg.DownloadPolicy)
	}
	if podCfg.DownloadK != 8 {
		t.Errorf("expected inherited DownloadK to be 8, got %d", podCfg.DownloadK)
	}
	if podCfg.AdRemoval != AdRemovalNone {
		t.Errorf("expected inherited AdRemoval to be %q, got %q", AdRemovalNone, podCfg.AdRemoval)
	}

	podDirPartial := filepath.Join(tempDir, "podcast_partial")
	_ = os.MkdirAll(podDirPartial, 0755)
	partialJSON := `{"download_policy": "all"}`
	_ = os.WriteFile(filepath.Join(podDirPartial, podcastConfigFileName), []byte(partialJSON), 0644)

	podCfgPartial := loadPodcastConfig(podDirPartial)
	if podCfgPartial.DownloadPolicy != DownloadPolicyAll {
		t.Errorf("expected explicit DownloadPolicy 'all', got %q", podCfgPartial.DownloadPolicy)
	}
	if podCfgPartial.DownloadK != 8 {
		t.Errorf("expected inherited DownloadK 8, got %d", podCfgPartial.DownloadK)
	}
	if podCfgPartial.AdRemoval != AdRemovalNone {
		t.Errorf("expected inherited AdRemoval 'none', got %q", podCfgPartial.AdRemoval)
	}
}

func TestCLIConfigListAndOptions(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	err := app.Execute([]string{"config", "list"})
	if err != nil {
		t.Fatalf("executing 'config list' failed: %v", err)
	}
	if action != "config" || opts.ConfigCmd != "show" {
		t.Errorf("expected action 'config' with ConfigCmd 'show', got action=%q, cmd=%q", action, opts.ConfigCmd)
	}

	var action2 string
	var opts2 CLIOptions
	app2 := buildCLIApp(&action2, &opts2)

	err = app2.Execute([]string{"config", "--default-download-policy", "all", "--default-download-k", "7", "--default-ad-policy", "none"})
	if err != nil {
		t.Fatalf("executing config with options failed: %v", err)
	}
	if opts2.DefaultDownloadPolicy != "all" {
		t.Errorf("expected opts2.DefaultDownloadPolicy = 'all', got %q", opts2.DefaultDownloadPolicy)
	}
	if opts2.DefaultDownloadK != 7 {
		t.Errorf("expected opts2.DefaultDownloadK = 7, got %d", opts2.DefaultDownloadK)
	}
	if opts2.DefaultAdRemoval != "none" {
		t.Errorf("expected opts2.DefaultAdRemoval = 'none', got %q", opts2.DefaultAdRemoval)
	}
}
