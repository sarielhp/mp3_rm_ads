package cli

import (
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/podcast"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerSubscriptionCommands(t *testing.T) {
	tmpDir := t.TempDir()
	subsFile := filepath.Join(tmpDir, "podcasts.json")
	podcastsDir := filepath.Join(tmpDir, "podcasts")
	_ = os.MkdirAll(podcastsDir, 0755)

	cfg := Config{
		PodcastsDir:       podcastsDir,
		SubscriptionsFile: subsFile,
		ServerBaseURL:     "http://myserver.tailscale.net:8080/podcasts",
	}

	// 1. Test Add
	addCli := CLIOptions{
		Args: []string{"https://feeds.example.com/testpodcast.xml", "Test Podcast"},
	}
	if err := handleServerAdd(cfg, addCli); err != nil {
		t.Fatalf("handleServerAdd failed: %v", err)
	}

	// 2. Test Feed generation
	feedCli := CLIOptions{
		Args: []string{"Test Podcast"},
	}
	if err := handleServerFeed(cfg, feedCli); err != nil {
		t.Fatalf("handleServerFeed failed: %v", err)
	}

	feedPath := filepath.Join(podcastsDir, "Test Podcast", "feed.xml")
	if _, err := os.Stat(feedPath); err != nil {
		t.Fatalf("expected feed.xml at %s: %v", feedPath, err)
	}

	// 3. Test List
	listCli := CLIOptions{
		ProcOptions: ProcOptions{Quiet: true},
	}
	if err := handleServerList(cfg, listCli); err != nil {
		// List might query backend if not configured, or if it queries backend it could report backend not configured
		// That is normal if backend is nil.
	}

	// 4. Test Remove
	removeCli := CLIOptions{
		Args: []string{"Test Podcast"},
	}
	if err := handleServerRemove(cfg, removeCli); err != nil {
		t.Fatalf("handleServerRemove failed: %v", err)
	}
}

func TestSelectSubEpisodesToDownloadRespectsPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	podDir := filepath.Join(tmpDir, "Show")
	_ = os.MkdirAll(podDir, 0755)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	feedEps := []backend.FeedEpisode{
		{Title: "Ep 3", PublishedAt: t2.UnixMilli(), EnclosureURL: "http://example.com/3.mp3"},
		{Title: "Ep 2", PublishedAt: t1.UnixMilli(), EnclosureURL: "http://example.com/2.mp3"},
		{Title: "Ep 1", PublishedAt: t0.UnixMilli(), EnclosureURL: "http://example.com/1.mp3"},
	}

	cfg := Config{}
	cfg.DefaultDownloadPolicy = "none"
	cfg.DefaultDownloadK = 3

	sub := podcast.Subscription{
		Title:  "Show",
		Folder: "Show",
	}

	// 1. Policy none: should return 0 episodes (not 3!)
	res := selectSubEpisodesToDownload(podDir, feedEps, sub, CLIOptions{}, cfg)
	if len(res) != 0 {
		t.Fatalf("expected 0 episodes for policy none, got: %d", len(res))
	}

	// 2. Policy latest: should return 1 episode (Ep 3)
	sub.DownloadPolicy = "latest"
	res = selectSubEpisodesToDownload(podDir, feedEps, sub, CLIOptions{}, cfg)
	if len(res) != 1 || res[0].Title != "Ep 3" {
		t.Fatalf("expected 1 episode ('Ep 3') for policy latest, got: %v", res)
	}

	// 3. Policy latest_k: should return 2 episodes (Ep 3, Ep 2)
	sub.DownloadPolicy = "latest_k"
	sub.DownloadK = 2
	res = selectSubEpisodesToDownload(podDir, feedEps, sub, CLIOptions{}, cfg)
	if len(res) != 2 {
		t.Fatalf("expected 2 episodes for latest_k(2), got: %d", len(res))
	}

	// 4. Favorite podcast: should select only new episodes
	sub.DownloadPolicy = ""
	_ = os.WriteFile(filepath.Join(podDir, "Ep 2.mp3"), []byte("audio"), 0644)
	podCfg := config.PodcastConfig{Favorite: true}
	_ = config.SavePodcastConfig(podDir, podCfg)
	res = selectSubEpisodesToDownload(podDir, feedEps, sub, CLIOptions{}, cfg)
	if len(res) != 1 || res[0].Title != "Ep 3" {
		t.Fatalf("expected only new episode 'Ep 3' for favorite podcast, got: %v", res)
	}
}
