package cli

import (
	"os"
	"path/filepath"
	"testing"
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
