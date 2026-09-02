package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func setupMockABSOPMLServer(createdPodcastTitle *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []Library{
					{ID: "lib-1", Name: "Podcasts", MediaType: "podcast", Folders: []LibraryFolder{{ID: "f-1", FullPath: "/podcasts"}}},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{{"id": "pod-1"}},
			})
		case r.URL.Path == "/api/items/pod-1":
			var p Podcast
			p.ID = "pod-1"
			p.Media.Metadata.Title = "Exported Show"
			p.Media.Metadata.FeedURL = "https://example.com/feed1.xml"
			_ = json.NewEncoder(w).Encode(p)
		case r.URL.Path == "/api/feeds":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"feeds": []map[string]string{{"entityId": "pod-1", "slug": "exported-slug"}},
			})
		case r.URL.Path == "/api/podcasts" && r.Method == "POST":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if media, ok := body["media"].(map[string]interface{}); ok {
				if meta, ok := media["metadata"].(map[string]interface{}); ok {
					*createdPodcastTitle, _ = meta["title"].(string)
				}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "pod-imported"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAudiobookshelfOPMLExportAndImport(t *testing.T) {
	var createdPodcastTitle string
	srv := setupMockABSOPMLServer(&createdPodcastTitle)
	defer srv.Close()

	be := NewAudiobookshelf(Config{Host: srv.URL, Token: "tok"})

	opmlBytes, err := be.ExportOPML(OPMLExportOptions{Quiet: true})
	if err != nil {
		t.Fatalf("ExportOPML failed: %v", err)
	}
	if !strings.Contains(string(opmlBytes), "Exported Show") {
		t.Errorf("expected Exported Show in OPML: %s", string(opmlBytes))
	}

	importXML := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Import Test</title></head>
  <body>
    <outline text="Podcasts">
      <outline type="rss" text="Newly Subscribed Show" xmlUrl="https://example.com/new-feed.xml" />
    </outline>
  </body>
</opml>`

	impRes, err := be.ImportOPML([]byte(importXML), OPMLImportOptions{Quiet: true})
	if err != nil {
		t.Fatalf("ImportOPML failed: %v", err)
	}
	if impRes.Subscribed != 1 {
		t.Errorf("expected 1 subscribed, got %d", impRes.Subscribed)
	}
	if createdPodcastTitle != "Newly Subscribed Show" {
		t.Errorf("expected Newly Subscribed Show created, got %s", createdPodcastTitle)
	}
}

func TestAudiobookshelfApplyKeepPolicy(t *testing.T) {
	var deletedIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/items/pod-1":
			var p Podcast
			p.ID = "pod-1"
			p.Media.Metadata.Title = "Daily Show"
			p.Media.Episodes = []Episode{
				{ID: "ep-1", Title: "Old Ep", PublishedAt: 1000},
				{ID: "ep-2", Title: "Middle Ep", PublishedAt: 2000},
				{ID: "ep-3", Title: "New Ep", PublishedAt: 3000},
			}
			_ = json.NewEncoder(w).Encode(p)
		case strings.HasPrefix(r.URL.Path, "/api/podcasts/pod-1/episode/"):
			epID := filepath.Base(strings.TrimSuffix(r.URL.Path, "?hard=1"))
			deletedIDs = append(deletedIDs, epID)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	be := NewAudiobookshelf(Config{
		Host:  srv.URL,
		Token: "tok",
	})

	deletedCount, err := be.ApplyKeepPolicy("pod-1", "Daily Show", 2, false, false, true)
	if err != nil {
		t.Fatalf("ApplyKeepPolicy failed: %v", err)
	}
	if deletedCount != 1 {
		t.Errorf("expected 1 episode deleted, got %d", deletedCount)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != "ep-1" {
		t.Errorf("expected ep-1 deleted, got: %v", deletedIDs)
	}
}
