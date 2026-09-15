package tui

import (
	"testing"
)

// The TUI writes its episode model into the podcast cache and reads it back on
// the next launch. The two halves are hand-written and were never checked
// against each other, so a field could be written and silently never restored.
// This pins which fields actually survive the round trip.
func TestCachedSummaryRoundTrip(t *testing.T) {
	original := tuiEpisode{
		filename:      "ep1.mp3",
		path:          "/lib/Show/ep1.mp3",
		title:         "Episode One",
		publishedAt:   1600000000000,
		duration:      1234.5,
		fileSize:      999,
		season:        "2",
		episode:       "7",
		hasTranscript: true,
		hasAdsRemoved: true,
	}

	sum := buildCachedEpisodeSummary(original, t.TempDir(), nil)

	var restored tuiEpisode
	applyCachedSummaryToEpisode(&restored, sum)

	// Fields the cache is responsible for carrying across a restart.
	if restored.title != original.title {
		t.Errorf("title: got %q, want %q", restored.title, original.title)
	}
	if restored.publishedAt != original.publishedAt {
		t.Errorf("publishedAt: got %d, want %d", restored.publishedAt, original.publishedAt)
	}
	if restored.duration != original.duration {
		t.Errorf("duration: got %v, want %v", restored.duration, original.duration)
	}
	if restored.season != original.season {
		t.Errorf("season: got %q, want %q", restored.season, original.season)
	}
	if restored.episode != original.episode {
		t.Errorf("episode: got %q, want %q", restored.episode, original.episode)
	}
	if !restored.hasTranscript {
		t.Error("hasTranscript did not survive the round trip")
	}
}

// hasAdsRemoved, fileSize, path and filename are written to the cache but
// deliberately not restored from it: loadSingleTUIEpisode recomputes each from
// the filesystem, which is authoritative and can change while pod is not
// running. This test documents that asymmetry so it reads as a decision rather
// than an oversight.
func TestCachedSummaryFieldsRecomputedNotRestored(t *testing.T) {
	sum := buildCachedEpisodeSummary(tuiEpisode{
		filename:      "ep1.mp3",
		path:          "/lib/Show/ep1.mp3",
		fileSize:      999,
		hasAdsRemoved: true,
	}, t.TempDir(), nil)

	if sum.SizeBytes != 999 || !sum.HasAdsRemoved {
		t.Fatalf("summary should still record them for inspection: %+v", sum)
	}

	var restored tuiEpisode
	applyCachedSummaryToEpisode(&restored, sum)

	if restored.fileSize != 0 {
		t.Errorf("fileSize should come from os.Stat, not the cache; got %d", restored.fileSize)
	}
	if restored.hasAdsRemoved {
		t.Error("hasAdsRemoved should come from the .cuts.json file, not the cache")
	}
	if restored.path != "" || restored.filename != "" {
		t.Errorf("path/filename should come from the directory walk; got %q/%q", restored.path, restored.filename)
	}
}
