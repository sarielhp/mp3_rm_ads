package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"pod/pkg/config"
	"pod/pkg/podcast"
	"strings"
	"testing"
)

func TestPolicyDisplayAndJSON(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Tech_Show")
	_ = os.MkdirAll(podDir, 0755)

	podID := podcast.GetOrSetPodcastShortID(podDir, "Tech Show")
	cfg := Config{PodcastsDir: tempDir}

	// Text mode
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{Args: []string{podID}}
	err := runPolicyCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Policy for Tech_Show") || !strings.Contains(out, podID) {
		t.Errorf("expected policy header with Tech_Show and %s, got: %s", podID, out)
	}
	if !strings.Contains(out, "Auto Download:") || !strings.Contains(out, "Auto Cleanup:") {
		t.Errorf("expected policy fields in output, got: %s", out)
	}

	// JSON mode
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w

	cliJSON := CLIOptions{Args: []string{podID}, JSON: true}
	err = runPolicyCommand(cfg, cliJSON)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyCommand json failed: %v", err)
	}

	jsonBytes, _ := io.ReadAll(r)
	var policyRes PodcastPolicyResult
	if err := json.Unmarshal(jsonBytes, &policyRes); err != nil {
		t.Fatalf("failed to unmarshal policy json: %v", err)
	}
	if policyRes.ID != podID {
		t.Errorf("mismatched policy ID: expected %s, got %s", podID, policyRes.ID)
	}
}

func TestPolicyUpdate(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "News_Cast")
	_ = os.MkdirAll(podDir, 0755)

	podID := podcast.GetOrSetPodcastShortID(podDir, "News Cast")
	cfg := Config{PodcastsDir: tempDir}

	cli := CLIOptions{
		Args: []string{podID},
		PolicyOptions: PolicyOptions{
			AutoDownloadStr: "false",
			DownloadPolicy:  "none",
			AutoCleanupStr:  "true",
			CleanupDays:     14,
			AdRemovalMode:   "latest",
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := runPolicyCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand update failed: %v", err)
	}

	podCfg := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if podCfg.IsAutoDownloadEnabled() {
		t.Errorf("expected AutoDownload to be false")
	}
	if podCfg.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected DownloadPolicy none, got %s", podCfg.DownloadPolicy)
	}
	if !podCfg.IsAutoCleanupEnabled() || podCfg.AutoCleanupDays != 14 {
		t.Errorf("expected AutoCleanup true with 14 days, got %v (%d)", podCfg.IsAutoCleanupEnabled(), podCfg.AutoCleanupDays)
	}
	if podCfg.AdRemoval != AdRemovalLatest {
		t.Errorf("expected AdRemoval latest, got %s", podCfg.AdRemoval)
	}
}

func TestPolicyShorthandNumber(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Shorthand_Show")
	_ = os.MkdirAll(podDir, 0755)

	podID := podcast.GetOrSetPodcastShortID(podDir, "Shorthand Show")
	cfg := Config{PodcastsDir: tempDir}

	cli1 := CLIOptions{
		Args: []string{podID, "1"},
	}
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := runPolicyCommand(cfg, cli1)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand with shorthand 1 failed: %v", err)
	}

	podCfg1 := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if !podCfg1.IsAutoDownloadEnabled() {
		t.Errorf("expected AutoDownload true for shorthand 1")
	}
	if podCfg1.DownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected DownloadPolicy %q, got %q", DownloadPolicyLatestK, podCfg1.DownloadPolicy)
	}
	if podCfg1.DownloadK != 1 {
		t.Errorf("expected DownloadK 1, got %d", podCfg1.DownloadK)
	}
	if podCfg1.AdRemoval != AdRemovalAll {
		t.Errorf("expected AdRemoval %q, got %q", AdRemovalAll, podCfg1.AdRemoval)
	}

	cli5 := CLIOptions{
		Args: []string{podID, "5"},
	}
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	err = runPolicyCommand(cfg, cli5)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand with shorthand 5 failed: %v", err)
	}

	podCfg5 := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if podCfg5.DownloadK != 5 {
		t.Errorf("expected DownloadK 5, got %d", podCfg5.DownloadK)
	}

	cliInvalid := CLIOptions{Args: []string{podID, "0"}}
	if err := runPolicyCommand(cfg, cliInvalid); err == nil {
		t.Errorf("expected error for count 0")
	}
	cliInvalidStr := CLIOptions{Args: []string{podID, "abc"}}
	if err := runPolicyCommand(cfg, cliInvalidStr); err == nil {
		t.Errorf("expected error for non-integer count")
	}
}

