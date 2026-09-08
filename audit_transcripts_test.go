package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditTranscriptMetrics(t *testing.T) {
	itemNormal := &transcriptAuditItem{audioDur: 300.0}
	tdNormal := TranscriptionData{
		Text: "This is a full and complete transcript of a podcast episode with enough text.",
		Segments: []TranscriptionSegment{
			{Start: 0, End: 100, Text: "Part 1"},
			{Start: 100, End: 200, Text: "Part 2"},
		},
	}
	evaluateTranscriptMetrics(itemNormal, tdNormal, 0.15, 50)
	if itemNormal.isSuspicious {
		t.Errorf("expected normal transcript not to be suspicious, got: %s", itemNormal.suspiciousMsg)
	}

	itemShort := &transcriptAuditItem{audioDur: 300.0}
	tdShort := TranscriptionData{
		Text: "Too short",
		Segments: []TranscriptionSegment{
			{Start: 0, End: 5, Text: "Too short"},
		},
	}
	evaluateTranscriptMetrics(itemShort, tdShort, 0.15, 50)
	if !itemShort.isSuspicious {
		t.Errorf("expected short text transcript to be flagged suspicious")
	}

	itemLowCoverage := &transcriptAuditItem{audioDur: 1000.0}
	tdLow := TranscriptionData{
		Text: "This text is longer than fifty characters so it passes the length check easily.",
		Segments: []TranscriptionSegment{
			{Start: 0, End: 50, Text: "Only 50 seconds"},
		},
	}
	evaluateTranscriptMetrics(itemLowCoverage, tdLow, 0.15, 50)
	if !itemLowCoverage.isSuspicious {
		t.Errorf("expected low coverage (5%% < 15%%) to be flagged suspicious")
	}
}

func TestAuditHealSuspicious(t *testing.T) {
	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "episode.mp3")
	if err := os.WriteFile(mp3Path, []byte("fake mp3"), 0644); err != nil {
		t.Fatalf("failed to write mp3: %v", err)
	}

	transPath := filepath.Join(tempDir, "episode.transcript.json")
	cutsPath := filepath.Join(tempDir, "episode.cuts.json")
	statPath := statusPathFor(mp3Path)

	_ = os.WriteFile(transPath, []byte("{}"), 0644)
	_ = os.WriteFile(cutsPath, []byte("{}"), 0644)
	_ = updateEpisodeStatus(mp3Path, func(st *EpisodeStatusFile) {
		st.Status = StateDone
	})

	item := &transcriptAuditItem{
		audioPath:      mp3Path,
		transcriptPath: transPath,
		statusPath:     statPath,
		cutsPath:       cutsPath,
		audioDur:       300,
		isSuspicious:   true,
		suspiciousMsg:  "empty text",
	}

	reportAndHealSuspicious(item, true, true)
	if !fileExists(transPath) || !fileExists(cutsPath) {
		t.Fatalf("dry run must not delete files")
	}

	reportAndHealSuspicious(item, false, true)
	if fileExists(transPath) {
		t.Errorf("expected transcript to be deleted")
	}
	if fileExists(cutsPath) {
		t.Errorf("expected cuts to be deleted")
	}

	st, err := loadEpisodeStatus(statPath)
	if err != nil || st.Status != StateNeedsAdR {
		t.Errorf("expected status StateNeedsAdR, got %v (err: %v)", st.Status, err)
	}
}

func TestAuditFailedAdDetectionStatus(t *testing.T) {
	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "episode.mp3")
	_ = os.WriteFile(mp3Path, []byte("fake mp3"), 0644)

	transPath := filepath.Join(tempDir, "episode.transcript.json")
	td := TranscriptionData{
		Text: "Full podcast text that passes length and coverage checks easily.",
		Segments: []TranscriptionSegment{
			{Start: 0, End: 200, Text: "All good"},
		},
	}
	tdBytes, _ := json.Marshal(td)
	_ = os.WriteFile(transPath, tdBytes, 0644)

	_ = updateTranscriptAdDetectionStatus(transPath, false, "failed", "openrouter", "auth error", 0)
	updateStatusAdDetection(mp3Path, false, "failed", "openrouter", "auth error")

	item := inspectEpisodeTranscript(mp3Path, 0.15, 50)
	if item == nil {
		t.Fatalf("expected item from inspectEpisodeTranscript")
	}
	if !item.adFailed {
		t.Errorf("expected adFailed to be true")
	}

	reportAndHealFailedAd(item, false, true)
	st, _ := loadEpisodeStatus(statusPathFor(mp3Path))
	if st.Status != StateNeedsAdR {
		t.Errorf("expected status StateNeedsAdR, got %s", st.Status)
	}
}

func TestAuditDisplayName(t *testing.T) {
	p1 := "/podcasts/Show/Ep 10/podcast.mp3"
	if name := auditDisplayName(p1); name != "Ep 10" {
		t.Errorf("expected Ep 10, got %s", name)
	}
	p2 := "/podcasts/Show/episode_42.mp3"
	if name := auditDisplayName(p2); name != "episode_42.mp3" {
		t.Errorf("expected episode_42.mp3, got %s", name)
	}
}
