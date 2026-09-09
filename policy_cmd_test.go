package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyDisplayAndJSON(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Tech_Show")
	_ = os.MkdirAll(podDir, 0755)

	podID := getOrSetPodcastShortID(podDir, "Tech Show")
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

	podID := getOrSetPodcastShortID(podDir, "News Cast")
	cfg := Config{PodcastsDir: tempDir}

	cli := CLIOptions{
		Args:            []string{podID},
		AutoDownloadStr: "false",
		DownloadPolicy:  "none",
		AutoCleanupStr:  "true",
		CleanupDays:     14,
		AdRemovalMode:   "latest",
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

	podCfg := loadPodcastConfig(podDir)
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

	podID := getOrSetPodcastShortID(podDir, "Shorthand Show")
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

	podCfg1 := loadPodcastConfig(podDir)
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

	podCfg5 := loadPodcastConfig(podDir)
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
