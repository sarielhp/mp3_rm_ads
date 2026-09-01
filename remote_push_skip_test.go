package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemotePushSkipInFlight(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	podcastsDir := filepath.Join(tempDir, "podcasts")
	_ = os.MkdirAll(podcastsDir, 0755)
	ep1 := filepath.Join(podcastsDir, "test1.mp3")
	ep2 := filepath.Join(podcastsDir, "test2.mp3")
	_ = os.WriteFile(ep1, []byte("fake audio 1"), 0644)
	_ = os.WriteFile(ep2, []byte("fake audio 2"), 0644)

	st1 := getOrCreateEpisodeStatus(ep1)
	st1.Status = StateQueuedRemote
	_ = saveEpisodeStatus(statusPathFor(ep1), st1)

	st2 := getOrCreateEpisodeStatus(ep2)
	st2.Status = StateDone
	_ = saveEpisodeStatus(statusPathFor(ep2), st2)

	if !isEpisodeInRemoteFlight(ep1) {
		t.Errorf("expected isEpisodeInRemoteFlight to be true for StateQueuedRemote")
	}
	if !isEpisodeCompleted(ep2) {
		t.Errorf("expected isEpisodeCompleted to be true for StateDone")
	}

	cfg := &Config{
		RemoteHost:    "push-box",
		RemoteWorkDir: "~/.abs_remote",
		PodcastsDir:   podcastsDir,
	}

	err := runRemotePush(cfg, nil, "push-box", mock, 0, true, false)
	if err != nil {
		t.Fatalf("expected runRemotePush to return nil, got: %v", err)
	}

	if len(mock.ExecutedCmds) > 0 {
		t.Errorf("expected 0 executed commands since files are already queued or done, got: %d", len(mock.ExecutedCmds))
	}
}
