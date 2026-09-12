package cli

import (
	"path/filepath"
	"reflect"
	"testing"

	"abs/pkg/pipeline"
)

func TestQueueLatestDefaultCount(t *testing.T) {
	root := t.TempDir()
	titles := []string{"Ep1", "Ep2", "Ep3", "Ep4", "Ep5", "Ep6", "Ep7"}
	dir, paths := createTestPodcastWithEpisodes(t, root, "Show", titles)

	cfg := Config{PodcastsDir: root}
	opts := CLIOptions{
		ProcOptions: ProcOptions{
			Quiet: true,
		},
	}

	if err := runQueueLatest(cfg, root, 5, "", opts); err != nil {
		t.Fatal(err)
	}

	queue, err := pipeline.ReadQueue(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(queue) != 5 {
		t.Fatalf("expected 5 queued episodes, got %d: %v", len(queue), queue)
	}

	expected := []string{
		filepath.Base(paths[6]),
		filepath.Base(paths[5]),
		filepath.Base(paths[4]),
		filepath.Base(paths[3]),
		filepath.Base(paths[2]),
	}
	if !reflect.DeepEqual(queue, expected) {
		t.Fatalf("queue mismatch: got %v, want %v", queue, expected)
	}
}

func TestQueueLatestExcludesClean(t *testing.T) {
	root := t.TempDir()
	titles := []string{"Old", "Middle", "Newest"}
	dir, paths := createTestPodcastWithEpisodes(t, root, "Show", titles)

	markEpisodeClean(t, paths[2])

	cfg := Config{PodcastsDir: root}
	opts := CLIOptions{
		ProcOptions: ProcOptions{
			Quiet: true,
		},
	}

	if err := runQueueLatest(cfg, root, 2, "", opts); err != nil {
		t.Fatal(err)
	}

	queue, err := pipeline.ReadQueue(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		filepath.Base(paths[1]),
		filepath.Base(paths[0]),
	}
	if !reflect.DeepEqual(queue, expected) {
		t.Fatalf("queue mismatch: got %v, want %v", queue, expected)
	}
}

func TestQueueLatestTargetPodcast(t *testing.T) {
	root := t.TempDir()
	dirA, pathsA := createTestPodcastWithEpisodes(t, root, "ShowA", []string{"A1", "A2"})
	dirB, _ := createTestPodcastWithEpisodes(t, root, "ShowB", []string{"B1", "B2"})

	cfg := Config{PodcastsDir: root}
	opts := CLIOptions{
		ProcOptions: ProcOptions{
			Quiet: true,
		},
	}

	if err := runQueueLatest(cfg, root, 5, "showa", opts); err != nil {
		t.Fatal(err)
	}

	queueA, err := pipeline.ReadQueue(dirA)
	if err != nil {
		t.Fatal(err)
	}
	if len(queueA) != 2 {
		t.Fatalf("expected 2 items in ShowA queue, got %d: %v", len(queueA), queueA)
	}
	expectedA := []string{filepath.Base(pathsA[1]), filepath.Base(pathsA[0])}
	if !reflect.DeepEqual(queueA, expectedA) {
		t.Fatalf("ShowA mismatch: got %v, want %v", queueA, expectedA)
	}

	queueB, _ := pipeline.ReadQueue(dirB)
	if len(queueB) != 0 {
		t.Fatalf("expected ShowB queue to be empty, got %v", queueB)
	}
}

func TestQueueLatestDryRun(t *testing.T) {
	root := t.TempDir()
	dir, _ := createTestPodcastWithEpisodes(t, root, "Show", []string{"Ep1", "Ep2"})

	cfg := Config{PodcastsDir: root}
	opts := CLIOptions{
		ProcOptions: ProcOptions{
			DryRun: true,
			Quiet:  true,
		},
	}

	if err := runQueueLatest(cfg, root, 5, "", opts); err != nil {
		t.Fatal(err)
	}

	queue, _ := pipeline.ReadQueue(dir)
	if len(queue) != 0 {
		t.Fatalf("dry run should not modify queue, got: %v", queue)
	}
}

func TestQueueLatestParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	if err := app.Execute([]string{"queue", "latest", "5", "--dry-run", "--quiet"}); err != nil {
		t.Fatal(err)
	}
	if action != "queue" || opts.QueueSubcmd != "latest" || !opts.DryRun || !opts.Quiet {
		t.Fatalf("unexpected parsed command: %s %+v", action, opts)
	}

	limit, target, err := parseQueueLatestArgs(opts.Args)
	if err != nil || limit != 5 || target != "" {
		t.Fatalf("parseQueueLatestArgs mismatch: limit=%d, target=%q, err=%v", limit, target, err)
	}

	limit, target, err = parseQueueLatestArgs([]string{"3", "mypod"})
	if err != nil || limit != 3 || target != "mypod" {
		t.Fatalf("parseQueueLatestArgs mismatch: limit=%d, target=%q, err=%v", limit, target, err)
	}

	limit, target, err = parseQueueLatestArgs([]string{"mypod", "3"})
	if err != nil || limit != 3 || target != "mypod" {
		t.Fatalf("parseQueueLatestArgs reversed mismatch: limit=%d, target=%q, err=%v", limit, target, err)
	}

	limit, target, err = parseQueueLatestArgs([]string{})
	if err != nil || limit != 5 || target != "" {
		t.Fatalf("parseQueueLatestArgs empty mismatch: limit=%d, target=%q, err=%v", limit, target, err)
	}

	if _, _, err := parseQueueLatestArgs([]string{"0"}); err == nil {
		t.Fatal("expected error for limit 0")
	}

	if _, _, err := parseQueueLatestArgs([]string{"-2"}); err == nil {
		t.Fatal("expected error for negative limit")
	}

	if _, _, err := parseQueueLatestArgs([]string{"pod1", "pod2"}); err == nil {
		t.Fatal("expected error for multiple target names")
	}
}
