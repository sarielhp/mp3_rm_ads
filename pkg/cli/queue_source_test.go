package cli

import (
	"os"
	"path/filepath"
	"pod/pkg/backend"
	"pod/pkg/podcast"
	"reflect"
	"testing"
	"time"
)

func TestQueueCatalogRejectsUnsafeAndConflictingPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "episode.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	dates, err := queueCatalogDates(root, []backend.CatalogEpisode{
		{AudioPath: "episode.mp3", PublishedAt: 1000},
		{AudioPath: "episode.mp3", PublishedAt: 2000},
		{AudioPath: "episode.mp3", PublishedAt: 1000},
		{AudioPath: "../outside.mp3", PublishedAt: 1000},
		{AudioPath: ".work/temp.mp3", PublishedAt: 1000},
		{AudioPath: "precut.mp3", PublishedAt: 1000},
	})
	if err != nil || len(dates) != 1 || !dates[path].IsZero() {
		t.Fatalf("dates=%v error=%v", dates, err)
	}
}

func TestQueueTodayStandalone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Show")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "today.mp3")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	cached := []podcast.CachedEpisodeSummary{{EpisodeFile: podcast.EpisodeFile{Path: path, Filename: "today.mp3", PublishedAt: now.UnixMilli()}}}
	if err := podcast.SavePodcastCache(dir, &podcast.CachedPodcastIndex{Episodes: cached}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{PodcastsDir: root}
	cfg.BackendType = "standalone"
	if err := runQueueToday(cfg, root, CLIOptions{ProcOptions: ProcOptions{Quiet: true}}, now); err != nil {
		t.Fatalf("unexpected error in standalone: %v", err)
	}
	if got := readTodayTestQueue(t, dir); !reflect.DeepEqual(got, []string{"today.mp3"}) {
		t.Fatalf("queue = %v", got)
	}
}
