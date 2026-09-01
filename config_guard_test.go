package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const realUserConfig = `{
  "podcasts_dir": "/srv/media/podcasts",
  "audiobookshelf_url": "https://abs.example.internal",
  "audiobookshelf_user": "someone",
  "audiobookshelf_pass": "USER-SECRET-VALUE",
  "podfetch_api_key": "USER-APIKEY-VALUE",
  "profiles": [
    {"id": 7, "name": "Tuned local model", "type": "ollama",
     "url": "http://10.0.0.5:11434/v1/chat/completions", "model": "qwen2.5:32b"}
  ],
  "active_profile_id": 7
}`

func TestCorruptConfigIsNotOverwrittenWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := testConfigPath
	testConfigPath = path
	t.Cleanup(func() { testConfigPath = old; setConfigLoadFailed(false) })

	// One trailing comma: the realistic hand-edit mistake.
	broken := strings.Replace(realUserConfig, `"model": "qwen2.5:32b"}`, `"model": "qwen2.5:32b"},`, 1)
	if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig()
	setPodcastsDir(&cfg, "/tmp/newdir")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"USER-SECRET-VALUE", "USER-APIKEY-VALUE", "Tuned local model", "abs.example.internal"} {
		if !strings.Contains(string(after), secret) {
			t.Errorf("a config write destroyed %q; an unparseable file must never be overwritten", secret)
		}
	}
}

func TestValidConfigStillSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := testConfigPath
	testConfigPath = path
	t.Cleanup(func() { testConfigPath = old; setConfigLoadFailed(false) })

	if err := os.WriteFile(path, []byte(realUserConfig), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	setPodcastsDir(&cfg, "/tmp/newdir")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "/tmp/newdir") {
		t.Errorf("a valid config was not updated; the guard is too aggressive")
	}
	if !strings.Contains(string(after), "USER-SECRET-VALUE") {
		t.Errorf("a valid config round-trip lost the password")
	}
}

func TestEnvOverridesAreNotPersistedToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := testConfigPath
	testConfigPath = path
	t.Cleanup(func() { testConfigPath = old; setConfigLoadFailed(false); setConfigFileSnapshotNil() })

	onDisk := `{"podcasts_dir":"/srv/media/podcasts",` +
		`"whisper_url":"http://real-whisper:8088/inference",` +
		`"audiobookshelf_pass":"REAL-PASSWORD"}`
	if err := os.WriteFile(path, []byte(onDisk), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("WHISPER_URL", "http://EPHEMERAL:9999/inference")
	t.Setenv("ABS_PASS", "EPHEMERAL-PASSWORD")

	cfg := loadConfig()
	if cfg.WhisperURL != "http://EPHEMERAL:9999/inference" {
		t.Fatalf("the env override did not take effect for this run: %q", cfg.WhisperURL)
	}

	// Any ordinary config mutation writes the whole struct back.
	setPodcastsDir(&cfg, "/tmp/newdir")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)
	if strings.Contains(got, "EPHEMERAL") {
		t.Errorf("a per-invocation environment override was written into config.json:\n%s", got)
	}
	if !strings.Contains(got, "http://real-whisper:8088/inference") {
		t.Errorf("the on-disk whisper_url was destroyed")
	}
	if !strings.Contains(got, "REAL-PASSWORD") {
		t.Errorf("the on-disk password was destroyed")
	}
	if !strings.Contains(got, "/tmp/newdir") {
		t.Errorf("the intended change was not written")
	}
}

func TestConfigIsWrittenPrivately(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	old := testConfigPath
	testConfigPath = path
	t.Cleanup(func() { testConfigPath = old; setConfigLoadFailed(false); setConfigFileSnapshotNil() })

	if err := os.WriteFile(path, []byte(`{"podcasts_dir":"/x"}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	cfg.AudiobookshelfPass = "secret"
	saveConfig(cfg)

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Errorf("config holding credentials is mode %04o; it must not be group/other readable",
			fi.Mode().Perm())
	}
}
