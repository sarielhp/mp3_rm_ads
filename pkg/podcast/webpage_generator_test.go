package podcast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePodcastWebpage(t *testing.T) {
	podDir := t.TempDir()
	sub := Subscription{
		ID:       "pod1",
		Title:    "History Show",
		FeedURL:  "https://example.com/feed.xml",
		Folder:   "History Show",
		ImageURL: "https://example.com/cover.jpg",
	}

	episodes := []LocalEpisodeMeta{
		{
			Path:        filepath.Join(podDir, "Episode 1.mp3"),
			Filename:    "Episode 1.mp3",
			Title:       "Episode 1: The Beginning",
			DurationSec: 3600,
			SizeBytes:   50000000,
			PubDate:     "Mon, 01 Jan 2026 12:00:00 +0000",
			Description: "First episode of the show.",
		},
	}

	htmlData, err := GeneratePodcastWebpageHTML(sub, podDir, episodes, "http://localhost:8080/podcasts")
	if err != nil {
		t.Fatalf("GeneratePodcastWebpageHTML failed: %v", err)
	}

	content := string(htmlData)
	if !strings.Contains(content, "History Show") {
		t.Errorf("expected HTML to contain title 'History Show'")
	}
	if !strings.Contains(content, "Episode 1: The Beginning") {
		t.Errorf("expected HTML to contain episode title")
	}
	if !strings.Contains(content, "<audio controls") {
		t.Errorf("expected HTML to contain audio player")
	}

	if err := WritePodcastWebpage(podDir, sub, "http://localhost:8080/podcasts", nil); err != nil {
		t.Fatalf("WritePodcastWebpage failed: %v", err)
	}

	idxPath := filepath.Join(podDir, "index.html")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("expected index.html to exist at %s: %v", idxPath, err)
	}
}

func TestGenerateCatalogWebpage(t *testing.T) {
	podcastsDir := t.TempDir()
	subs := []Subscription{
		{
			ID:      "show1",
			Title:   "Alpha Show",
			FeedURL: "https://example.com/alpha.xml",
			Folder:  "Alpha Show",
		},
		{
			ID:      "show2",
			Title:   "Beta Show",
			FeedURL: "https://example.com/beta.xml",
			Folder:  "Beta Show",
		},
	}

	htmlData, err := GenerateCatalogWebpageHTML(podcastsDir, subs, "http://localhost:8080/podcasts")
	if err != nil {
		t.Fatalf("GenerateCatalogWebpageHTML failed: %v", err)
	}

	content := string(htmlData)
	if !strings.Contains(content, "Alpha Show") || !strings.Contains(content, "Beta Show") {
		t.Errorf("expected catalog HTML to contain both show titles")
	}

	if err := WriteCatalogWebpage(podcastsDir, subs, "http://localhost:8080/podcasts"); err != nil {
		t.Fatalf("WriteCatalogWebpage failed: %v", err)
	}

	idxPath := filepath.Join(podcastsDir, "index.html")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("expected catalog index.html to exist at %s: %v", idxPath, err)
	}
}
