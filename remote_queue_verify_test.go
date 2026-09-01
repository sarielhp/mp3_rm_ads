package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoteQueuePolicyRecentFirstThenShortest(t *testing.T) {
	tempDir := t.TempDir()
	now := time.Date(2026, 9, 1, 14, 0, 0, 0, time.UTC)

	ep1 := filepath.Join(tempDir, "rec_1h_long.mp3")
	ep2 := filepath.Join(tempDir, "rec_5h_med.mp3")
	ep3 := filepath.Join(tempDir, "rec_12h_short.mp3")
	ep4 := filepath.Join(tempDir, "old_2d_shortest.mp3")
	ep5 := filepath.Join(tempDir, "old_3d_med.mp3")
	ep6 := filepath.Join(tempDir, "old_5d_long.mp3")

	all := []string{ep1, ep2, ep3, ep4, ep5, ep6}
	for _, p := range all {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}

	_ = saveEpisodeStatus(statusPathFor(ep1), &EpisodeStatusFile{
		MediaFile: "rec_1h_long.mp3", PublishedAt: now.Add(-1 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 3000.0},
	})
	_ = saveEpisodeStatus(statusPathFor(ep2), &EpisodeStatusFile{
		MediaFile: "rec_5h_med.mp3", PublishedAt: now.Add(-5 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 1800.0},
	})
	_ = saveEpisodeStatus(statusPathFor(ep3), &EpisodeStatusFile{
		MediaFile: "rec_12h_short.mp3", PublishedAt: now.Add(-12 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 600.0},
	})
	_ = saveEpisodeStatus(statusPathFor(ep4), &EpisodeStatusFile{
		MediaFile: "old_2d_shortest.mp3", PublishedAt: now.Add(-48 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 300.0},
	})
	_ = saveEpisodeStatus(statusPathFor(ep5), &EpisodeStatusFile{
		MediaFile: "old_3d_med.mp3", PublishedAt: now.Add(-72 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 1500.0},
	})
	_ = saveEpisodeStatus(statusPathFor(ep6), &EpisodeStatusFile{
		MediaFile: "old_5d_long.mp3", PublishedAt: now.Add(-120 * time.Hour).Format(time.RFC3339),
		Original: EpisodeAudioMeta{DurationSec: 3600.0},
	})

	scrambled := []string{ep6, ep3, ep5, ep1, ep4, ep2}
	sortAudioFilesByQueuePolicy(scrambled, now)

	expected := []string{ep1, ep2, ep3, ep4, ep5, ep6}

	if len(scrambled) != len(expected) {
		t.Fatalf("expected %d episodes, got %d", len(expected), len(scrambled))
	}

	for i := range expected {
		if scrambled[i] != expected[i] {
			t.Errorf("queue position [%d]: expected %s, got %s", i+1, filepath.Base(expected[i]), filepath.Base(scrambled[i]))
		}
	}
}

func TestEnsureRemoteEnvironmentMock(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	cfg := &Config{RemoteHost: "mock-box", RemoteWorkDir: filepath.Join(tempDir, "abs_remote")}

	err := ensureRemoteEnvironmentAndWorker(cfg, "mock-box", cfg.RemoteWorkDir, mock, true)
	if err != nil {
		t.Fatalf("ensureRemoteEnvironmentAndWorker failed on mock: %v", err)
	}
}
