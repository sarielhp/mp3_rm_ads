package cli

import (
	"io"
	"os"
	"path/filepath"
	"pod/pkg/podcast"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQueueDisplayContinuesPastAmbiguousEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"first/podcast.mp3", "second/podcast.mp3"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	queue := `["podcast.mp3","first/podcast.mp3","missing.mp3"]`
	if err := os.WriteFile(filepath.Join(dir, "queue.json"), []byte(queue), 0644); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 9, 10, 12, 34, 0, 0, time.UTC)
	title := "A meaningful episode title that is long enough to require shortening in the queue table"
	cache := &podcast.CachedPodcastIndex{Episodes: []podcast.CachedEpisodeSummary{
		{Path: "first/podcast.mp3", ID: "e12345", Title: title, Duration: 1234, PublishedAt: date.UnixMilli()},
		{Path: "second/podcast.mp3", ID: "e54321", Title: "Different episode", Duration: 55},
	}}
	if err := podcast.SavePodcastCache(dir, cache); err != nil {
		t.Fatal(err)
	}
	before := queueTree(t, dir)
	items, err := collectQueueDisplayItems(podcast.PodcastDirEntry{Dir: dir, ShortID: "pod1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d rows", len(items))
	}
	if items[0].EpisodeID != "" || items[0].Title != "Ambiguous legacy entry" {
		t.Fatalf("guessed episode: %+v", items[0])
	}
	if items[1].EpisodeID != "e12345" || items[1].Title != title || items[1].DurationSec != 1234 {
		t.Fatalf("wrong metadata: %+v", items[1])
	}
	cells := queueDisplayCells(items[1])
	if cells[3] != " 0:20" || cells[4] != queuePublicationTime(date, time.Now()) || cells[5] != shortenedQueueTitle(title) {
		t.Fatalf("wrong display: %v", cells)
	}
	if items[2].ResolutionError == "" {
		t.Fatal("missing entry lost diagnostic")
	}
	if !reflect.DeepEqual(before, queueTree(t, dir)) {
		t.Fatal("listing mutated queue or metadata")
	}
	if _, err := collectPodcastQueueItems(podcast.PodcastDirEntry{Dir: dir}); err == nil {
		t.Fatal("processing accepted ambiguous entry")
	}
	assertQueueTableColumns(t, items)
}

func TestQueuePublicationRelativeDates(t *testing.T) {
	zone, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 9, 0, 30, 0, 0, zone)
	for age, want := range map[int]string{
		0: "today  9:05", 1: "yesterday  9:05", 2: "-2d  9:05",
		3: "-3d  9:05", 10: "-10d  9:05", 11: "2026-02-26  9:05",
	} {
		date := time.Date(2026, 3, 9-age, 9, 5, 9, 0, zone)
		if got := queuePublicationTime(date.UTC(), now); got != want {
			t.Errorf("age %d: got %q, want %q", age, got, want)
		}
	}
}

func TestShortenedQueueTitle(t *testing.T) {
	for input, want := range map[string]string{
		"The Daily Show with Jane":        "Daily Jane",
		"THE Science SHOW WITH Ada":       "Science Ada",
		"Theater Showcase without limits": "Theater Showcase without limits",
		"The, Show: with Jane":            "Jane",
		"The Show":                        "The Show",
	} {
		if got := shortenedQueueTitle(input); got != want {
			t.Errorf("%q: got %q, want %q", input, got, want)
		}
	}
}

func assertQueueTableColumns(t *testing.T, items []queueEpisodeItem) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	printQueueTable(items)
	_ = w.Close()
	os.Stdout = stdout
	data, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{"Podcast ID", "Episode ID", "Pri", "Length", "P-date", "Title"} {
		if !strings.Contains(string(data), header) {
			t.Errorf("missing column %s", header)
		}
	}
	if strings.Contains(string(data), ".mp3") || strings.Contains(string(data), "File") {
		t.Fatalf("table shows files: %s", data)
	}
}
