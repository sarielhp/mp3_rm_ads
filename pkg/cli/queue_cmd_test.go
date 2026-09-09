package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueueListEmpty(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{PodcastsDir: tempDir}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{QueueSubcmd: "list"}
	err := runQueueCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runQueueCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	if !strings.Contains(string(outBytes), "empty") {
		t.Errorf("expected empty queue message, got: %s", string(outBytes))
	}
}

func testQueueAddAndList(t *testing.T, cfg Config, podDir, ep1ID string) {
	cliAdd := CLIOptions{QueueSubcmd: "add", Args: []string{ep1ID}}
	if err := runQueueCommand(cfg, cliAdd); err != nil {
		t.Fatalf("runQueueCommand add failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err != nil {
		t.Fatalf("expected queue.json to exist: %v", err)
	}
	var entries []string
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 1 || entries[0] != "ep1.mp3" {
		t.Errorf("expected [ep1.mp3] in queue, got: %v", entries)
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cliListJSON := CLIOptions{QueueSubcmd: "list", JSON: true}
	_ = runQueueCommand(cfg, cliListJSON)

	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)
	var listItems []queueEpisodeItem
	if err := json.Unmarshal(outBytes, &listItems); err != nil || len(listItems) != 1 {
		t.Fatalf("failed to parse queue json: %v, got %s", err, string(outBytes))
	}
	if listItems[0].EpisodeID != ep1ID {
		t.Errorf("expected episode %s in list, got %s", ep1ID, listItems[0].EpisodeID)
	}
}

func testQueueRemoveAndClear(t *testing.T, cfg Config, podDir, podID, ep1ID string) {
	qFile := filepath.Join(podDir, "queue.json")
	cliRemove := CLIOptions{QueueSubcmd: "remove", Args: []string{ep1ID}}
	if err := runQueueCommand(cfg, cliRemove); err != nil {
		t.Fatalf("runQueueCommand remove failed: %v", err)
	}
	data, _ := os.ReadFile(qFile)
	var entries []string
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue after removal, got: %v", entries)
	}

	cliAddPod := CLIOptions{QueueSubcmd: "add", Args: []string{podID}}
	if err := runQueueCommand(cfg, cliAddPod); err != nil {
		t.Fatalf("runQueueCommand add podcast failed: %v", err)
	}
	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 2 {
		t.Errorf("expected 2 episodes in queue after adding podcast, got: %v", entries)
	}

	cliClear := CLIOptions{QueueSubcmd: "clear", Args: []string{podID}}
	if err := runQueueCommand(cfg, cliClear); err != nil {
		t.Fatalf("runQueueCommand clear failed: %v", err)
	}
	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue after clear, got: %v", entries)
	}
}

func TestQueueAddRemoveAndClear(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Show_Q")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "ep1.mp3")
	ep2 := filepath.Join(podDir, "ep2.mp3")
	_ = os.WriteFile(ep1, []byte("audio1"), 0644)
	_ = os.WriteFile(ep2, []byte("audio2"), 0644)

	podID := getOrSetPodcastShortID(podDir, "Show_Q")
	ep1ID := getOrSetEpisodeShortID(podDir, podID, ep1)
	_ = getOrSetEpisodeShortID(podDir, podID, ep2)

	cfg := Config{PodcastsDir: tempDir}
	testQueueAddAndList(t, cfg, podDir, ep1ID)
	testQueueRemoveAndClear(t, cfg, podDir, podID, ep1ID)
}

func TestUpdateQueue_ConcurrentTransactions(t *testing.T) {
	tempDir := t.TempDir()

	var wg syncWG
	for i := 0; i < 10; i++ {
		fn := fmt.Sprintf("ep%d.mp3", i)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			addEpisodeToQueueFile(tempDir, name)
		}(fn)
	}
	wg.Wait()

	qFile := filepath.Join(tempDir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err != nil {
		t.Fatalf("expected queue.json: %v", err)
	}
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(entries) != 10 {
		t.Fatalf("expected exactly 10 episodes in queue, got %d: %v", len(entries), entries)
	}

	for i := 0; i < 5; i++ {
		fn := fmt.Sprintf("ep%d.mp3", i)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			removeEpisodeFromQueueFile(tempDir, name)
		}(fn)
	}
	wg.Wait()

	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 5 {
		t.Fatalf("expected exactly 5 episodes in queue after concurrent remove, got %d", len(entries))
	}
}

