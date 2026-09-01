package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// An unplayable track must not silently consume the rest of the queue.
func TestUnplayableTrackDoesNotDrainTheQueue(t *testing.T) {
	orig := playerSpawnEnabled
	playerSpawnEnabled = true
	t.Cleanup(func() { playerSpawnEnabled = orig })

	missing := filepath.Join(t.TempDir(), "gone")
	p := &AudioPlayer{Volume: 70}
	p.Queue = []PlayerTrack{
		{Title: "Ep 2", Path: missing + "-2.mp3", Duration: 1800},
		{Title: "Ep 3", Path: missing + "-3.mp3", Duration: 1800},
		{Title: "Ep 4", Path: missing + "-4.mp3", Duration: 1800},
	}
	t.Cleanup(p.Stop)

	p.PlayTrack(PlayerTrack{Title: "Ep 1", Path: missing + "-1.mp3", Duration: 1800})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		playing := p.IsPlaying
		p.mu.Unlock()
		if !playing {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	p.mu.Lock()
	remaining := len(p.Queue)
	lastErr := p.LastError
	playing := p.IsPlaying
	p.mu.Unlock()

	if remaining != 3 {
		t.Errorf("queue was drained: %d of 3 tracks remain after one unplayable track", remaining)
	}
	if playing {
		t.Errorf("IsPlaying is still true after playback failed")
	}
	if lastErr == "" {
		t.Errorf("playback failed with no error recorded; the user is told nothing")
	}
}

func TestUnplayableTrackDoesNotPersistAnEmptiedQueue(t *testing.T) {
	orig := playerSpawnEnabled
	playerSpawnEnabled = true
	t.Cleanup(func() { playerSpawnEnabled = orig })

	missing := filepath.Join(t.TempDir(), "gone")
	p := &AudioPlayer{Volume: 70}
	p.Queue = []PlayerTrack{
		{Title: "Ep 2", Path: missing + "-2.mp3", Duration: 1800},
		{Title: "Ep 3", Path: missing + "-3.mp3", Duration: 1800},
	}
	t.Cleanup(p.Stop)

	p.PlayTrack(PlayerTrack{Title: "Ep 1", Path: missing + "-1.mp3", Duration: 1800})
	time.Sleep(600 * time.Millisecond)

	data, err := os.ReadFile(getPlayQueueFilePath())
	if err != nil {
		t.Skipf("no play queue file was written: %v", err)
	}
	var persisted PlayQueuePersist
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("play_queue.json is not valid JSON: %v", err)
	}
	if len(persisted.Queue) != 2 {
		t.Errorf("persisted queue holds %d tracks, want 2; the drain was written to disk",
			len(persisted.Queue))
	}
}