func TestPolicyAllPodcasts(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Podcast_One")
	pod2 := filepath.Join(tempDir, "Podcast_Two")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)
	podcast.GetOrSetPodcastShortID(pod1, "Podcast One")
	podcast.GetOrSetPodcastShortID(pod2, "Podcast Two")

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{
		Args: []string{"all"},
		PolicyOptions: PolicyOptions{
			AutoDownloadStr: "false",
			DownloadPolicy:  "none",
			AdRemovalMode:   "latest",
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := runPolicyCommand(cfg, cli)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand all failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Policy updated for 2 podcast(s)") {
		t.Errorf("expected output to mention 2 podcasts updated, got: %s", string(outBytes))
	}

	cfg1 := config.LoadPodcastConfig(pod1, config.PodcastConfig{})
	cfg2 := config.LoadPodcastConfig(pod2, config.PodcastConfig{})
	if cfg1.IsAutoDownloadEnabled() || cfg2.IsAutoDownloadEnabled() {
		t.Errorf("expected auto download false for both podcasts")
	}
	if cfg1.DownloadPolicy != DownloadPolicyNone || cfg2.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected download policy none for both podcasts")
	}
}

func TestPolicyNonFavoritesUpdate(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "regular_show")
	pod2 := filepath.Join(tempDir, "favorite_show")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)

	c1 := config.DefaultPodcastConfig(nil)
	c1.DownloadPolicy = "latest"
	_ = config.SavePodcastConfig(pod1, c1)

	c2 := config.DefaultPodcastConfig(nil)
	c2.Favorite = true
	c2.DownloadPolicy = "new"
	_ = config.SavePodcastConfig(pod2, c2)

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{
		Args: []string{"non-favorites"},
		PolicyOptions: PolicyOptions{
			DownloadPolicy: "none",
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := runPolicyCommand(cfg, cli)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand non-favorites failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Policy updated for 1 non-favorite podcast(s)") {
		t.Errorf("expected output to mention 1 non-favorite podcast updated, got: %s", string(outBytes))
	}

	cfg1 := config.LoadPodcastConfig(pod1, config.PodcastConfig{})
	cfg2 := config.LoadPodcastConfig(pod2, config.PodcastConfig{})
	if cfg1.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected regular_show download policy to be none, got: %s", cfg1.DownloadPolicy)
	}
	if cfg2.DownloadPolicy != "new" || !cfg2.Favorite {
		t.Errorf("expected favorite_show to remain untouched (new, favorite=true), got: %s, fav=%v", cfg2.DownloadPolicy, cfg2.Favorite)
	}
}

func TestPolicyDefaultUpdate(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	_ = os.MkdirAll(configDir, 0755)
	t.Setenv("HOME", tempDir)

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{
		Args: []string{"default"},
		PolicyOptions: PolicyOptions{
			AutoDownloadStr: "false",
			DownloadPolicy:  "none",
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := runPolicyCommand(cfg, cli)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand default failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Global default policy updated") {
		t.Errorf("expected global default policy updated output, got: %s", string(outBytes))
	}

	savedCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if savedCfg.DefaultDownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected default_download_policy 'none', got %q", savedCfg.DefaultDownloadPolicy)
	}
}

func TestPolicyAllWithSetDefault(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	_ = os.MkdirAll(configDir, 0755)
	t.Setenv("HOME", tempDir)

	pod := filepath.Join(tempDir, "Podcast_Alpha")
	_ = os.MkdirAll(pod, 0755)
	podcast.GetOrSetPodcastShortID(pod, "Podcast Alpha")

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{
		PolicyOptions: PolicyOptions{
			PolicyAll:        true,
			AutoDownloadStr:  "false",
			DownloadPolicy:   "none",
			SetDefaultPolicy: true,
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := runPolicyCommand(cfg, cli)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("runPolicyCommand --all --set-default failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Policy updated for 1 podcast(s)") {
		t.Errorf("expected policy updated for 1 podcast, got: %s", string(outBytes))
	}
	if !strings.Contains(string(outBytes), "global default updated: none") {
		t.Errorf("expected global default updated output, got: %s", string(outBytes))
	}

	savedCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if savedCfg.DefaultDownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected default_download_policy 'none', got %q", savedCfg.DefaultDownloadPolicy)
	}
}
