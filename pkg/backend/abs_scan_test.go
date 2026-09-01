package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAudiobookshelfSyncDurationAndScan(t *testing.T) {
	var scannedItem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"results": []map[string]string{
					{"id": "pod-123"},
				},
			})
		case r.URL.Path == "/api/items/pod-123":
			var p Podcast
			p.ID = "pod-123"
			p.RelPath = "Tech_Talk"
			p.Media.Metadata.Title = "Tech_Talk"
			_ = json.NewEncoder(w).Encode(p)
		case strings.HasPrefix(r.URL.Path, "/api/items/pod-123/scan"):
			scannedItem = "pod-123"
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api/items/pod-123/cover"):
			w.Write([]byte("img"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	be := NewAudiobookshelf(Config{
		Host:        srv.URL,
		Token:       "tok",
		PodcastsDir: tempDir,
	})

	err := be.SyncDuration(filepath.Join(tempDir, "Tech_Talk", "ep1.mp3"), 100.0)
	if err != nil {
		t.Fatalf("SyncDuration failed: %v", err)
	}
	if scannedItem != "pod-123" {
		t.Errorf("expected scannedItem pod-123, got %s", scannedItem)
	}

	scanRes, err := be.Scan(ScanOptions{
		PodcastsDir: tempDir,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if scanRes.NewPodcasts != 1 {
		t.Errorf("expected 1 new podcast detected, got %d", scanRes.NewPodcasts)
	}
}
