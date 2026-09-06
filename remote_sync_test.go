package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func setupDownloadIntegrityTestEnv(tempDir, remoteWorkDir, localPodcasts string) string {
	remotePodDir := filepath.Join(remoteWorkDir, "Show")
	_ = os.MkdirAll(remotePodDir, 0755)

	goodAudio := filepath.Join(remotePodDir, "good.mp3")
	badAudio := filepath.Join(remotePodDir, "bad.mp3")
	emptyAudio := filepath.Join(remotePodDir, "empty.mp3")

	goodData := []byte(strings.Repeat("G", 500))
	badData := []byte(strings.Repeat("B", 200))

	_ = os.WriteFile(goodAudio, goodData, 0644)
	_ = os.WriteFile(badAudio, badData, 0644)
	_ = os.WriteFile(emptyAudio, []byte{}, 0644)

	goodStat := filepath.Join(remotePodDir, "good.mp3.json")
	badStat := filepath.Join(remotePodDir, "bad.mp3.json")
	_ = saveEpisodeStatus(goodStat, &EpisodeStatusFile{MediaFile: "good.mp3", Status: StateReadyForCopyBack})
	_ = saveEpisodeStatus(badStat, &EpisodeStatusFile{MediaFile: "bad.mp3", Status: StateReadyForCopyBack})

	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:          filepath.Join("Show", "good.mp3"),
		Status:           StateReadyForCopyBack,
		CleanedSizeBytes: int64(len(goodData)),
	})
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:          filepath.Join("Show", "bad.mp3"),
		Status:           StateReadyForCopyBack,
		CleanedSizeBytes: 1000,
	})
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:          filepath.Join("Show", "empty.mp3"),
		Status:           StateReadyForCopyBack,
		CleanedSizeBytes: 0,
	})
	return donePath
}

func TestDownloadIntegrityVerification(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	donePath := setupDownloadIntegrityTestEnv(tempDir, remoteWorkDir, localPodcasts)

	cfg := &Config{
		RemoteHost:    "mock-box",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	if err := runRemotePull(cfg, "mock-box", mock, true, false); err != nil {
		t.Fatalf("runRemotePull failed: %v", err)
	}

	if !fileExists(filepath.Join(localPodcasts, "Show", "good.mp3")) {
		t.Errorf("expected good.mp3 to be pulled locally")
	}
	if fileExists(filepath.Join(localPodcasts, "Show", "bad.mp3")) {
		t.Errorf("expected bad.mp3 with mismatched size to NOT be pulled locally")
	}
	if fileExists(filepath.Join(localPodcasts, "Show", "empty.mp3")) {
		t.Errorf("expected empty.mp3 to NOT be pulled locally")
	}

	doneM, err := loadDoneManifest(donePath)
	if err != nil {
		t.Fatalf("failed to load done manifest: %v", err)
	}
	if _, ok := doneM.Episodes[filepath.Join("Show", "good.mp3")]; ok {
		t.Errorf("expected good.mp3 to be removed from done manifest after pull")
	}
	if _, ok := doneM.Episodes[filepath.Join("Show", "bad.mp3")]; !ok {
		t.Errorf("expected bad.mp3 to remain in done manifest for retry")
	}
	if _, ok := doneM.Episodes[filepath.Join("Show", "empty.mp3")]; !ok {
		t.Errorf("expected empty.mp3 to remain in done manifest")
	}
}

func TestPIDLockAcquisitionAndStaleRecovery(t *testing.T) {
	tempDir := t.TempDir()
	lockPath := filepath.Join(tempDir, ".worker.lock")

	unlock1, err := acquireWorkerLock(tempDir)
	if err != nil {
		t.Fatalf("failed to acquire worker lock on clean dir: %v", err)
	}
	if !fileExists(lockPath) {
		t.Fatalf("expected lockfile to exist after acquire")
	}

	unlock2, err := acquireWorkerLock(tempDir)
	if err != nil {
		t.Fatalf("expected re-acquiring lock from same PID to succeed, got error: %v", err)
	}
	unlock2()
	unlock1()

	externalFlock := flock.New(lockPath)
	locked, err := externalFlock.TryLock()
	if err != nil || !locked {
		t.Fatalf("failed to simulate external worker lock: %v", err)
	}

	_, err = acquireWorkerLock(tempDir)
	if err == nil {
		t.Fatalf("expected error when external lock is held, got nil")
	}

	if err := externalFlock.Unlock(); err != nil {
		t.Fatalf("failed to unlock external lock: %v", err)
	}

	unlockRecovered, err := acquireWorkerLock(tempDir)
	if err != nil {
		t.Fatalf("expected acquiring lock after external release to succeed, got: %v", err)
	}
	unlockRecovered()
}

func TestDirtyFlagScanTrigger(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteDir, "DirtyPod")
	_ = os.MkdirAll(podDir, 0755)

	audioFile := filepath.Join(podDir, "ep.mp3")
	_ = os.WriteFile(audioFile, []byte("fake mp3 audio"), 0644)
	transcriptJSON := filepath.Join(podDir, "ep.transcript.json")
	saveJSONTranscript(audioFile, &TranscriptionData{Text: "Test text"}, transcriptJSON, true, nil)

	cfg := &Config{RemoteWorkDir: remoteDir}

	if err := runRemoteScan(cfg, remoteDir, true, true, false); err != nil {
		t.Fatalf("runRemoteScan with ifDirty clean failed: %v", err)
	}

	donePath := filepath.Join(remoteDir, "done.json")
	if fileExists(donePath) {
		t.Errorf("expected no scan execution when ifDirty is true and .scan_trigger does not exist")
	}

	triggerFile := filepath.Join(remoteDir, ".scan_trigger")
	_ = os.WriteFile(triggerFile, []byte{}, 0644)

	if err := runRemoteScan(cfg, remoteDir, true, true, false); err != nil {
		t.Fatalf("runRemoteScan with ifDirty trigger failed: %v", err)
	}

	if fileExists(triggerFile) {
		t.Errorf("expected .scan_trigger to be removed after scan")
	}

	if !fileExists(donePath) {
		t.Errorf("expected done.json to be created after scanning dirty dir")
	}

	audioFile2 := filepath.Join(podDir, "ep2.mp3")
	_ = os.WriteFile(audioFile2, []byte("fake mp3 audio 2"), 0644)
	transcriptJSON2 := filepath.Join(podDir, "ep2.transcript.json")
	saveJSONTranscript(audioFile2, &TranscriptionData{Text: "Test text 2"}, transcriptJSON2, true, nil)

	if err := runRemoteScan(cfg, remoteDir, false, true, false); err != nil {
		t.Fatalf("runRemoteScan with ifDirty=false failed: %v", err)
	}

	doneM, err := loadDoneManifest(donePath)
	if err != nil || len(doneM.Episodes) != 2 {
		t.Errorf("expected 2 processed episodes in done.json, got %d (err: %v)", len(doneM.Episodes), err)
	}
}

