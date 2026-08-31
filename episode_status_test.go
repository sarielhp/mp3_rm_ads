package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEpisodeStatusLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "bogi.mp3")
	_ = os.WriteFile(audioPath, []byte("dummy audio bytes"), 0644)

	st := getOrCreateEpisodeStatus(audioPath)
	if st == nil {
		t.Fatalf("expected non-nil episode status")
	}
	if st.Status != StateDownloaded {
		t.Errorf("expected initial status %s, got %s", StateDownloaded, st.Status)
	}
	if st.MediaFile != "bogi.mp3" {
		t.Errorf("expected MediaFile bogi.mp3, got %s", st.MediaFile)
	}

	statPath := statusPathFor(audioPath)
	if !fileExists(statPath) {
		t.Fatalf("expected status file %s to exist", statPath)
	}

	updateEpisodeStatus(audioPath, func(s *EpisodeStatusFile) {
		s.Status = StateTranscribingLocally
		s.Original.SizeBytes = 12345
		s.Original.DurationSec = 60.0
	})

	loaded, err := loadEpisodeStatus(statPath)
	if err != nil {
		t.Fatalf("loadEpisodeStatus failed: %v", err)
	}
	if loaded.Status != StateTranscribingLocally {
		t.Errorf("expected status %s, got %s", StateTranscribingLocally, loaded.Status)
	}
	if loaded.Original.SizeBytes != 12345 || loaded.Original.DurationSec != 60.0 {
		t.Errorf("unexpected loaded original metadata: %+v", loaded.Original)
	}

	updateEpisodeStatus(audioPath, func(s *EpisodeStatusFile) {
		s.Status = StateDone
		s.Cleaned.DurationSec = 50.0
		s.Cleaned.SizeBytes = 10000
		s.Cleaned.AdDurationSec = 10.0
		s.Ads = []EpisodeAdCut{
			{Start: 10.0, End: 20.0, Reason: "sponsor"},
		}
	})

	if !isEpisodeCompleted(audioPath) {
		t.Errorf("expected isEpisodeCompleted to return true for StateDone")
	}
}

func TestRelativeMediaDir(t *testing.T) {
	tempDir := t.TempDir()
	podcastsDir := filepath.Join(tempDir, "podcasts")
	epPath := filepath.Join(podcastsDir, "History_Show", "ep1.mp3")

	rel, err := computeRelativeMediaDir(podcastsDir, epPath)
	if err != nil {
		t.Fatalf("computeRelativeMediaDir failed: %v", err)
	}
	expected := filepath.Join("History_Show", "ep1.mp3")
	if rel != expected {
		t.Errorf("expected %s, got %s", expected, rel)
	}
}

func TestDoneAndArchiveManifest(t *testing.T) {
	tempDir := t.TempDir()
	donePath := filepath.Join(tempDir, "done.json")
	archPath := filepath.Join(tempDir, "archive.json")

	item := RemoteDoneItem{
		RelPath:             "Podcast/ep1.mp3",
		Status:              StateReadyForCopyBack,
		OriginalDurationSec: 100.0,
		CleanedDurationSec:  80.0,
		CutDurationSec:      20.0,
		WorkerHost:          "gpu-1",
	}

	if err := addDoneEpisode(donePath, item); err != nil {
		t.Fatalf("addDoneEpisode failed: %v", err)
	}

	m, err := loadDoneManifest(donePath)
	if err != nil {
		t.Fatalf("loadDoneManifest failed: %v", err)
	}
	if len(m.Episodes) != 1 || m.Episodes["Podcast/ep1.mp3"].CutDurationSec != 20.0 {
		t.Errorf("unexpected done manifest: %+v", m)
	}

	if err := archiveDoneEpisode(donePath, archPath, "Podcast/ep1.mp3"); err != nil {
		t.Fatalf("archiveDoneEpisode failed: %v", err)
	}

	doneAfter, _ := loadDoneManifest(donePath)
	if len(doneAfter.Episodes) != 0 {
		t.Errorf("expected done.json to have 0 items after archiving, got %d", len(doneAfter.Episodes))
	}

	archAfter, err := loadDoneManifest(archPath)
	if err != nil || len(archAfter.Episodes) != 1 {
		t.Fatalf("expected archive.json to have 1 item, got %d (err: %v)", len(archAfter.Episodes), err)
	}
	if archAfter.Episodes["Podcast/ep1.mp3"].Status != StateArchived {
		t.Errorf("expected archived status, got %s", archAfter.Episodes["Podcast/ep1.mp3"].Status)
	}
}

func TestProcDryRunWithStatusFramework(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Test_Pod")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	ep2 := filepath.Join(podDir, "ep2.mp3")
	ep3 := filepath.Join(podDir, "ep3.mp3")
	_ = os.WriteFile(ep1, []byte("audio 1"), 0644)
	_ = os.WriteFile(ep2, []byte("audio 2"), 0644)
	_ = os.WriteFile(ep3, []byte("audio 3"), 0644)

	updateEpisodeStatus(ep1, func(st *EpisodeStatusFile) {
		st.Status = StateDone
		st.Cleaned.DurationSec = 100.0
	})
	updateEpisodeStatus(ep2, func(st *EpisodeStatusFile) {
		st.Status = StateReadyForCopyBack
	})
	updateEpisodeStatus(ep3, func(st *EpisodeStatusFile) {
		st.Status = StateDownloaded
	})

	cli := CLIOptions{
		DryRun:  true,
		Verbose: true,
	}
	cfg := Config{
		PodcastsDir: tempDir,
	}

	handleProcDryRun([]string{ep1, ep2, ep3}, cli, cfg)
}
