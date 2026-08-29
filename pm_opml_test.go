package main

import (
	"encoding/json"
	"fmt"
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

	exportOPML(cfg, outPath, true, false)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read generated OPML file: %v", err)
	}
	if !strings.Contains(string(content), "daily-news") {
		t.Errorf("generated OPML does not contain expected feed: %s", string(content))
	}
}

func TestParseOPMLXML(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Subscriptions</title></head>
  <body>
    <outline text="News Category">
      <outline type="rss" text="The Daily" xmlUrl="https://feeds.simplecast.com/qm_9iD0J" />
    </outline>
    <outline type="rss" title="Hard Fork" xmlUrl="https://feeds.simplecast.com/54nAGcIl" />
  </body>
</opml>`

	feeds, err := parseOPMLXML([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseOPMLXML failed: %v", err)
	}

	if len(feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(feeds))
	}
	if feeds[0].Title != "The Daily" || feeds[0].URL != "https://feeds.simplecast.com/qm_9iD0J" {
		t.Errorf("unexpected feed 0: %+v", feeds[0])
	}
	if feeds[1].Title != "Hard Fork" || feeds[1].URL != "https://feeds.simplecast.com/54nAGcIl" {
		t.Errorf("unexpected feed 1: %+v", feeds[1])
	}
}

func TestImportOPML(t *testing.T) {
	createdFeeds := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/libraries":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{
						"id":        "lib-1",
						"name":      "Podcasts",
						"mediaType": "podcast",
						"folders": []map[string]interface{}{
							{"id": "folder-1", "fullPath": "/podcasts/clean"},
						},
					},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "item-existing"},
				},
			})
		case r.URL.Path == "/api/items/item-existing":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-existing",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{
						"title":   "Existing Podcast",
						"feedUrl": "https://example.com/existing.xml",
					},
				},
			})
		case r.URL.Path == "/api/podcasts" && r.Method == "POST":
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			media := payload["media"].(map[string]interface{})
			meta := media["metadata"].(map[string]interface{})
			createdFeeds = append(createdFeeds, meta["feedUrl"].(string))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-new",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	opmlFile := filepath.Join(tempDir, "test.opml")
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline type="rss" text="Existing Podcast" xmlUrl="https://example.com/existing.xml" />
    <outline type="rss" text="Brand New Podcast" xmlUrl="https://example.com/new.xml" />
  </body>
</opml>`
	if err := os.WriteFile(opmlFile, []byte(xmlData), 0644); err != nil {
		t.Fatalf("failed to write test opml: %v", err)
	}

	cfg := Config{
		AudiobookshelfURL:   server.URL,
		AudiobookshelfToken: "test-token",
	}

	importOPML(cfg, opmlFile, true, false)

	if len(createdFeeds) != 1 {
		t.Fatalf("expected 1 newly created feed subscription, got %d", len(createdFeeds))
	}
	if createdFeeds[0] != "https://example.com/new.xml" {
		t.Errorf("expected subscription to https://example.com/new.xml, got %s", createdFeeds[0])
	}
}

func TestImportOPMLSkipsSelfFeeds(t *testing.T) {
	createdFeeds := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/libraries":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{
						"id":        "lib-1",
						"name":      "Podcasts",
						"mediaType": "podcast",
						"folders": []map[string]interface{}{
							{"id": "folder-1", "fullPath": "/podcasts/clean"},
						},
					},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{},
			})
		case r.URL.Path == "/api/feeds":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"feeds": []map[string]interface{}{
					{
						"entityId": "item-1",
						"slug":     "local-feed-slug",
						"feedUrl":  "/feed/local-feed-slug",
					},
				},
			})
		case r.URL.Path == "/api/podcasts" && r.Method == "POST":
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			media := payload["media"].(map[string]interface{})
			meta := media["metadata"].(map[string]interface{})
			createdFeeds = append(createdFeeds, meta["feedUrl"].(string))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-new",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	opmlFile := filepath.Join(tempDir, "test.opml")
	xmlData := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline type="rss" text="ABS Self Feed" xmlUrl="%s/feed/local-feed-slug" />
    <outline type="rss" text="External Podcast" xmlUrl="https://example.com/external.xml" />
  </body>
</opml>`, server.URL)
	if err := os.WriteFile(opmlFile, []byte(xmlData), 0644); err != nil {
		t.Fatalf("failed to write test opml: %v", err)
	}

	cfg := Config{
		AudiobookshelfURL:   server.URL,
		AudiobookshelfToken: "test-token",
	}

	importOPML(cfg, opmlFile, true, false)

	if len(createdFeeds) != 1 {
		t.Fatalf("expected exactly 1 subscription (self-feed skipped), got %d", len(createdFeeds))
	}
	if createdFeeds[0] != "https://example.com/external.xml" {
		t.Errorf("expected subscription to https://example.com/external.xml, got %s", createdFeeds[0])
	}
}
