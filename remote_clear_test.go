package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRemoteClear(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	remoteWorkDir := filepath.Join(tempDir, "remote_work")
	_ = os.MkdirAll(filepath.Join(remoteWorkDir, "PodcastA"), 0755)

	ep1 := filepath.Join(remoteWorkDir, "PodcastA", "ep1.mp3")
	ep1Stat := filepath.Join(remoteWorkDir, "PodcastA", "ep1.mp3.json")
	_ = os.WriteFile(ep1, []byte("audio"), 0644)
	_ = saveEpisodeStatus(ep1Stat, &EpisodeStatusFile{
		Status: StateAwaitingTranscription,
	})

	localDir := filepath.Join(tempDir, "local_podcasts", "PodcastA")
	_ = os.MkdirAll(localDir, 0755)
	localEp1 := filepath.Join(localDir, "ep1.mp3")
	_ = os.WriteFile(localEp1, []byte("audio"), 0644)
	_ = saveEpisodeStatus(statusPathFor(localEp1), &EpisodeStatusFile{
		Status: StateQueuedRemote,
	})

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   filepath.Join(tempDir, "local_podcasts"),
	}

	err := runRemoteClear(&cfg, "mock-host", mock, false)
	if err != nil {
		t.Fatalf("runRemoteClear failed: %v", err)
	}

	stat := getOrCreateEpisodeStatus(localEp1)
	if stat.Status != StateDownloaded {
		t.Errorf("expected local status to be reset to downloaded, got: %s", stat.Status)
	}
}

func TestRunRemoteClearNoHost(t *testing.T) {
	cfg := Config{}
	err := runRemoteClear(&cfg, "", nil, true)
	if err == nil {
		t.Error("expected error when no host configured or provided")
	}
}
