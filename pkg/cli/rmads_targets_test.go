package cli

import (
	"testing"
)

func TestResolvePodcastTarget(t *testing.T) {
	tmp := t.TempDir()
	podDir, _ := createTestPodcastWithEpisodes(t, tmp, "Daily Tech", []string{"Ep 1"})
	cfg := loadPodcastConfig(podDir)

	cliShort := CLIOptions{Podcast: cfg.ID}
	p1, ok1 := resolvePodcastTarget(tmp, cliShort)
	if !ok1 || p1 == nil || p1.Dir != podDir {
		t.Fatalf("expected to resolve podcast by short ID %s, got %v", cfg.ID, p1)
	}

	cliArgShort := CLIOptions{Args: []string{cfg.ID}}
	p2, ok2 := resolvePodcastTarget(tmp, cliArgShort)
	if !ok2 || p2 == nil || p2.Dir != podDir {
		t.Fatalf("expected to resolve podcast by arg short ID, got %v", p2)
	}

	cliArgTitle := CLIOptions{Args: []string{"Daily Tech"}}
	p3, ok3 := resolvePodcastTarget(tmp, cliArgTitle)
	if !ok3 || p3 == nil || p3.Dir != podDir {
		t.Fatalf("expected to resolve podcast by title, got %v", p3)
	}

	cliArgIndex := CLIOptions{Args: []string{"1"}}
	p4, ok4 := resolvePodcastTarget(tmp, cliArgIndex)
	if !ok4 || p4 == nil || p4.Dir != podDir {
		t.Fatalf("expected to resolve podcast by index 1, got %v", p4)
	}

	cliMP3 := CLIOptions{Args: []string{"ep1.mp3"}}
	if _, ok := resolvePodcastTarget(tmp, cliMP3); ok {
		t.Errorf("expected .mp3 arg to NOT resolve as podcast")
	}

	cliJSON := CLIOptions{Args: []string{"ep1.transcript.json"}}
	if _, ok := resolvePodcastTarget(tmp, cliJSON); ok {
		t.Errorf("expected .json arg to NOT resolve as podcast")
	}

	cliRoot := CLIOptions{Args: []string{tmp}}
	if _, ok := resolvePodcastTarget(tmp, cliRoot); ok {
		t.Errorf("expected root podcasts directory to NOT resolve as a single podcast")
	}
}
