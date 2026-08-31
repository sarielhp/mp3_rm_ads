package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLsLatestCommand(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Daily_Show")
	pod2 := filepath.Join(tempDir, "News_Hour")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)

	ep1 := filepath.Join(pod1, "ep1.mp3")
	ep2 := filepath.Join(pod2, "ep2.mp3")
	ep3 := filepath.Join(pod2, "ep3.mp3")

	_ = os.WriteFile(ep1, []byte("audio1"), 0644)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(ep2, []byte("audio2"), 0644)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(ep3, []byte("audio3"), 0644)

	_ = saveEpisodeStatus(statusPathFor(ep3), &EpisodeStatusFile{
		Status: StateDone,
	})

	cfg := Config{
		PodcastsDir: tempDir,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{
		LsSubcmd: "latest",
		Count:    2,
	}
	err := runLsCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runLsCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Latest 2 Episodes Across All Podcasts") {
		t.Errorf("expected Latest 2 header, got: %s", out)
	}
	if !strings.Contains(out, "ep3") {
		t.Errorf("expected newest episode ep3 to be listed, got: %s", out)
	}
	if !strings.Contains(out, "Clean") {
		t.Errorf("expected ep3 status Clean, got: %s", out)
	}
}

func TestLsSinglePodcastCommand(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "ShowA")
	_ = os.MkdirAll(pod1, 0755)
	ep1 := filepath.Join(pod1, "ep1.mp3")
	_ = os.WriteFile(ep1, []byte("audio"), 0644)

	id := getOrSetPodcastShortID(pod1, "ShowA")

	cfg := Config{
		PodcastsDir: tempDir,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{
		Args: []string{id},
	}
	err := runLsCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runLsCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Episodes for") || !strings.Contains(out, "ep1") {
		t.Errorf("expected podcast episodes listing, got: %s", out)
	}
}
