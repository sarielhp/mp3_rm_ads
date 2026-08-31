package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteMirrorScanAndDoneJSON(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteDir, "Science_Podcast")
	_ = os.MkdirAll(podDir, 0755)

	audioFile := filepath.Join(podDir, "episode1.mp3")
	_ = os.WriteFile(audioFile, []byte("fake mp3 data"), 0644)

	transcriptJSON := filepath.Join(podDir, "episode1.transcript.json")
	dummyTranscript := TranscriptionData{
		Text: "Welcome to science podcast today.",
		Segments: []TranscriptionSegment{
			{Start: 0.0, End: 10.0, Text: "Welcome to science podcast today."},
		},
	}
	saveJSONTranscript(audioFile, &dummyTranscript, transcriptJSON, true, nil)

	cfg := &Config{
		RemoteWorkDir: remoteDir,
	}

	err := runRemoteScan(cfg, remoteDir, true, false)
	if err != nil {
		t.Fatalf("runRemoteScan failed: %v", err)
	}

	statFile := statusPathFor(audioFile)
	st, err := loadEpisodeStatus(statFile)
	if err != nil {
		t.Fatalf("failed to load episode status %s: %v", statFile, err)
	}
	if st.Status != StateReadyForCopyBack {
		t.Errorf("expected status %s, got %s", StateReadyForCopyBack, st.Status)
	}

	donePath := filepath.Join(remoteDir, "done.json")
	doneM, err := loadDoneManifest(donePath)
	if err != nil {
		t.Fatalf("failed to load done manifest %s: %v", donePath, err)
	}
	relExpected := filepath.Join("Science_Podcast", "episode1.mp3")
	if _, ok := doneM.Episodes[relExpected]; !ok {
		t.Errorf("expected %s in done manifest, got %+v", relExpected, doneM.Episodes)
	}
}

func TestRemoteAckAndZeroTruncate(t *testing.T) {
	tempDir := t.TempDir()
	remoteDir := filepath.Join(tempDir, "abs_remote")
	podDir := filepath.Join(remoteDir, "News_Show")
	_ = os.MkdirAll(podDir, 0755)

	audioFile := filepath.Join(podDir, "news.mp3")
	_ = os.WriteFile(audioFile, []byte("raw audio data bytes here"), 0644)
	_ = os.WriteFile(audioFile+".precut", []byte("precut backup"), 0644)

	statFile := statusPathFor(audioFile)
	st := &EpisodeStatusFile{
		Version:   1,
		MediaFile: "news.mp3",
		Status:    StateReadyForCopyBack,
	}
	_ = saveEpisodeStatus(statFile, st)

	donePath := filepath.Join(remoteDir, "done.json")
	relPath := filepath.Join("News_Show", "news.mp3")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath: relPath,
		Status:  StateReadyForCopyBack,
	})

	err := runRemoteAck(remoteDir, relPath)
	if err != nil {
		t.Fatalf("runRemoteAck failed: %v", err)
	}

	fi, err := os.Stat(audioFile)
	if err != nil {
		t.Fatalf("expected audio file to exist after ack: %v", err)
	}
	if fi.Size() != 0 {
		t.Errorf("expected audio file to be truncated to 0 bytes, got size %d", fi.Size())
	}

	if fileExists(audioFile + ".precut") {
		t.Errorf("expected precut file to be removed after ack")
	}

	stAfter, _ := loadEpisodeStatus(statFile)
	if stAfter.Status != StateArchived {
		t.Errorf("expected archived status, got %s", stAfter.Status)
	}

	doneM, _ := loadDoneManifest(donePath)
	if len(doneM.Episodes) != 0 {
		t.Errorf("expected done manifest to be empty, got %d", len(doneM.Episodes))
	}

	archPath := filepath.Join(remoteDir, "archive.json")
	archM, err := loadDoneManifest(archPath)
	if err != nil || len(archM.Episodes) != 1 {
		t.Fatalf("expected 1 item in archive.json, got %d (err: %v)", len(archM.Episodes), err)
	}
}

func TestRemotePullFromMirror(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	remotePodDir := filepath.Join(remoteWorkDir, "My_Pod")
	_ = os.MkdirAll(remotePodDir, 0755)

	remAudio := filepath.Join(remotePodDir, "show.mp3")
	remCuts := filepath.Join(remotePodDir, "show.cuts.json")
	remTrans := filepath.Join(remotePodDir, "show.transcript.json")
	remStat := filepath.Join(remotePodDir, "show.mp3.json")

	_ = os.WriteFile(remAudio, []byte("clean audio content"), 0644)
	_ = os.WriteFile(remCuts, []byte(`{"cut_intervals":[]}`), 0644)
	_ = os.WriteFile(remTrans, []byte(`{"text":"transcript text"}`), 0644)
	st := &EpisodeStatusFile{
		Version:   1,
		MediaFile: "show.mp3",
		Status:    StateReadyForCopyBack,
	}
	_ = saveEpisodeStatus(remStat, st)

	relPath := filepath.Join("My_Pod", "show.mp3")
	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:            relPath,
		Status:             StateReadyForCopyBack,
		CleanedDurationSec: 120.0,
		CutDurationSec:     30.0,
	})

	cfg := &Config{
		RemoteHost:    "mock-box",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	err := runRemotePull(cfg, "mock-box", mock, true, false)
	if err != nil {
		t.Fatalf("runRemotePull failed: %v", err)
	}

	localAudio := filepath.Join(localPodcasts, relPath)
	data, err := os.ReadFile(localAudio)
	if err != nil || string(data) != "clean audio content" {
		t.Errorf("expected clean audio pulled to %s, got: %s (err: %v)", localAudio, string(data), err)
	}

	localStat, err := loadEpisodeStatus(statusPathFor(localAudio))
	if err != nil || localStat.Status != StateDone {
		t.Errorf("expected local status StateDone, got %+v (err: %v)", localStat, err)
	}
}
