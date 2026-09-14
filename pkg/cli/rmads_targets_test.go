package cli

import (
	"errors"
	"pod/pkg/config"
	"pod/pkg/podcast"
	"testing"
)

func TestResolvePodcastTarget(t *testing.T) {
	tmp := t.TempDir()
	podDir, _ := createTestPodcastWithEpisodes(t, tmp, "Daily Tech", []string{"Ep 1"})
	cfg := config.LoadPodcastConfig(podDir, config.PodcastConfig{})

	cliShort := CLIOptions{Podcast: cfg.ID}
	p1, err1 := resolvePodcastTarget(tmp, cliShort)
	if err1 != nil || p1 == nil || p1.Dir != podDir {
		t.Fatalf("expected to resolve podcast by short ID %s, got %v, err %v", cfg.ID, p1, err1)
	}

	cliArgShort := CLIOptions{Args: []string{cfg.ID}}
	p2, err2 := resolvePodcastTarget(tmp, cliArgShort)
	if err2 != nil || p2 == nil || p2.Dir != podDir {
		t.Fatalf("expected to resolve podcast by arg short ID, got %v, err %v", p2, err2)
	}

	cliArgTitle := CLIOptions{Args: []string{"Daily Tech"}}
	p3, err3 := resolvePodcastTarget(tmp, cliArgTitle)
	if err3 != nil || p3 == nil || p3.Dir != podDir {
		t.Fatalf("expected to resolve podcast by title, got %v, err %v", p3, err3)
	}

	cliArgIndex := CLIOptions{Args: []string{"1"}}
	p4, err4 := resolvePodcastTarget(tmp, cliArgIndex)
	if err4 != nil || p4 == nil || p4.Dir != podDir {
		t.Fatalf("expected to resolve podcast by index 1, got %v, err %v", p4, err4)
	}

	cliMP3 := CLIOptions{Args: []string{"ep1.mp3"}}
	if p, err := resolvePodcastTarget(tmp, cliMP3); p != nil || err != nil {
		t.Errorf("expected .mp3 arg to NOT resolve as podcast")
	}

	cliJSON := CLIOptions{Args: []string{"ep1.transcript.json"}}
	if p, err := resolvePodcastTarget(tmp, cliJSON); p != nil || err != nil {
		t.Errorf("expected .json arg to NOT resolve as podcast")
	}

	cliRoot := CLIOptions{Args: []string{tmp}}
	if p, err := resolvePodcastTarget(tmp, cliRoot); p != nil || err != nil {
		t.Errorf("expected root podcasts directory to NOT resolve as a single podcast")
	}

	createTestPodcastWithEpisodes(t, tmp, "Daily News", []string{"Ep 1"})
	cliAmb := CLIOptions{Podcast: "Daily"}
	_, errAmb := resolvePodcastTarget(tmp, cliAmb)
	if errAmb == nil || !errors.Is(errAmb, podcast.ErrAmbiguousPodcast) {
		t.Fatalf("expected ErrAmbiguousPodcast for 'Daily', got %v", errAmb)
	}
}
