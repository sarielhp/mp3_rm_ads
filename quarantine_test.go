package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureABSIgnore(t *testing.T) {
	tempDir := t.TempDir()
	if err := ensureABSIgnore(tempDir); err != nil {
		t.Fatalf("ensureABSIgnore failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, ".absignore"))
	if err != nil {
		t.Fatalf("reading .absignore failed: %v", err)
	}

	content := string(data)
	if !containsStr(content, "*.precut") || !containsStr(content, "*.bak") {
		t.Errorf("expected .absignore to contain *.precut and *.bak, got %q", content)
	}
}

func TestParseABSEpisodePublishedAt(t *testing.T) {
	ep1 := &absEpisode{PublishedAt: 1724000000000}
	if got := parseABSEpisodePublishedAt(ep1); got != 1724000000000 {
		t.Errorf("expected 1724000000000, got %d", got)
	}

	ep2 := &absEpisode{PubDate: "Sun, 20 Aug 2026 12:00:00 GMT"}
	expectedT, _ := time.Parse(time.RFC1123, "Sun, 20 Aug 2026 12:00:00 GMT")
	if got := parseABSEpisodePublishedAt(ep2); got != expectedT.UnixMilli() {
		t.Errorf("expected %d, got %d", expectedT.UnixMilli(), got)
	}

	ep3 := &absEpisode{PubDate: "2026-08-20T12:00:00Z"}
	expectedISO, _ := time.Parse(time.RFC3339, "2026-08-20T12:00:00Z")
	if got := parseABSEpisodePublishedAt(ep3); got != expectedISO.UnixMilli() {
		t.Errorf("expected %d, got %d", expectedISO.UnixMilli(), got)
	}

	if got := parseABSEpisodePublishedAt(nil); got != 0 {
		t.Errorf("expected 0 for nil, got %d", got)
	}
}

func TestQuarantineAbandonedDuplicates(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Hard Fork")
	os.MkdirAll(podDir, 0755)

	guidMP3 := filepath.Join(podDir, "OpenAI Pause (90b50030-4e0f-4e45-af9d-6).mp3")
	bareMP3 := filepath.Join(podDir, "OpenAI Pause.mp3")
	bareCuts := filepath.Join(podDir, "OpenAI Pause.cuts.json")
	bareTranscript := filepath.Join(podDir, "OpenAI Pause.transcript.json")
	barePrecut := filepath.Join(podDir, "OpenAI Pause.mp3.precut")

	os.WriteFile(guidMP3, []byte("tracked audio"), 0644)
	os.WriteFile(bareMP3, []byte("abandoned audio"), 0644)
	os.WriteFile(bareCuts, []byte("{}"), 0644)
	os.WriteFile(bareTranscript, []byte("{}"), 0644)
	os.WriteFile(barePrecut, []byte("precut audio"), 0644)

	trackedEpisodes := []absEpisode{
		{
			Title: "OpenAI Pause",
			AudioFile: &absAudioFile{
				Metadata: &AudioFileMetadata{
					Filename: "OpenAI Pause (90b50030-4e0f-4e45-af9d-6).mp3",
				},
			},
		},
	}

	quarantined := quarantineAbandonedDuplicates(podDir, trackedEpisodes)
	if len(quarantined) != 1 {
		t.Fatalf("expected 1 quarantined file, got %d (%v)", len(quarantined), quarantined)
	}
	if quarantined[0] != "OpenAI Pause.mp3" {
		t.Errorf("expected 'OpenAI Pause.mp3', got %q", quarantined[0])
	}

	if _, err := os.Stat(bareMP3); !os.IsNotExist(err) {
		t.Errorf("expected bare MP3 to no longer exist")
	}
	if _, err := os.Stat(bareMP3 + ".bak"); err != nil {
		t.Errorf("expected bare MP3 .bak to exist: %v", err)
	}
	if _, err := os.Stat(bareCuts + ".bak"); err != nil {
		t.Errorf("expected bare cuts .bak to exist: %v", err)
	}
	if _, err := os.Stat(bareTranscript + ".bak"); err != nil {
		t.Errorf("expected bare transcript .bak to exist: %v", err)
	}
	if _, err := os.Stat(barePrecut + ".bak"); err != nil {
		t.Errorf("expected bare precut .bak to exist: %v", err)
	}

	if _, err := os.Stat(guidMP3); err != nil {
		t.Errorf("tracked GUID MP3 should not be touched: %v", err)
	}

	if _, err := os.Stat(filepath.Join(podDir, ".absignore")); err != nil {
		t.Errorf("expected .absignore to be created: %v", err)
	}
}

func TestQuarantineAbandonedDuplicates_MissingTrackedFileOnDiskDoesNotQuarantine(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "TestPod")
	os.MkdirAll(podDir, 0755)

	bareMP3 := filepath.Join(podDir, "Episode 01.mp3")
	guidMP3 := filepath.Join(podDir, "Episode 01 (9f8e7d).mp3")
	os.WriteFile(bareMP3, []byte("active audio"), 0644)

	trackedEpisodes := []absEpisode{
		{
			Title: "Episode 01",
			AudioFile: &absAudioFile{
				Metadata: &AudioFileMetadata{
					Filename: "Episode 01 (9f8e7d).mp3",
				},
			},
		},
	}

	// Replacement file does not exist on disk yet: must not quarantine active audio
	quarantined := quarantineAbandonedDuplicates(podDir, trackedEpisodes)
	if len(quarantined) != 0 {
		t.Fatalf("expected 0 quarantined files when replacement doesn't exist, got %d", len(quarantined))
	}
	if _, err := os.Stat(bareMP3); err != nil {
		t.Fatalf("active file was deleted/quarantined unexpectedly: %v", err)
	}

	// Now create replacement file on disk
	os.WriteFile(guidMP3, []byte("replacement audio"), 0644)
	quarantined = quarantineAbandonedDuplicates(podDir, trackedEpisodes)
	if len(quarantined) != 1 || quarantined[0] != "Episode 01.mp3" {
		t.Fatalf("expected 1 quarantined file once replacement exists, got %+v", quarantined)
	}
	if _, err := os.Stat(bareMP3 + ".bak"); err != nil {
		t.Fatalf("expected bare MP3 to be quarantined to .bak: %v", err)
	}
}
