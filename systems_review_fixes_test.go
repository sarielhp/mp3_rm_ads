package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
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

func TestWorkerLockOSAdvisoryExclusion(t *testing.T) {
	tempDir := t.TempDir()
	lockPath := filepath.Join(tempDir, ".worker.lock")

	extFlock := flock.New(lockPath)
	locked, err := extFlock.TryLock()
	if err != nil || !locked {
		t.Fatalf("failed to acquire extFlock: %v", err)
	}
	defer extFlock.Unlock()

	_, err = acquireWorkerLock(tempDir)
	if err == nil {
		t.Fatalf("expected acquireWorkerLock to fail when flock is held")
	}
	if err.Error() != "remote worker is already running" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckSkipOrLockAudioFile_LockErrorFailsClosed(t *testing.T) {
	cli := CLIOptions{Quiet: true}
	invalidPath := filepath.Join(t.TempDir(), "nonexistent", "podcast.mp3")

	lock, proceed, shouldStop := checkSkipOrLockAudioFile(invalidPath, invalidPath, 0, 1, 0, cli)
	if lock != nil {
		lock.Release()
		t.Errorf("expected nil lock on acquire error, got %v", lock)
	}
	if proceed {
		t.Errorf("expected proceed=false on acquire error, got true")
	}
	if shouldStop {
		t.Errorf("expected shouldStop=false on acquire error, got true")
	}
}

func TestInstallCutAudioAndPreserveOriginal_SuccessAndRollback(t *testing.T) {
	tempDir := t.TempDir()
	mainMP3 := filepath.Join(tempDir, "ep.mp3")
	precut := filepath.Join(tempDir, "ep.mp3.precut")
	tmpOut := filepath.Join(tempDir, "temp_cut.mp3")
	workDir := filepath.Join(tempDir, ".work")
	_ = os.MkdirAll(workDir, 0755)

	_ = os.WriteFile(mainMP3, []byte("original audio"), 0644)
	_ = os.WriteFile(tmpOut, []byte("cut audio"), 0644)

	ok := installCutAudioAndPreserveOriginal(mainMP3, mainMP3, precut, mainMP3, tmpOut, workDir, true)
	if !ok {
		t.Fatalf("expected successful install")
	}
	if b, _ := os.ReadFile(mainMP3); string(b) != "cut audio" {
		t.Errorf("expected mainMP3 to have cut audio, got %s", string(b))
	}
	if b, _ := os.ReadFile(precut); string(b) != "original audio" {
		t.Errorf("expected precut to have original audio, got %s", string(b))
	}

	_ = os.Remove(precut)
	_ = os.WriteFile(mainMP3, []byte("original audio 2"), 0644)
	failed := installCutAudioAndPreserveOriginal(mainMP3, mainMP3, precut, mainMP3, "/nonexistent/path.mp3", workDir, true)
	if failed {
		t.Fatalf("expected failure when temp output does not exist")
	}
	if b, _ := os.ReadFile(mainMP3); string(b) != "original audio 2" {
		t.Errorf("expected mainMP3 to remain intact, got %s", string(b))
	}
	if fileExists(precut) {
		t.Errorf("expected precut to be cleaned up on failure")
	}
}

func TestHandleNoAdsDetected_InstallOutputFailureDoesNotCommit(t *testing.T) {
	tempDir := t.TempDir()
	mainMP3 := filepath.Join(tempDir, "ep.mp3")
	srcAudio := filepath.Join(tempDir, "src.wav")
	roDir := filepath.Join(tempDir, "readonly")
	_ = os.MkdirAll(roDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(roDir, 0755) })

	unwritableOut := filepath.Join(roDir, "nested", "out.mp3")
	_ = os.WriteFile(mainMP3, []byte("main mp3"), 0644)
	_ = os.WriteFile(srcAudio, []byte("source wav"), 0644)

	now := time.Now()
	handleNoAdsDetected(mainMP3, srcAudio, unwritableOut, 100.0, LLMProfile{}, CLIOptions{Quiet: true}, now, now, now)

	cutsPath := filepath.Join(tempDir, "ep.mp3.cuts.json")
	if fileExists(cutsPath) {
		t.Errorf("expected cuts.json not to be created on install failure")
	}

	st, _ := loadEpisodeStatus(statusPathFor(mainMP3))
	if st != nil && st.Status == StateDone {
		t.Errorf("expected status not to be marked done on install failure")
	}
}
