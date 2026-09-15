package podcast

import (
	"os"
	"path/filepath"
	"testing"

	"pod/pkg/backend"
)

func TestStandaloneBackend(t *testing.T) {
	podcastsDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "podcasts.json")

	store, err := NewSubscriptionStore(storePath)
	if err != nil {
		t.Fatalf("NewSubscriptionStore failed: %v", err)
	}

	sub := Subscription{
		ID:      "show1",
		Title:   "Great Podcast",
		FeedURL: "https://example.com/great.xml",
		Folder:  "Great Podcast",
	}
	if err := store.Add(sub); err != nil {
		t.Fatalf("store.Add failed: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("store.Save failed: %v", err)
	}

	// Create podcast directory with a test mp3 file
	showDir := filepath.Join(podcastsDir, "Great Podcast")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	epPath := filepath.Join(showDir, "Episode 1.mp3")
	if err := os.WriteFile(epPath, []byte("fake mp3 data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := backend.Config{
		PodcastsDir: podcastsDir,
	}
	be := &StandaloneBackend{
		cfg:        cfg,
		store:      store,
		downloader: NewDownloader(),
	}

	if be.Name() != "standalone" {
		t.Errorf("expected name 'standalone', got %s", be.Name())
	}

	pods, err := be.Podcasts()
	if err != nil {
		t.Fatalf("Podcasts() failed: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 podcast, got %d", len(pods))
	}
	if pods[0].Media.Metadata.Title != "Great Podcast" {
		t.Errorf("expected title 'Great Podcast', got %s", pods[0].Media.Metadata.Title)
	}
	if len(pods[0].Media.Episodes) != 1 {
		t.Errorf("expected 1 episode in podcast, got %d", len(pods[0].Media.Episodes))
	}

	// Test GetPodcast
	p, err := be.GetPodcast("show1")
	if err != nil || p == nil {
		t.Fatalf("GetPodcast('show1') failed: %v", err)
	}

	// Test Libraries
	libs, err := be.PodcastLibraries()
	if err != nil || len(libs) == 0 {
		t.Fatalf("PodcastLibraries() failed: %v", err)
	}

	// Test UpdatePodcastSettings
	if err := be.UpdatePodcastSettings("show1", true, false, 0); err != nil {
		t.Fatalf("UpdatePodcastSettings failed: %v", err)
	}

	// Test OPML export
	opmlData, err := be.ExportOPML(backend.OPMLExportOptions{})
	if err != nil || len(opmlData) == 0 {
		t.Fatalf("ExportOPML failed: %v", err)
	}

	// Test DeletePodcast
	if err := be.DeletePodcast("show1"); err != nil {
		t.Fatalf("DeletePodcast failed: %v", err)
	}
	podsAfterDelete, err := be.Podcasts()
	if err != nil {
		t.Fatalf("Podcasts() after delete failed: %v", err)
	}
	if len(podsAfterDelete) != 0 {
		t.Errorf("expected 0 podcasts after delete, got %d", len(podsAfterDelete))
	}
}
