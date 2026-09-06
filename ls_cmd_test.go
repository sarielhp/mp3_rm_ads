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

func TestLsAllPodcastsCommand(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Alpha_Show")
	pod2 := filepath.Join(tempDir, "Beta_Cast")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)

	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(pod2, "ep2.mp3"), []byte("audio"), 0644)

	id1 := getOrSetPodcastShortID(pod1, "Alpha Show")
	id2 := getOrSetPodcastShortID(pod2, "Beta Cast")

	cfg := Config{PodcastsDir: tempDir}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{}
	err := runLsCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runLsCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Podcasts in Library") || !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		t.Errorf("expected podcasts list with IDs %s and %s, got: %s", id1, id2, out)
	}
}

func TestLsAllPodcastsJSONAndQuiet(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Gamma_Show")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio"), 0644)
	id1 := getOrSetPodcastShortID(pod1, "Gamma Show")

	cfg := Config{PodcastsDir: tempDir}

	// JSON test
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cliJSON := CLIOptions{Args: []string{"podcasts"}, JSON: true}
	err := runLsCommand(cfg, cliJSON)

	_ = w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runLsCommand json failed: %v", err)
	}
	outBytes, _ := io.ReadAll(r)
	if !strings.Contains(string(outBytes), id1) || !strings.Contains(string(outBytes), "episode_count") {
		t.Errorf("expected json output with id and episode_count, got: %s", string(outBytes))
	}

	// Quiet test
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w

	cliQuiet := CLIOptions{Quiet: true}
	err = runLsCommand(cfg, cliQuiet)

	_ = w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runLsCommand quiet failed: %v", err)
	}
	qBytes, _ := io.ReadAll(r)
	if strings.TrimSpace(string(qBytes)) != id1 {
		t.Errorf("expected quiet output %q, got: %s", id1, string(qBytes))
	}
}

func TestLsSinglePodcastJSON(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Delta_Show")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio"), 0644)
	id1 := getOrSetPodcastShortID(pod1, "Delta Show")

	cfg := Config{PodcastsDir: tempDir}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{Args: []string{id1}, JSON: true}
	err := runLsCommand(cfg, cli)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runLsCommand failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)
	if !strings.Contains(out, "podcast_id") || !strings.Contains(out, id1) {
		t.Errorf("expected json episodes list, got: %s", out)
	}
}
