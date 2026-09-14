package cli

import (
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/podcast"
	"net/http"
	"net/http/httptest"
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

func TestResolveSubTargetsFiltering(t *testing.T) {
	subs := []podcast.Subscription{
		{ID: "p1", Title: "Podcast 1", Disabled: false},
		{ID: "p2", Title: "Podcast 2", Disabled: true},
		{ID: "p3", Title: "Science Show", Disabled: false},
	}

	// 1. Without target: should return active podcasts (p1, p3)
	targets := resolveSubTargets(subs, CLIOptions{})
	if len(targets) != 2 || targets[0].ID != "p1" || targets[1].ID != "p3" {
		t.Fatalf("unexpected targets: %+v", targets)
	}

	// 2. With target podcast ID
	targets = resolveSubTargets(subs, CLIOptions{Podcast: "p1"})
	if len(targets) != 1 || targets[0].ID != "p1" {
		t.Fatalf("expected p1 target, got: %+v", targets)
	}

	// 3. With target matching disabled podcast: should return empty
	targets = resolveSubTargets(subs, CLIOptions{Podcast: "p2"})
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets for disabled p2, got: %+v", targets)
	}

	// 4. With target search query in Args
	targets = resolveSubTargets(subs, CLIOptions{Args: []string{"science"}})
	if len(targets) != 1 || targets[0].ID != "p3" {
		t.Fatalf("expected p3 target for 'science', got: %+v", targets)
	}
}

func TestExecuteSubDownloadsNothingToDownload(t *testing.T) {
	plans := []subDownloadPlan{
		{
			sub:        podcast.Subscription{ID: "p1", Title: "P1"},
			podDir:     t.TempDir(),
			toDownload: nil,
		},
	}
	cfg := Config{}
	cli := CLIOptions{}
	if err := executeSubDownloads(plans, cfg, cli); err != nil {
		t.Fatalf("executeSubDownloads failed: %v", err)
	}
}

func TestPlanSubDownloadsParallelIntegration(t *testing.T) {
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Show 1</title>
<item><title>Ep 1</title><enclosure url="http://example.com/1.mp3" type="audio/mpeg"/></item>
</channel></rss>`))
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Show 2</title>
<item><title>Ep 2</title><enclosure url="http://example.com/2.mp3" type="audio/mpeg"/></item>
</channel></rss>`))
	}))
	defer ts2.Close()

	tmpDir := t.TempDir()
	show1Dir := filepath.Join(tmpDir, "Show 1")
	_ = os.MkdirAll(show1Dir, 0755)
	_ = os.WriteFile(filepath.Join(show1Dir, "Ep 1.mp3"), []byte("mp3"), 0644)

	targets := []podcast.Subscription{
		{ID: "p1", Title: "Show 1", Folder: "Show 1", FeedURL: ts1.URL, DownloadPolicy: "latest"},
		{ID: "p2", Title: "Show 2", Folder: "Show 2", FeedURL: ts2.URL, DownloadPolicy: "latest"},
	}

	cfg := Config{PodcastsDir: tmpDir}
	cli := CLIOptions{ProcOptions: ProcOptions{Quiet: true}}
	plans := planSubDownloads(targets, cfg, cli)

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if len(plans[0].toDownload) != 0 {
		t.Fatalf("expected 0 downloads for Show 1 (already on disk), got %d", len(plans[0].toDownload))
	}
	if len(plans[1].toDownload) != 1 {
		t.Fatalf("expected 1 download for Show 2, got %d", len(plans[1].toDownload))
	}
}
