package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOPMLXML(t *testing.T) {
	feeds := []PodcastFeedInfo{
		{
			Title: "Test & Podcast <1>",
			URL:   "http://localhost:8087/feed/test-feed-1",
		},
		{
			Title: `Quotes "Podcast" 'Two'`,
			URL:   "http://localhost:8087/feed/test-feed-2",
		},
	}

	data, err := buildOPMLXML(feeds)
	if err != nil {
		t.Fatalf("buildOPMLXML failed: %v", err)
	}

	xmlStr := string(data)
	if !strings.HasPrefix(xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n") {
		t.Errorf("expected XML header prefix, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `<opml version="2.0">`) {
		t.Errorf("expected opml version 2.0 root element, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `<title>Audiobookshelf Podcast Feeds</title>`) {
		t.Errorf("expected head title element, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `<outline text="Audiobookshelf Podcasts">`) {
		t.Errorf("expected body outline group, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="http://localhost:8087/feed/test-feed-1"`) {
		t.Errorf("expected xmlUrl for feed 1, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `text="Test &amp; Podcast &lt;1&gt;"`) {
		t.Errorf("expected escaped text for feed 1, got %q", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="http://localhost:8087/feed/test-feed-2"`) {
		t.Errorf("expected xmlUrl for feed 2, got %q", xmlStr)
	}
}

func TestBuildOPMLXMLEmpty(t *testing.T) {
	data, err := buildOPMLXML(nil)
	if err != nil {
		t.Fatalf("buildOPMLXML failed: %v", err)
	}
	xmlStr := string(data)
	if !strings.Contains(xmlStr, `<opml version="2.0">`) {
		t.Errorf("expected valid empty OPML, got %q", xmlStr)
	}
}

func TestFetchPodcastFeedsAndOpenFeed(t *testing.T) {
	feedOpened := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{"id": "lib-1", "name": "Podcasts", "mediaType": "podcast"},
					{"id": "lib-2", "name": "Audiobooks", "mediaType": "book"},
				},
			})
		case "/api/libraries/lib-1/items":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "item-1",
						"media": map[string]interface{}{
							"metadata": map[string]interface{}{
								"title": "Open Feed Podcast",
							},
						},
					},
					{
						"id": "item-2",
						"media": map[string]interface{}{
							"metadata": map[string]interface{}{
								"title": "Closed Feed Podcast",
							},
						},
					},
				},
			})
		case "/api/items/item-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-1",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{
						"title": "Open Feed Podcast",
					},
				},
				"rssFeed": map[string]interface{}{
					"id":   "feed-1",
					"slug": "open-slug",
				},
			})
		case "/api/items/item-2":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-2",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{
						"title": "Closed Feed Podcast",
					},
				},
				"rssFeed": nil,
			})
		case "/api/feeds/item/item-2/open":
			feedOpened = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"slug": "dynamically-opened-slug",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewABSClient(server.URL, "test-token")
	client.Quiet = true
	client.MaxAttempts = 1

	feeds, err := fetchPodcastFeeds(client, server.URL, true, false)
	if err != nil {
		t.Fatalf("fetchPodcastFeeds failed: %v", err)
	}

	if len(feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(feeds))
	}
	if !feedOpened {
		t.Errorf("expected closed feed to be dynamically opened via API")
	}
	if feeds[0].URL != server.URL+"/feed/open-slug" {
		t.Errorf("expected open feed URL %q, got %q", server.URL+"/feed/open-slug", feeds[0].URL)
	}
	if feeds[1].URL != server.URL+"/feed/dynamically-opened-slug" {
		t.Errorf("expected dynamically opened feed URL %q, got %q", server.URL+"/feed/dynamically-opened-slug", feeds[1].URL)
	}
}

func TestExportOPMLToFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{"id": "lib-1", "name": "Podcasts", "mediaType": "podcast"},
				},
			})
		case "/api/libraries/lib-1/items":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "item-1",
						"media": map[string]interface{}{
							"metadata": map[string]interface{}{
								"title": "Daily News",
							},
						},
					},
				},
			})
		case "/api/items/item-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-1",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{
						"title": "Daily News",
					},
				},
				"rssFeed": map[string]interface{}{
					"slug": "daily-news",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "sub", "feeds.opml")

	cfg := Config{
		AudiobookshelfURL:   server.URL,
		AudiobookshelfToken: "test-token",
	}
	cli := CLIOptions{
		Output: outPath,
		Quiet:  true,
	}

	exportOPML(cfg, cli)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read generated OPML file: %v", err)
	}
	if !strings.Contains(string(content), "daily-news") {
		t.Errorf("generated OPML does not contain expected feed: %s", string(content))
	}
}
