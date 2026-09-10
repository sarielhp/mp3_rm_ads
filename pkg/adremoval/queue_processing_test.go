package adremoval

import (
	"abs/pkg/pipeline"
	"abs/pkg/util"
	"os"
	"path/filepath"
	"testing"
)

func TestQueueRemoteHostSelection(t *testing.T) {
	cfg := Config{}
	cfg.RemoteHost = "other"
	host, err := resolveRemoteProcessingTargetHost(ProcOptions{Remote: true, RemoteHost: "chosen"}, cfg)
	if err != nil || host != "chosen" {
		t.Fatalf("host=%q error=%v", host, err)
	}
	if _, err := resolveRemoteProcessingTargetHost(ProcOptions{Remote: true}, Config{}); err == nil {
		t.Fatal("missing remote host accepted")
	}
	host, err = resolveRemoteProcessingTargetHost(ProcOptions{Local: true, RemoteHost: "chosen"}, Config{})
	if err != nil || host != "" {
		t.Fatalf("local host=%q error=%v", host, err)
	}
}

func TestQueuedCompletedEpisodeWithoutTranscriptIsNotSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(path), &EpisodeStatusFile{Status: StateDone}); err != nil {
		t.Fatal(err)
	}
	lock, process, stop := checkSkipOrLockAudioFile(path, path, 0, 1, 0, ProcOptions{Quiet: true})
	if lock != nil {
		defer lock.Release()
	}
	if !process || stop {
		t.Fatal("stale done status caused missing-transcript episode to be skipped")
	}
}

func TestQueueRetainsLockedEpisode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	pipeline.AddToQueue(dir, "episode.mp3")
	lock, err := util.AcquireFileLock(path)
	if err != nil || lock == nil {
		t.Fatalf("lock: %v", err)
	}
	defer lock.Release()
	opts := ProcOptions{Quiet: true, Local: true, WhisperEngine: "local"}
	if err := ProcessQueuedTarget(dir, path, "rm_ads", opts, Config{}); err == nil {
		t.Fatal("skipped processing reported success")
	}
	if got := pipeline.QueuedEpisodes(dir); len(got) != 1 || got[0] != "episode.mp3" {
		t.Fatalf("queue = %v, want episode retained", got)
	}
}