func TestBatchRemoteAckAndFullDeletion(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteDir, "BatchPod")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	ep2 := filepath.Join(podDir, "ep2.mp3")
	_ = os.WriteFile(ep1, []byte("audio 1"), 0644)
	_ = os.WriteFile(ep1+".precut", []byte("precut 1"), 0644)
	_ = os.WriteFile(ep1+".tmp.mp3", []byte("tmp 1"), 0644)
	_ = os.WriteFile(ep2, []byte("audio 2"), 0644)
	_ = os.WriteFile(ep2+".precut", []byte("precut 2"), 0644)

	stat1 := filepath.Join(podDir, "ep1.mp3.json")
	stat2 := filepath.Join(podDir, "ep2.mp3.json")
	_ = saveEpisodeStatus(stat1, &EpisodeStatusFile{MediaFile: "ep1.mp3", Status: StateReadyForCopyBack})
	_ = saveEpisodeStatus(stat2, &EpisodeStatusFile{MediaFile: "ep2.mp3", Status: StateReadyForCopyBack})

	donePath := filepath.Join(remoteDir, "done.json")
	rel1 := filepath.Join("BatchPod", "ep1.mp3")
	rel2 := filepath.Join("BatchPod", "ep2.mp3")
	_ = addDoneEpisode(donePath, RemoteDoneItem{RelPath: rel1, Status: StateReadyForCopyBack})
	_ = addDoneEpisode(donePath, RemoteDoneItem{RelPath: rel2, Status: StateReadyForCopyBack})

	if err := runRemoteAck(remoteDir, []string{rel1, rel2}); err != nil {
		t.Fatalf("runRemoteAck failed: %v", err)
	}

	if fileExists(ep1) || fileExists(ep1+".precut") || fileExists(ep1+".tmp.mp3") {
		t.Errorf("expected ep1 and precut/tmp files to be fully deleted")
	}
	if fileExists(ep2) || fileExists(ep2+".precut") {
		t.Errorf("expected ep2 and precut files to be fully deleted")
	}

	st1, _ := loadEpisodeStatus(stat1)
	if st1 == nil || st1.Status != StateArchived {
		t.Errorf("expected ep1 status to be StateArchived, got %+v", st1)
	}
	st2, _ := loadEpisodeStatus(stat2)
	if st2 == nil || st2.Status != StateArchived {
		t.Errorf("expected ep2 status to be StateArchived, got %+v", st2)
	}

	doneM, _ := loadDoneManifest(donePath)
	if len(doneM.Episodes) != 0 {
		t.Errorf("expected done.json to be empty after ack, got %d items", len(doneM.Episodes))
	}

	archPath := filepath.Join(remoteDir, "archive.json")
	archM, err := loadDoneManifest(archPath)
	if err != nil || len(archM.Episodes) != 2 {
		t.Errorf("expected 2 items in archive.json, got %d (err: %v)", len(archM.Episodes), err)
	}
}
