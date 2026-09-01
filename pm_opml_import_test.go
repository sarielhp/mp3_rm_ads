package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
