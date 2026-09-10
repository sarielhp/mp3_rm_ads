package podcast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEpisodeIDsDistinguishCachedNestedPaths(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a", "podcast.mp3")
	b := filepath.Join(dir, "b", "podcast.mp3")
	for _, path := range []string{a, b} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cache := &CachedPodcastIndex{Episodes: []CachedEpisodeSummary{{Path: a, Filename: "podcast.mp3", ID: "e12345"}}}
	if err := SavePodcastCache(dir, cache); err != nil {
		t.Fatal(err)
	}
	if id := EpisodeShortIDReadOnly(dir, "show", a); id != "e12345" {
		t.Fatalf("lost cached ID: %s", id)
	}
	if id := GetOrSetEpisodeShortID(dir, "show", b); id == "e12345" {
		t.Fatal("reused another episode's ID")
	}
}
