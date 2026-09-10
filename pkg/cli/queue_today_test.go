package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"abs/pkg/podcast"
)

func TestQueueTodaySelection(t *testing.T) {
	root := t.TempDir()
	zone, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, zone)
	dir, paths := createTestPodcastWithEpisodes(t, root, "Today", []string{"midnight", "late", "yesterday", "tomorrow", "unknown", "clean", "cached"})
	dates := []string{"2026-03-08T00:00:00-06:00", "2026-03-09T04:59:59Z", "2026-03-07T23:59:59-06:00", "2026-03-09T00:00:00-05:00", "", "2026-03-08T12:00:00-05:00", "invalid"}
	for i, path := range paths {
		st := getOrCreateEpisodeStatus(path)
		st.PublishedAt = dates[i]
		if err := saveEpisodeStatus(statusPathFor(path), st); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
	}
	markEpisodeClean(t, paths[5])
	index := &podcast.CachedPodcastIndex{Episodes: []podcast.CachedEpisodeSummary{
		{Filename: filepath.Base(paths[6]), PublishedAt: now.UnixMilli()},
		{Filename: "unknown extended.mp3", PublishedAt: now.UnixMilli()},
	}}
	if err := podcast.SavePodcastCache(dir, index); err != nil {
		t.Fatal(err)
	}
	addEpisodeToQueueFile(dir, "existing.mp3")
	if err := handleQueueToday(root, CLIOptions{
		ProcOptions: ProcOptions{
			DryRun: true,
		},
	}, now); err != nil {
		t.Fatal(err)
	}
	if got := readTodayTestQueue(t, dir); !reflect.DeepEqual(got, []string{"existing.mp3"}) {
		t.Fatalf("dry run changed queue: %v", got)
	}
	for i := 0; i < 2; i++ {
		if err := handleQueueToday(root, CLIOptions{
			ProcOptions: ProcOptions{
				Quiet: true,
			},
		}, now); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"existing.mp3", "cached.mp3", "late.mp3", "midnight.mp3"}
	if got := readTodayTestQueue(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("queue = %v, want %v", got, want)
	}
}

func readTodayTestQueue(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestQueueTodayParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"queue", "today", "--dry-run", "--quiet"}); err != nil {
		t.Fatal(err)
	}
	if action != "queue" || opts.QueueSubcmd != "today" || !opts.DryRun || !opts.Quiet {
		t.Fatalf("unexpected parsed command: %s %+v", action, opts)
	}
	for _, args := range [][]string{{"queue", "today", "extra"}, {"download"}} {
		if err := app.Execute(args); err == nil {
			t.Fatalf("expected rejection for %v", args)
		}
	}
}
