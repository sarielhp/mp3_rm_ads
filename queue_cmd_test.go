package main

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
	ep2ID := getOrSetEpisodeShortID(podDir, podID, ep2)

	cfg := Config{PodcastsDir: tempDir}

	// 1. Add ep1
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

	// 2. List queue with --json
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

	// 3. Remove ep1
	cliRemove := CLIOptions{QueueSubcmd: "remove", Args: []string{ep1ID}}
	if err := runQueueCommand(cfg, cliRemove); err != nil {
		t.Fatalf("runQueueCommand remove failed: %v", err)
	}
	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue after removal, got: %v", entries)
	}

	// 4. Add podcast (adds all uncleaned episodes: ep1 and ep2)
	cliAddPod := CLIOptions{QueueSubcmd: "add", Args: []string{podID}}
	if err := runQueueCommand(cfg, cliAddPod); err != nil {
		t.Fatalf("runQueueCommand add podcast failed: %v", err)
	}
	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 2 {
		t.Errorf("expected 2 episodes in queue after adding podcast, got: %v", entries)
	}

	// 5. Clear queue
	cliClear := CLIOptions{QueueSubcmd: "clear", Args: []string{podID}}
	if err := runQueueCommand(cfg, cliClear); err != nil {
		t.Fatalf("runQueueCommand clear failed: %v", err)
	}
	data, _ = os.ReadFile(qFile)
	_ = json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue after clear, got: %v", entries)
	}
	_ = ep2ID
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
