package cli

import (
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestQueueTodayReadsPodfetchDatesWithoutCachedDates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Show")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	dbPath := filepath.Join(root, "catalog.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer backend.ClosePodfetchDB(dbPath)
	defer backend.SetAudiobookshelfDisabled(false)
	defer backend.SetPodfetchDisabled(false)
	_, err = db.Exec(`CREATE TABLE podcast_episodes (podcast_id TEXT, guid TEXT, url TEXT, name TEXT, file_episode_path TEXT, date_of_recording TEXT, deleted INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	var cached []podcast.CachedEpisodeSummary
	for i, name := range []string{"today", "yesterday", "unknown", "deleted"} {
		path := filepath.Join(dir, name, "podcast.mp3")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
			t.Fatal(err)
		}
		date := now.Format(time.RFC3339)
		if i == 1 {
			date = now.AddDate(0, 0, -1).Format(time.RFC3339)
		}
		if i == 2 {
			date = ""
		}
		deleted := 0
		if i == 3 {
			deleted = 1
		}
		_, err = db.Exec(`INSERT INTO podcast_episodes VALUES ('show', '', '', ?, ?, ?, ?)`, name, "podcasts/Show/"+name+"/podcast.mp3", date, deleted)
		if err != nil {
			t.Fatal(err)
		}
		cacheDate := now.UnixMilli()
		if i == 0 {
			cacheDate = 0
		}
		cached = append(cached, podcast.CachedEpisodeSummary{Path: path, Filename: "podcast.mp3", PublishedAt: cacheDate})
	}
	if err := podcast.SavePodcastCache(dir, &podcast.CachedPodcastIndex{Episodes: cached}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{PodcastsDir: root}
	cfg.BackendType = "podfetch"
	cfg.PodfetchDBPath = dbPath
	before := queueTree(t, root)
	if err := runQueueToday(cfg, root, CLIOptions{ProcOptions: ProcOptions{DryRun: true}}, now); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, queueTree(t, root)) {
		t.Fatal("dry run changed library")
	}
	if err := runQueueToday(cfg, root, CLIOptions{ProcOptions: ProcOptions{Quiet: true}}, now); err != nil {
		t.Fatal(err)
	}
	if got := readTodayTestQueue(t, dir); !reflect.DeepEqual(got, []string{filepath.Join("today", "podcast.mp3")}) {
		t.Fatalf("queue = %v", got)
	}
	before = queueTree(t, root)
	cfg.PodfetchDBPath = filepath.Join(root, "missing.db")
	if err := runQueueToday(cfg, root, CLIOptions{}, now); err == nil {
		t.Fatal("source failure hidden")
	}
	if !reflect.DeepEqual(before, queueTree(t, root)) {
		t.Fatal("source failure changed library")
	}
}

func TestQueueCatalogRejectsUnsafeAndConflictingPaths(t *testing.T) {
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
	cached := []podcast.CachedEpisodeSummary{{Path: path, Filename: "today.mp3", PublishedAt: now.UnixMilli()}}
	if err := podcast.SavePodcastCache(dir, &podcast.CachedPodcastIndex{Episodes: cached}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{PodcastsDir: root}
	cfg.AudiobookshelfURL = "http://example.com"
	cfg.BackendType = "standalone"
	if err := runQueueToday(cfg, root, CLIOptions{ProcOptions: ProcOptions{Quiet: true}}, now); err != nil {
		t.Fatalf("unexpected error in standalone: %v", err)
	}
	if got := readTodayTestQueue(t, dir); !reflect.DeepEqual(got, []string{"today.mp3"}) {
		t.Fatalf("queue = %v", got)
	}
}
