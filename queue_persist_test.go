package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlayQueuePersistence(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	p := &AudioPlayer{}
	track1 := PlayerTrack{Title: "Ep 1", Path: "ep1.mp3", Duration: 100}
	track2 := PlayerTrack{Title: "Ep 2", Path: "ep2.mp3", Duration: 200}
	track3 := PlayerTrack{Title: "Ep 3", Path: "ep3.mp3", Duration: 300}

	p.IsPlaying = true
	p.Current = &track1
	p.EnqueueAndPlay(track2)
	p.EnqueueAndPlay(track3)

	p.SaveQueueToFile()

	queueFile := filepath.Join(tempDir, ".config", "abs", "play_queue.json")
	if _, err := os.Stat(queueFile); err != nil {
		t.Fatalf("expected play_queue.json at %s, got error: %v", queueFile, err)
	}

	p2 := &AudioPlayer{}
	p2.LoadQueueFromFile()

	if len(p2.Queue) != 2 {
		t.Fatalf("expected 2 items loaded in p2.Queue, got %d", len(p2.Queue))
	}
	if p2.Queue[0].Title != "Ep 2" || p2.Queue[1].Title != "Ep 3" {
		t.Errorf("unexpected loaded queue order: %+v", p2.Queue)
	}
}

func TestPlayQueueMoveAndRemove(t *testing.T) {
	p := &AudioPlayer{}
	track1 := PlayerTrack{Title: "A", Path: "a.mp3"}
	track2 := PlayerTrack{Title: "B", Path: "b.mp3"}
	track3 := PlayerTrack{Title: "C", Path: "c.mp3"}

	p.Queue = []PlayerTrack{track1, track2, track3}

	ok := p.MoveUnifiedItem(0, 2)
	if !ok || len(p.Queue) != 3 || p.Queue[0].Title != "B" || p.Queue[2].Title != "A" {
		t.Fatalf("unexpected queue after MoveUnifiedItem(0, 2): %+v", p.Queue)
	}

	ok = p.RemoveUnifiedItem(1)
	if !ok || len(p.Queue) != 2 || p.Queue[0].Title != "B" || p.Queue[1].Title != "A" {
		t.Fatalf("unexpected queue after RemoveUnifiedItem(1): %+v", p.Queue)
	}

	current := PlayerTrack{Title: "Now Playing", Path: "now.mp3"}
	p.Current = &current
	unified := p.GetUnifiedQueue()
	if len(unified) != 3 || !unified[0].IsCurrent || unified[0].Track.Title != "Now Playing" {
		t.Fatalf("unexpected GetUnifiedQueue: %+v", unified)
	}

	ok = p.RemoveUnifiedItem(1)
	if !ok || len(p.Queue) != 1 || p.Queue[0].Title != "A" {
		t.Fatalf("unexpected queue after RemoveUnifiedItem(1) with current: %+v", p.Queue)
	}
}

func TestAdQueueHelpers(t *testing.T) {
	pods := []tuiPodcast{
		{
			name: "Tech Show",
			dir:  "/podcasts/tech",
			episodes: []tuiEpisode{
				{filename: "ep1.mp3", title: "Episode One", hasAdsRemoved: true},
				{filename: "ep2.mp3", title: "Episode Two", hasAdsRemoved: false},
				{filename: "ep3.mp3", title: "Episode Three", hasAdsRemoved: false},
			},
		},
	}
	q := map[string][]string{
		"/podcasts/tech": {"ep1.mp3", "ep2.mp3", "ep3.mp3"},
	}

	items := getAllAdQueueItems(pods, q)
	if len(items) != 2 {
		t.Fatalf("expected 2 ad queue items (ep1 excluded), got %d", len(items))
	}
	if items[0].Title != "Episode Two" || items[0].HasAdsRemoved {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1].Title != "Episode Three" || items[1].HasAdsRemoved {
		t.Errorf("unexpected second item: %+v", items[1])
	}

	var savedDir string
	var savedEntries []string
	saveFn := func(dir string, entries []string) {
		savedDir = dir
		savedEntries = entries
	}

	removeAdQueueItem(items[0], q, saveFn)
	if savedDir != "/podcasts/tech" || len(savedEntries) != 2 || savedEntries[0] != "ep1.mp3" || savedEntries[1] != "ep3.mp3" {
		t.Errorf("unexpected saved after remove: %v, %v", savedDir, savedEntries)
	}
}
