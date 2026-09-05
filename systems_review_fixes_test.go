package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sariel/abs/pkg/backend"
)

func TestTogglePauseStartsLoadedTrack(t *testing.T) {
	orig := playerSpawnEnabled
	playerSpawnEnabled = false
	t.Cleanup(func() { playerSpawnEnabled = orig })

	p := &AudioPlayer{
		Volume:   70,
		Position: 120,
		Current: &PlayerTrack{
			Title:    "Loaded Episode",
			Duration: 1800,
			Path:     "/dummy/path.mp3",
		},
		IsPlaying: false,
		IsPaused:  false,
	}

	p.TogglePause()

	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.IsPlaying {
		t.Errorf("expected IsPlaying to be true after TogglePause on loaded track, got false")
	}
	if p.IsPaused {
		t.Errorf("expected IsPaused to be false when starting from stopped, got true")
	}
}

func TestDownloadQueueReconcileStaleDownloading(t *testing.T) {
	tmpDir := t.TempDir()
	qPath := filepath.Join(tmpDir, "download_queue.json")
	origQueuePath := testDownloadQueuePath
	testDownloadQueuePath = qPath
	t.Cleanup(func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = origQueuePath })

	q := &DownloadQueuePersist{
		Items: []DownloadQueueItem{
			{
				ID:           "test-stale-1",
				PodcastTitle: "Pod A",
				EpisodeTitle: "Ep 1",
				Status:       "downloading",
			},
			{
				ID:           "test-queued-2",
				PodcastTitle: "Pod A",
				EpisodeTitle: "Ep 2",
				Status:       "queued",
			},
		},
	}
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := os.WriteFile(qPath, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Reconcile
	resetReconcileDownloadQueueOnceForTest()
	loaded := loadDownloadQueue()

	foundStale := false
	for _, item := range loaded.Items {
		if item.ID == "test-stale-1" {
			foundStale = true
			if item.Status != "queued" {
				t.Errorf("expected status 'queued' after reconciliation, got '%s'", item.Status)
			}
		}
	}
	if !foundStale {
		t.Errorf("item 'test-stale-1' was not found in loaded queue")
	}
}

func TestDoneManifestLockConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "done.json")

	const numWorkers = 5
	const itemsPerWorker = 5
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < itemsPerWorker; i++ {
				relPath := fmt.Sprintf("pod_%d/ep_%d.mp3", workerID, i)
				item := RemoteDoneItem{
					RelPath: relPath,
					Status:  StateDone,
				}
				if err := addDoneEpisode(manifestPath, item); err != nil {
					t.Errorf("addDoneEpisode error: %v", err)
				}
			}
		}(w)
	}

	wg.Wait()

	m, err := loadDoneManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadDoneManifest error: %v", err)
	}

	expectedCount := numWorkers * itemsPerWorker
	if len(m.Episodes) != expectedCount {
		t.Errorf("expected %d episodes in done manifest, got %d", expectedCount, len(m.Episodes))
	}
}

func TestWriteFileAtomicDurable(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "nested", "atomic.txt")
	content := []byte("hello durable atomic world\n")

	if err := writeFileAtomic(targetPath, content, 0644); err != nil {
		t.Fatalf("writeFileAtomic failed: %v", err)
	}

	readBack, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readBack) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(readBack), string(content))
	}

	// Verify no stray .tmp files left in the directory
	entries, err := os.ReadDir(filepath.Dir(targetPath))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "atomic.txt" {
		t.Errorf("unexpected directory contents: %+v", entries)
	}
}

func TestBuildABSEpisodeMapNilMetadata(t *testing.T) {
	episodes := []absEpisode{
		{
			ID:    "ep-1",
			Title: "Episode One",
			AudioFile: &absAudioFile{
				Metadata: nil,
			},
		},
		{
			ID:    "ep-2",
			Title: "Episode Two",
			AudioFile: &absAudioFile{
				Metadata: &backend.AudioFileMetadata{
					Filename: "ep2.mp3",
					RelPath:  "show/ep2.mp3",
				},
			},
		},
	}

	m := buildABSEpisodeMap(episodes)
	if m["Episode One"] == nil {
		t.Errorf("expected episode one to be in map")
	}
	if m["ep2.mp3"] == nil {
		t.Errorf("expected ep2.mp3 to be in map")
	}
}

func TestQuarantineAbandonedDuplicatesNilMetadata(t *testing.T) {
	tempDir := t.TempDir()
	episodes := []absEpisode{
		{
			ID:    "ep-nil-meta",
			Title: "Episode Nil",
			AudioFile: &absAudioFile{
				Metadata: nil,
			},
		},
	}
	quarantined := quarantineAbandonedDuplicates(tempDir, episodes)
	if len(quarantined) != 0 {
		t.Errorf("expected 0 quarantined, got %d", len(quarantined))
	}
}

func TestTranscriptSearchIndexBoundsClamping(t *testing.T) {
	m := &tuiModel{
		searchQuery:        "match",
		transcriptMatchIdx: 15,
		transcriptLines: []string{
			"first line with match",
			"second line with match",
			"third line with nothing",
		},
	}

	m.prevTranscriptMatch()
	if m.transcriptMatchIdx < 0 || m.transcriptMatchIdx >= 2 {
		t.Errorf("expected transcriptMatchIdx in [0, 1], got %d", m.transcriptMatchIdx)
	}

	m.transcriptMatchIdx = 20
	m.nextTranscriptMatch()
	if m.transcriptMatchIdx < 0 || m.transcriptMatchIdx >= 2 {
		t.Errorf("expected transcriptMatchIdx in [0, 1], got %d", m.transcriptMatchIdx)
	}
}

func TestEnsureConfigExistsPermissions(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	orig := testConfigPath
	testConfigPath = cfgPath
	t.Cleanup(func() { testConfigPath = orig })

	ensureConfigExists()

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected config file permissions 0600, got %04o", perm)
	}
}

func TestAcquireWorkerLockAtomicExcl(t *testing.T) {
	tempDir := t.TempDir()
	unlock1, err := acquireWorkerLock(tempDir)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	defer unlock1()

	unlockSelf, err := acquireWorkerLock(tempDir)
	if err != nil {
		t.Fatalf("expected same-PID re-entry to succeed, got %v", err)
	}
	unlockSelf()
}
