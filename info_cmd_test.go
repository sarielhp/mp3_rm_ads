package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfoPodcastCardAndJSON(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Huberman_Lab")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	_ = os.WriteFile(ep1, []byte("audio"), 0644)

	podID := getOrSetPodcastShortID(podDir, "Huberman Lab")
	cfg := Config{PodcastsDir: tempDir}

	// Test text card
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{Args: []string{podID}}
	err := runInfoCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runInfoCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Podcast: Huberman_Lab") || !strings.Contains(out, podID) {
		t.Errorf("expected podcast info header with Huberman_Lab and %s, got: %s", podID, out)
	}
	if !strings.Contains(out, "Policy & Sync:") || !strings.Contains(out, "Library Stats:") {
		t.Errorf("expected policy and library stats in output, got: %s", out)
	}

	// Test JSON mode
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w

	cliJSON := CLIOptions{Args: []string{podID}, JSON: true}
	err = runInfoCommand(cfg, cliJSON)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runInfoCommand json failed: %v", err)
	}

	jsonBytes, _ := io.ReadAll(r)
	var podInfo PodcastInfoJSON
	if err := json.Unmarshal(jsonBytes, &podInfo); err != nil {
		t.Fatalf("failed to parse podcast info json: %v", err)
	}
	if podInfo.ID != podID || podInfo.Title != "Huberman_Lab" {
		t.Errorf("mismatched podInfo: got %+v", podInfo)
	}
}

func TestInfoEpisodeCardAndJSON(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Daily_Cast")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	_ = os.WriteFile(ep1, []byte("audio"), 0644)

	podID := getOrSetPodcastShortID(podDir, "Daily Cast")
	epID := getOrSetEpisodeShortID(podDir, podID, ep1)

	cfg := Config{PodcastsDir: tempDir}

	// Text card
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{Args: []string{epID}}
	err := runInfoCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runInfoCommand failed for episode: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Episode: ep1") || !strings.Contains(out, epID) {
		t.Errorf("expected episode info for %s, got: %s", epID, out)
	}

	// JSON mode
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w

	cliJSON := CLIOptions{Args: []string{epID}, JSON: true}
	err = runInfoCommand(cfg, cliJSON)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runInfoCommand json failed for episode: %v", err)
	}

	jsonBytes, _ := io.ReadAll(r)
	var epInfo EpisodeInfoJSON
	if err := json.Unmarshal(jsonBytes, &epInfo); err != nil {
		t.Fatalf("failed to parse episode info json: %v", err)
	}
	if epInfo.ID != epID || epInfo.PodcastID != podID {
		t.Errorf("mismatched epInfo: got %+v", epInfo)
	}
}

func TestInfoEpisodeWithCuts(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "ShowWithAds")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	_ = os.WriteFile(ep1, []byte("audio"), 0644)

	cutsData := CutsData{
		OriginalDurationSec: 600,
		TotalCutDurationSec: 60,
		CutIntervals: []CutEntry{
			{
				StartSec:       10,
				EndSec:         70,
				DurationSec:    60,
				StartFormatted: "00:10",
				EndFormatted:   "01:10",
				Reason:         "Sponsor segment",
			},
		},
	}
	cutsBytes, _ := json.Marshal(cutsData)
	_ = os.WriteFile(stripExt(ep1)+".cuts.json", cutsBytes, 0644)

	podID := getOrSetPodcastShortID(podDir, "ShowWithAds")
	epID := getOrSetEpisodeShortID(podDir, podID, ep1)

	cfg := Config{PodcastsDir: tempDir}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{Args: []string{epID}, ShowCuts: true}
	err := runInfoCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runInfoCommand failed with cuts: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Sponsor segment") || !strings.Contains(out, "00:10") {
		t.Errorf("expected cuts information in output, got: %s", out)
	}
}

func TestFormatPodcastInfoHebrew(t *testing.T) {
	info := PodcastInfoJSON{
		ID:        "pod1",
		Title:     "פודקאסט חדשות",
		Directory: "/podcasts/news",
		Author:    "יוסי כהן",
		RecentEpisodes: []RecentEpisodeDTO{
			{
				ID:       "ep01",
				Title:    "פרק ראשון",
				Date:     "2026-09-06",
				Status:   "Clean",
				Duration: "10:00",
			},
		},
	}

	out := formatPodcastInfo(info)

	expectedTitle := displayName("פודקאסט חדשות")
	expectedAuthor := displayName("יוסי כהן")
	expectedEp := displayName("פרק ראשון")

	if !strings.Contains(out, expectedTitle) {
		t.Errorf("expected output to contain %q, got: %s", expectedTitle, out)
	}
	if !strings.Contains(out, expectedAuthor) {
		t.Errorf("expected output to contain %q, got: %s", expectedAuthor, out)
	}
	if !strings.Contains(out, expectedEp) {
		t.Errorf("expected output to contain %q, got: %s", expectedEp, out)
	}
}

func TestFormatEpisodeInfoHebrew(t *testing.T) {
	info := EpisodeInfoJSON{
		ID:           "ep01",
		PodcastID:    "pod1",
		PodcastTitle: "פודקאסט היסטוריה",
		Title:        "פרק 1 - העת העתיקה",
		AudioPath:    "/podcasts/hist/ep1.mp3",
		Status:       "Clean",
	}

	out := formatEpisodeInfo(info)

	expectedPod := displayName("פודקאסט היסטוריה")
	expectedTitle := displayName("פרק 1 - העת העתיקה")

	if !strings.Contains(out, expectedPod) {
		t.Errorf("expected output to contain %q, got: %s", expectedPod, out)
	}
	if !strings.Contains(out, expectedTitle) {
		t.Errorf("expected output to contain %q, got: %s", expectedTitle, out)
	}
}
