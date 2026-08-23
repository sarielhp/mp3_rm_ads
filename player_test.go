package main

import (
	"strings"
	"testing"
)

func TestFormatPlayerTime(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "00:00"},
		{45, "00:45"},
		{125, "02:05"},
		{3665, "1:01:05"},
		{7320, "2:02:00"},
	}

	for _, tt := range tests {
		got := formatPlayerTime(tt.in)
		if got != tt.want {
			t.Errorf("formatPlayerTime(%.1f) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderProgressBar(t *testing.T) {
	p := &AudioPlayer{
		Position: 60,
		Duration: 120,
	}

	bar := p.RenderProgressBar(40)
	if !strings.Contains(bar, "01:00 / 02:00") {
		t.Errorf("expected time string '01:00 / 02:00', got %q", bar)
	}
	if !strings.HasPrefix(bar, "[") {
		t.Errorf("expected progress bar to start with '[', got %q", bar)
	}
}

func TestRenderVolumeBar(t *testing.T) {
	p := &AudioPlayer{
		Volume: 70,
		Muted:  false,
	}

	bar := p.RenderVolumeBar(20)
	if !strings.Contains(bar, "70%") {
		t.Errorf("expected volume bar to contain '70%%', got %q", bar)
	}

	p.Muted = true
	barMuted := p.RenderVolumeBar(20)
	if !strings.Contains(barMuted, "MUTED") {
		t.Errorf("expected volume bar to show MUTED, got %q", barMuted)
	}
}

func TestPlayerQueueManagement(t *testing.T) {
	p := &AudioPlayer{}
	track1 := PlayerTrack{Title: "Ep 1", Path: "ep1.mp3", Duration: 100}
	track2 := PlayerTrack{Title: "Ep 2", Path: "ep2.mp3", Duration: 200}
	track3 := PlayerTrack{Title: "Ep 3", Path: "ep3.mp3", Duration: 300}

	p.IsPlaying = true
	p.Current = &track1
	ok1 := p.EnqueueAndPlay(track2)
	ok2 := p.EnqueueAndPlay(track3)
	if !ok1 || !ok2 {
		t.Error("expected tracks to be added")
	}

	// Try adding duplicates: currently playing track1 and queued track2
	dup1 := p.EnqueueAndPlay(track1)
	dup2 := p.EnqueueAndPlay(track2)
	if dup1 || dup2 {
		t.Error("expected duplicate tracks to be rejected")
	}

	if len(p.Queue) != 2 {
		t.Fatalf("expected 2 items in queue, got %d", len(p.Queue))
	}
	if p.Queue[0].Title != "Ep 2" || p.Queue[1].Title != "Ep 3" {
		t.Errorf("unexpected queue order: %+v", p.Queue)
	}

	p.ClearQueue()
	if len(p.Queue) != 0 {
		t.Errorf("expected empty queue after ClearQueue, got %d", len(p.Queue))
	}
}

func TestPlayerSeekBoundaries(t *testing.T) {
	track := PlayerTrack{Duration: 100}
	p := &AudioPlayer{
		Current:  &track,
		Position: 50,
		Duration: 100,
		IsPaused: true,
	}

	p.Seek(30)
	if p.Position != 80 {
		t.Errorf("expected position 80, got %.1f", p.Position)
	}

	p.Seek(50) // exceeds duration
	if p.Position != 100 {
		t.Errorf("expected position clamped to 100, got %.1f", p.Position)
	}

	p.Seek(-200) // below 0
	if p.Position != 0 {
		t.Errorf("expected position clamped to 0, got %.1f", p.Position)
	}
}
