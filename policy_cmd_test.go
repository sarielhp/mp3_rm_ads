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
