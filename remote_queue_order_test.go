package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSortAudioFilesByDurationAndPriority(t *testing.T) {
	tempDir := t.TempDir()
	epLong := filepath.Join(tempDir, "long.mp3")
	epShort := filepath.Join(tempDir, "short.mp3")
	epMedium := filepath.Join(tempDir, "medium.mp3")
	epUrgentLong := filepath.Join(tempDir, "urgent_long.mp3")

	for _, p := range []string{epLong, epShort, epMedium, epUrgentLong} {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}

	_ = saveEpisodeStatus(statusPathFor(epLong), &EpisodeStatusFile{
		MediaFile: "long.mp3", Priority: 0, Original: EpisodeAudioMeta{DurationSec: 3600.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epShort), &EpisodeStatusFile{
		MediaFile: "short.mp3", Priority: 0, Original: EpisodeAudioMeta{DurationSec: 120.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epMedium), &EpisodeStatusFile{
		MediaFile: "medium.mp3", Priority: 0, Original: EpisodeAudioMeta{DurationSec: 600.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epUrgentLong), &EpisodeStatusFile{
		MediaFile: "urgent_long.mp3", Priority: 10, Original: EpisodeAudioMeta{DurationSec: 5000.0},
	})

	files := []string{epLong, epMedium, epShort, epUrgentLong}
	sortAudioFilesByDuration(files)

	if len(files) != 4 || files[0] != epUrgentLong || files[1] != epShort || files[2] != epMedium || files[3] != epLong {
		t.Errorf("unexpected sorted order: %+v", files)
	}
}

func TestSortAudioFilesByFileSizeFallback(t *testing.T) {
	tempDir := t.TempDir()
	epLarge := filepath.Join(tempDir, "large.mp3")
	epSmall := filepath.Join(tempDir, "small.mp3")

	_ = os.WriteFile(epLarge, []byte("large content payload here 1234567890"), 0644)
	_ = os.WriteFile(epSmall, []byte("small"), 0644)

	files := []string{epLarge, epSmall}
	sortAudioFilesByDuration(files)

	if files[0] != epSmall || files[1] != epLarge {
		t.Errorf("expected small file before large file on size fallback, got %+v", files)
	}
}

type OrderTrackingMockTransport struct {
	*MockRemoteTransport
	UploadedFiles []string
}

func (m *OrderTrackingMockTransport) Upload(host, localSrc, remoteDst string) error {
	if filepath.Ext(localSrc) == ".mp3" {
		m.UploadedFiles = append(m.UploadedFiles, filepath.Base(localSrc))
	}
	return m.MockRemoteTransport.Upload(host, localSrc, remoteDst)
}

func TestRemotePushUploadOrderAndPriority(t *testing.T) {
	tempDir := t.TempDir()
	baseMock := NewMockRemoteTransport(tempDir)
	mock := &OrderTrackingMockTransport{MockRemoteTransport: baseMock}

	podcastsDir := filepath.Join(tempDir, "podcasts")
	_ = os.MkdirAll(podcastsDir, 0755)

	epLong := filepath.Join(podcastsDir, "long.mp3")
	epShort := filepath.Join(podcastsDir, "short.mp3")
	epMedium := filepath.Join(podcastsDir, "medium.mp3")

	for _, p := range []string{epLong, epShort, epMedium} {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}

	_ = saveEpisodeStatus(statusPathFor(epLong), &EpisodeStatusFile{
		MediaFile: "long.mp3", Original: EpisodeAudioMeta{DurationSec: 3600.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epShort), &EpisodeStatusFile{
		MediaFile: "short.mp3", Original: EpisodeAudioMeta{DurationSec: 60.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epMedium), &EpisodeStatusFile{
		MediaFile: "medium.mp3", Original: EpisodeAudioMeta{DurationSec: 300.0},
	})

	cfg := &Config{RemoteHost: "push-box", RemoteWorkDir: "~/.abs_remote", PodcastsDir: podcastsDir}

	err := runRemotePush(cfg, []string{epLong, epMedium, epShort}, "push-box", mock, 0, true, false)
	if err != nil {
		t.Fatalf("runRemotePush failed: %v", err)
	}

	if len(mock.UploadedFiles) != 3 || mock.UploadedFiles[0] != "short.mp3" || mock.UploadedFiles[1] != "medium.mp3" || mock.UploadedFiles[2] != "long.mp3" {
		t.Errorf("expected upload order [short.mp3, medium.mp3, long.mp3], got %+v", mock.UploadedFiles)
	}

	epPrio := filepath.Join(podcastsDir, "prio.mp3")
	_ = os.WriteFile(epPrio, []byte("prio"), 0644)
	err = runRemotePush(cfg, []string{epPrio}, "push-box", mock, 7, true, false)
	if err != nil {
		t.Fatalf("runRemotePush with priority failed: %v", err)
	}
	st, err := loadEpisodeStatus(statusPathFor(epPrio))
	if err != nil || st.Priority != 7 {
		t.Errorf("expected priority 7, got %+v (err: %v)", st, err)
	}
}

func TestRemoteScanProcessesShorterFirst(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteDir, "Show")
	_ = os.MkdirAll(podDir, 0755)

	epLong := filepath.Join(podDir, "long.mp3")
	epShort := filepath.Join(podDir, "short.mp3")
	epMed := filepath.Join(podDir, "med.mp3")

	for _, p := range []string{epLong, epShort, epMed} {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}

	_ = saveEpisodeStatus(statusPathFor(epLong), &EpisodeStatusFile{
		MediaFile: "long.mp3", Status: StateAwaitingTranscription, Original: EpisodeAudioMeta{DurationSec: 3600.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epShort), &EpisodeStatusFile{
		MediaFile: "short.mp3", Status: StateAwaitingTranscription, Original: EpisodeAudioMeta{DurationSec: 60.0},
	})
	_ = saveEpisodeStatus(statusPathFor(epMed), &EpisodeStatusFile{
		MediaFile: "med.mp3", Status: StateAwaitingTranscription, Original: EpisodeAudioMeta{DurationSec: 300.0},
	})

	dummyTranscript := TranscriptionData{
		Text: "Test", Segments: []TranscriptionSegment{{Start: 0.0, End: 5.0, Text: "Test"}},
	}
	saveJSONTranscript(epLong, &dummyTranscript, filepath.Join(podDir, "long.transcript.json"), true, nil)
	saveJSONTranscript(epShort, &dummyTranscript, filepath.Join(podDir, "short.transcript.json"), true, nil)
	saveJSONTranscript(epMed, &dummyTranscript, filepath.Join(podDir, "med.transcript.json"), true, nil)

	cfg := &Config{RemoteWorkDir: remoteDir}
	err := runRemoteScan(cfg, remoteDir, false, true, false)
	if err != nil {
		t.Fatalf("runRemoteScan failed: %v", err)
	}

	donePath := filepath.Join(remoteDir, "done.json")
	doneM, err := loadDoneManifest(donePath)
	if err != nil || len(doneM.Episodes) != 3 {
		t.Fatalf("expected 3 finished episodes in done manifest, got %+v (err: %v)", doneM, err)
	}
}

func TestBatchWorkerSortsByPriorityAndDuration(t *testing.T) {
	tempDir := t.TempDir()
	batchDir := filepath.Join(tempDir, "batch_work")
	inDir := filepath.Join(batchDir, "in")
	outDir := filepath.Join(batchDir, "out")
	_ = os.MkdirAll(inDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	epLong := filepath.Join(inDir, "long.mp3")
	epShort := filepath.Join(inDir, "short.mp3")
	epUrgent := filepath.Join(inDir, "urgent.mp3")
	for _, p := range []string{epLong, epShort, epUrgent} {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}

	manifest := RemoteBatchManifest{
		BatchID:    "test-batch-sort",
		CreatedAt:  "2026-08-30T12:00:00Z",
		Status:     BatchStatusQueued,
		TotalItems: 3,
		Items: []RemoteBatchJobItem{
			{
				ID: "item-long", SourceFile: epLong, AudioFileName: "long.mp3",
				OriginalDurationSec: 3600.0, Priority: 0, Status: BatchStatusQueued,
			},
			{
				ID: "item-short", SourceFile: epShort, AudioFileName: "short.mp3",
				OriginalDurationSec: 60.0, Priority: 0, Status: BatchStatusQueued,
			},
			{
				ID: "item-urgent", SourceFile: epUrgent, AudioFileName: "urgent.mp3",
				OriginalDurationSec: 5000.0, Priority: 10, Status: BatchStatusQueued,
			},
		},
	}
	_ = saveManifest(filepath.Join(batchDir, "manifest.json"), &manifest)

	dummyTranscript := TranscriptionData{
		Text: "Hello", Segments: []TranscriptionSegment{{Start: 0.0, End: 2.0, Text: "Hello"}},
	}
	saveJSONTranscript(filepath.Join(outDir, "long.mp3"), &dummyTranscript, filepath.Join(outDir, "long.transcript.json"), true, nil)
	saveJSONTranscript(filepath.Join(outDir, "short.mp3"), &dummyTranscript, filepath.Join(outDir, "short.transcript.json"), true, nil)
	saveJSONTranscript(filepath.Join(outDir, "urgent.mp3"), &dummyTranscript, filepath.Join(outDir, "urgent.transcript.json"), true, nil)

	err := runBatchWorker(batchDir, true, false)
	if err != nil {
		t.Fatalf("runBatchWorker failed: %v", err)
	}

	updated, err := loadManifest(filepath.Join(batchDir, "manifest.json"))
	if err != nil || len(updated.Items) != 3 {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if updated.Items[0].AudioFileName != "urgent.mp3" || updated.Items[1].AudioFileName != "short.mp3" || updated.Items[2].AudioFileName != "long.mp3" {
		t.Errorf("expected manifest items sorted [urgent, short, long], got %+v", updated.Items)
	}
}

func TestRemoteStatusShowsActiveAndQueueDuration(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	remoteWorkDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteWorkDir, "Show")
	_ = os.MkdirAll(podDir, 0755)

	activeEp := filepath.Join(podDir, "active.mp3")
	queuedEp := filepath.Join(podDir, "queued.mp3")
	_ = os.WriteFile(activeEp, []byte("active"), 0644)
	_ = os.WriteFile(queuedEp, []byte("queued"), 0644)

	_ = saveEpisodeStatus(statusPathFor(activeEp), &EpisodeStatusFile{
		MediaFile: "active.mp3", Status: StateTranscribingRemotely, Original: EpisodeAudioMeta{DurationSec: 1500.0},
	})
	_ = saveEpisodeStatus(statusPathFor(queuedEp), &EpisodeStatusFile{
		MediaFile: "queued.mp3", Status: StateAwaitingTranscription, Original: EpisodeAudioMeta{DurationSec: 300.0},
	})

	cfg := &Config{RemoteHost: "stat-box", RemoteWorkDir: remoteWorkDir}
	err := runRemoteStatus(cfg, "stat-box", mock, false, false)
	if err != nil {
		t.Fatalf("runRemoteStatus failed: %v", err)
	}
}
