package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGeneratePodcastShortID(t *testing.T) {
	cases := []struct {
		title    string
		expected string
	}{
		{"Huberman Lab", "hbrmn"},
		{"The Daily", "thdly"},
		{"Planet Money", "plntm"},
		{"Radiolab", "radio"},
		{"99% Invisible", "99inv"},
		{"Lex Fridman Podcast", "lxfrd"},
		{"This American Life", "thsam"},
		{"Freakonomics Radio", "frknm"},
		{"Darknet Diaries", "drknt"},
	}

	for _, c := range cases {
		got := generatePodcastShortID(c.title)
		if got != c.expected {
			t.Errorf("generatePodcastShortID(%q) = %q, expected %q", c.title, got, c.expected)
		}
		if len(got) != 5 {
			t.Errorf("expected 5 characters, got %d for %q", len(got), got)
		}
		matched, _ := regexp.MatchString(`^[a-z0-9]{5}$`, got)
		if !matched {
			t.Errorf("expected lowercase alphanumeric string, got %q", got)
		}
	}
}

func TestGeneratePodcastShortIDShortAndNonASCII(t *testing.T) {
	shortCases := []string{"AI", "BBC", "99", "", "   "}
	for _, title := range shortCases {
		got := generatePodcastShortID(title)
		if len(got) != 5 {
			t.Errorf("expected 5 chars for short title %q, got %q", title, got)
		}
		matched, _ := regexp.MatchString(`^[a-z0-9]{5}$`, got)
		if !matched {
			t.Errorf("expected lowercase alphanumeric string, got %q for %q", got, title)
		}
		clean := strings.TrimSpace(title)
		if clean != "" {
			h := sha256.Sum256([]byte(clean))
			expectedHex := hex.EncodeToString(h[:])[:5]
			if got != expectedHex {
				t.Errorf("expected sha256 hex fallback %q for %q, got %q", expectedHex, title, got)
			}
		}
	}

	hebrewCases := []string{"עושים היסטוריה", "חיות כיס", "התשובה"}
	for _, title := range hebrewCases {
		got := generatePodcastShortID(title)
		if len(got) != 5 {
			t.Errorf("expected 5 chars for Hebrew title %q, got %q", title, got)
		}
		matched, _ := regexp.MatchString(`^[a-z0-9]{5}$`, got)
		if !matched {
			t.Errorf("expected lowercase alphanumeric string, got %q for %q", got, title)
		}
		h := sha256.Sum256([]byte(title))
		expectedHex := hex.EncodeToString(h[:])[:5]
		if got != expectedHex {
			t.Errorf("expected sha256 hex fallback %q for %q, got %q", expectedHex, title, got)
		}
	}
}

func TestGetOrSetPodcastShortIDPersistence(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Lex_Fridman")
	_ = os.MkdirAll(podDir, 0755)

	id1 := getOrSetPodcastShortID(podDir, "Lex Fridman Podcast")
	if id1 != "lxfrd" {
		t.Errorf("expected id 'lxfrd', got %q", id1)
	}

	cfgPath := filepath.Join(podDir, "podcast.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("expected podcast.json to exist, err: %v", err)
	}

	var cfg PodcastConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal podcast.json: %v", err)
	}
	if cfg.ID != "lxfrd" {
		t.Errorf("expected persisted id 'lxfrd', got %q", cfg.ID)
	}

	id2 := getOrSetPodcastShortID(podDir, "Different Title That Should Not Overwrite")
	if id2 != "lxfrd" {
		t.Errorf("expected existing id 'lxfrd' to be returned, got %q", id2)
	}

	cfg.ID = "custom"
	_ = savePodcastConfig(podDir, cfg)

	id3 := getOrSetPodcastShortID(podDir, "Lex Fridman")
	if id3 != "custom" {
		t.Errorf("expected custom id 'custom' to be preserved, got %q", id3)
	}
}