func TestPrintQueueTableHebrew(t *testing.T) {
	items := []queueEpisodeItem{
		{
			PodcastID: "pod1",
			EpisodeID: "ep01",
			Title:     "פרק מיוחד בעברית",
			AudioPath: "/podcasts/heb/ep01.mp3",
			Filename:  "פרק מיוחד בעברית.mp3",
		},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	printQueueTable(items)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	expected := displayName("פרק מיוחד בעברית")
	if !strings.Contains(out, expected) {
		t.Errorf("expected queue table to contain %q, got: %s", expected, out)
	}
}

func TestQueueRun_Empty(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{QueueSubcmd: "run", Quiet: true}
	if err := runQueueCommand(cfg, cli); err != nil {
		t.Fatalf("expected nil error on empty queue run, got: %v", err)
	}
}

func TestQueueRun_DryRun(t *testing.T) {
	tempDir := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tempDir, "DryShow", []string{"Ep1", "Ep2"})
	addEpisodeToQueueFile(podDir, filepath.Base(paths[0]))
	addEpisodeToQueueFile(podDir, filepath.Base(paths[1]))

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{QueueSubcmd: "run", DryRun: true, Quiet: true}
	if err := runQueueCommand(cfg, cli); err != nil {
		t.Fatalf("runQueueCommand dry-run failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, _ := os.ReadFile(qFile)
	var entries []string
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 2 {
		t.Fatalf("expected 2 episodes to remain in queue after dry run, got %d", len(entries))
	}
}

func TestQueueRun_CleansAndDequeues(t *testing.T) {
	tempDir := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tempDir, "QueueShow", []string{"Ep1", "Ep2"})
	addEpisodeToQueueFile(podDir, filepath.Base(paths[0]))
	addEpisodeToQueueFile(podDir, filepath.Base(paths[1]))

	markEpisodeClean(t, paths[0])
	markEpisodeClean(t, paths[1])

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{QueueSubcmd: "run", Quiet: true, Local: true}
	if err := runQueueCommand(cfg, cli); err != nil {
		t.Fatalf("runQueueCommand run failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, _ := os.ReadFile(qFile)
	var entries []string
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Fatalf("expected 0 episodes in queue after processing clean episodes, got %d", len(entries))
	}
}

func TestQueueRun_SpecificTarget(t *testing.T) {
	tempDir := t.TempDir()
	pod1Dir, paths1 := createTestPodcastWithEpisodes(t, tempDir, "PodA", []string{"EpA"})
	pod2Dir, paths2 := createTestPodcastWithEpisodes(t, tempDir, "PodB", []string{"EpB"})

	addEpisodeToQueueFile(pod1Dir, filepath.Base(paths1[0]))
	addEpisodeToQueueFile(pod2Dir, filepath.Base(paths2[0]))

	markEpisodeClean(t, paths1[0])
	markEpisodeClean(t, paths2[0])

	pod1Cfg := loadPodcastConfig(pod1Dir)
	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{QueueSubcmd: "run", Args: []string{pod1Cfg.ID}, Quiet: true, Local: true}
	if err := runQueueCommand(cfg, cli); err != nil {
		t.Fatalf("runQueueCommand target failed: %v", err)
	}

	q1, _ := os.ReadFile(filepath.Join(pod1Dir, "queue.json"))
	var entries1 []string
	_ = json.Unmarshal(q1, &entries1)
	if len(entries1) != 0 {
		t.Fatalf("expected PodA queue to be empty, got %v", entries1)
	}

	q2, _ := os.ReadFile(filepath.Join(pod2Dir, "queue.json"))
	var entries2 []string
	_ = json.Unmarshal(q2, &entries2)
	if len(entries2) != 1 {
		t.Fatalf("expected PodB queue to still have 1 entry, got %v", entries2)
	}
}

func TestQueueRun_MissingFileAutoDequeued(t *testing.T) {
	tempDir := t.TempDir()
	podDir, _ := createTestPodcastWithEpisodes(t, tempDir, "MissingShow", []string{})
	addEpisodeToQueueFile(podDir, "nonexistent.mp3")

	cfg := Config{PodcastsDir: tempDir}
	cli := CLIOptions{QueueSubcmd: "run", Quiet: true}
	if err := runQueueCommand(cfg, cli); err != nil {
		t.Fatalf("runQueueCommand failed on missing file: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, _ := os.ReadFile(qFile)
	var entries []string
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Fatalf("expected missing file to be removed from queue, got %v", entries)
	}
}

func TestQueueRun_CLIHelpParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	if err := app.Execute([]string{"queue", "run", "pod-1", "--dry-run", "--quiet"}); err != nil {
		t.Fatalf("expected queue run to parse: %v", err)
	}
	if action != "queue" || opts.QueueSubcmd != "run" {
		t.Fatalf("expected action=queue, subcmd=run, got action=%s, subcmd=%s", action, opts.QueueSubcmd)
	}
	if !opts.DryRun || !opts.Quiet {
		t.Fatalf("expected dry-run and quiet to be true")
	}
	if len(opts.Args) != 1 || opts.Args[0] != "pod-1" {
		t.Fatalf("expected args ['pod-1'], got %v", opts.Args)
	}
}
