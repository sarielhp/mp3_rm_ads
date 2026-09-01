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
