package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"abs/pkg/backend"
	"abs/pkg/types"
)

func TestEnsureABSIgnore(t *testing.T) {
	tempDir := t.TempDir()
	if err := EnsureABSIgnore(tempDir); err != nil {
		t.Fatalf("EnsureABSIgnore failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, ".absignore"))
	if err != nil {
		t.Fatalf("reading .absignore failed: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "*.precut") || !strings.Contains(content, "*.bak") {
		t.Errorf("expected .absignore to contain *.precut and *.bak, got %q", content)
	}
}

func TestParseABSEpisodePublishedAt(t *testing.T) {
	ep1 := &backend.Episode{PublishedAt: 1724000000000}
	if got := ParseABSEpisodePublishedAt(ep1); got != 1724000000000 {
		t.Errorf("expected 1724000000000, got %d", got)
	}

	ep2 := &backend.Episode{PubDate: "Sun, 20 Aug 2026 12:00:00 GMT"}
	expectedT, _ := time.Parse(time.RFC1123, "Sun, 20 Aug 2026 12:00:00 GMT")
	if got := ParseABSEpisodePublishedAt(ep2); got != expectedT.UnixMilli() {
		t.Errorf("expected %d, got %d", expectedT.UnixMilli(), got)
	}

	ep3 := &backend.Episode{PubDate: "2026-08-20T12:00:00Z"}
	expectedISO, _ := time.Parse(time.RFC3339, "2026-08-20T12:00:00Z")
	if got := ParseABSEpisodePublishedAt(ep3); got != expectedISO.UnixMilli() {
		t.Errorf("expected %d, got %d", expectedISO.UnixMilli(), got)
	}

	if got := ParseABSEpisodePublishedAt(nil); got != 0 {
		t.Errorf("expected 0 for nil, got %d", got)
	}
}

func TestNormalizeEpisodeTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Episode 1.mp3", "episode 1"},
		{"Episode 1 (90b50030-4e0f-4e45-af9d-6).mp3", "episode 1"},
		{"My Great Show.mp3", "my great show"},
	}
	for _, c := range cases {
		if got := NormalizeEpisodeTitle(c.in); got != c.want {
			t.Errorf("NormalizeEpisodeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestQuarantineAbandonedDuplicates(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Hard Fork")
	_ = os.MkdirAll(podDir, 0755)

	guidMP3 := filepath.Join(podDir, "OpenAI Pause (90b50030-4e0f-4e45-af9d-6).mp3")
	bareMP3 := filepath.Join(podDir, "OpenAI Pause.mp3")
	bareCuts := filepath.Join(podDir, "OpenAI Pause.cuts.json")
	bareTranscript := filepath.Join(podDir, "OpenAI Pause.transcript.json")
	barePrecut := filepath.Join(podDir, "OpenAI Pause.mp3.precut")

	_ = os.WriteFile(guidMP3, []byte("tracked audio"), 0644)
	_ = os.WriteFile(bareMP3, []byte("abandoned audio"), 0644)
	_ = os.WriteFile(bareCuts, []byte("{}"), 0644)
	_ = os.WriteFile(bareTranscript, []byte("{}"), 0644)
	_ = os.WriteFile(barePrecut, []byte("precut audio"), 0644)

	trackedEpisodes := []backend.Episode{
		{
			Title: "OpenAI Pause",
			AudioFile: &backend.PodcastAudioFile{
				Metadata: &backend.AudioFileMetadata{
					Filename: "OpenAI Pause (90b50030-4e0f-4e45-af9d-6).mp3",
				},
			},
		},
	}

	quarantined := QuarantineAbandonedDuplicates(podDir, trackedEpisodes)
	if len(quarantined) != 1 {
		t.Fatalf("expected 1 quarantined file, got %d (%v)", len(quarantined), quarantined)
	}
	if quarantined[0] != "OpenAI Pause.mp3" {
		t.Errorf("expected 'OpenAI Pause.mp3', got %q", quarantined[0])
	}

	if _, err := os.Stat(bareMP3); !os.IsNotExist(err) {
		t.Errorf("expected bare MP3 to no longer exist")
	}
	if _, err := os.Stat(bareMP3 + ".bak"); err != nil {
		t.Errorf("expected bare MP3 .bak to exist: %v", err)
	}
	if _, err := os.Stat(bareCuts + ".bak"); err != nil {
		t.Errorf("expected bare cuts .bak to exist: %v", err)
	}
	if _, err := os.Stat(bareTranscript + ".bak"); err != nil {
		t.Errorf("expected bare transcript .bak to exist: %v", err)
	}
	if _, err := os.Stat(barePrecut + ".bak"); err != nil {
		t.Errorf("expected bare precut .bak to exist: %v", err)
	}
}

func TestResolveAudioFiles(t *testing.T) {
	tempDir := t.TempDir()
	mp3Path := filepath.Join(tempDir, "ep.mp3")
	_ = os.WriteFile(mp3Path, []byte("audio"), 0644)

	mainMP3, precut, src := ResolveAudioFiles(mp3Path, false)
	if mainMP3 != mp3Path {
		t.Errorf("expected mainMP3 %q, got %q", mp3Path, mainMP3)
	}
	if precut != mp3Path+".precut" {
		t.Errorf("expected precut %q, got %q", mp3Path+".precut", precut)
	}
	if src != mp3Path {
		t.Errorf("expected src %q, got %q", mp3Path, src)
	}
}

func TestResolveOutputFile(t *testing.T) {
	mainMP3 := "/podcasts/ep1.mp3"
	out := ResolveOutputFile(mainMP3, "", 1)
	if out != mainMP3 {
		t.Errorf("expected default to mainMP3, got %s", out)
	}

	custom := "/tmp/output.mp3"
	outCustom := ResolveOutputFile(mainMP3, custom, 1)
	if outCustom != custom {
		t.Errorf("expected custom output, got %s", outCustom)
	}
}

func TestFormatTranscript(t *testing.T) {
	td := &types.TranscriptionData{
		Segments: []types.TranscriptionSegment{
			{Start: 0.0, End: 5.0, Text: "Hello"},
			{Start: 5.0, End: 10.0, Text: "World"},
		},
	}
	formatted := FormatTranscript(td, 10.0)
	if !strings.Contains(formatted, "[0.0s -> 5.0s] Hello") {
		t.Errorf("unexpected formatted transcript: %q", formatted)
	}
}