func TestResolvePodcastDirByIDOrName(t *testing.T) {
	tempDir := t.TempDir()

	pod1 := filepath.Join(tempDir, "Alpha_Show")
	pod2 := filepath.Join(tempDir, "Beta_Podcast")
	pod3 := filepath.Join(tempDir, "Gamma_Radio")

	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)
	_ = os.MkdirAll(pod3, 0755)

	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(pod2, "ep1.mp3"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(pod3, "ep1.mp3"), []byte("audio"), 0644)

	id1 := getOrSetPodcastShortID(pod1, "Alpha Show")
	id2 := getOrSetPodcastShortID(pod2, "Beta Podcast")
	id3 := getOrSetPodcastShortID(pod3, "Gamma Radio")

	d, title, ok := resolvePodcastDirByIDOrName(tempDir, id2)
	if !ok || d != pod2 || title != "Beta_Podcast" {
		t.Errorf("resolve by short ID failed: got (%q, %q, %v), expected (%q, %q, true)", d, title, ok, pod2, "Beta_Podcast")
	}

	d, _, ok = resolvePodcastDirByIDOrName(tempDir, strings.ToUpper(id1))
	if !ok || d != pod1 {
		t.Errorf("resolve by upper short ID failed: got (%q, %v), expected (%q, true)", d, ok, pod1)
	}

	d, _, ok = resolvePodcastDirByIDOrName(tempDir, "1")
	if !ok || d != pod1 {
		t.Errorf("resolve by 1-based index 1 failed: got (%q, %v), expected (%q, true)", d, ok, pod1)
	}

	d, _, ok = resolvePodcastDirByIDOrName(tempDir, "3")
	if !ok || d != pod3 {
		t.Errorf("resolve by 1-based index 3 failed: got (%q, %v), expected (%q, true)", d, ok, pod3)
	}

	d, _, ok = resolvePodcastDirByIDOrName(tempDir, "Alpha_Show")
	if !ok || d != pod1 {
		t.Errorf("resolve by folder name failed: got (%q, %v), expected (%q, true)", d, ok, pod1)
	}

	d, _, ok = resolvePodcastDirByIDOrName(tempDir, "gamma")
	if !ok || d != pod3 {
		t.Errorf("resolve by substring failed: got (%q, %v), expected (%q, true)", d, ok, pod3)
	}

	_, _, ok = resolvePodcastDirByIDOrName(tempDir, "NonExistent")
	if ok {
		t.Errorf("expected non-existent query to return ok=false")
	}

	_ = id3
}

func TestStatusReportColumns(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Alpha_Show")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio"), 0644)

	id := getOrSetPodcastShortID(pod1, "Alpha_Show")

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	renderLocalDiskPodcastStatus(tempDir, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "#") || !strings.Contains(out, "ID") || !strings.Contains(out, "Podcast Name") {
		t.Errorf("expected header with #, ID, Podcast Name, got: %s", out)
	}
	if !strings.Contains(out, id) {
		t.Errorf("expected short ID %q in status output, got: %s", id, out)
	}
	if !strings.Contains(out, "1 ") {
		t.Errorf("expected 1-based index '1' in status output, got: %s", out)
	}
}

func TestBatchProcTargetedPodcastExecution(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Pod_One")
	pod2 := filepath.Join(tempDir, "Pod_Two")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)

	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio1"), 0644)
	_ = os.WriteFile(filepath.Join(pod2, "ep2.mp3"), []byte("audio2"), 0644)

	id1 := getOrSetPodcastShortID(pod1, "Pod_One")
	id2 := getOrSetPodcastShortID(pod2, "Pod_Two")

	cfg := Config{
		PodcastsDir: tempDir,
	}

	cli1 := CLIOptions{
		Args:   []string{id1},
		DryRun: true,
		Quiet:  true,
	}
	processAudioFilesBatch(cli1, cfg, "proc")

	cli2 := CLIOptions{
		Podcast: id2,
		DryRun:  true,
		Quiet:   true,
	}
	processAudioFilesBatch(cli2, cfg, "proc")

	cliIndex := CLIOptions{
		Args:   []string{"1"},
		DryRun: true,
		Quiet:  true,
	}
	processAudioFilesBatch(cliIndex, cfg, "proc")
}
