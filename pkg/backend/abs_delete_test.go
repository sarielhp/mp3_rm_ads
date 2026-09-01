package backend

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAudiobookshelfDeletePodcast(t *testing.T) {
	var deletedItemPath string
	var deleteMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleteMethod = r.Method
		deletedItemPath = r.URL.Path
		if r.Method == "DELETE" && r.URL.Path == "/api/items/pod-123" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	be := NewAudiobookshelf(Config{
		Host:  srv.URL,
		Token: "test-token",
	})

	if err := be.DeletePodcast("pod-123"); err != nil {
		t.Fatalf("DeletePodcast failed: %v", err)
	}
	if deleteMethod != "DELETE" || deletedItemPath != "/api/items/pod-123" {
		t.Errorf("expected DELETE /api/items/pod-123, got %s %s", deleteMethod, deletedItemPath)
	}

	if err := be.DeleteItem("pod-123"); err != nil {
		t.Fatalf("DeleteItem failed: %v", err)
	}

	if err := be.DeletePodcast("non-existent"); err == nil {
		t.Errorf("expected error for 404 response on non-existent item")
	}
}

func TestPodFetchDeletePodcastDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "podfetch.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE podcasts (id INTEGER PRIMARY KEY, name TEXT, directory TEXT, rssfeed TEXT, created_at DATETIME);
		CREATE TABLE podcast_episodes (id INTEGER PRIMARY KEY, podcast_id INTEGER, episode_id TEXT, name TEXT, url TEXT);
		INSERT INTO podcasts VALUES (10, 'Test Pod', 'test_pod', 'https://example.com/rss', '2026-08-30 00:00:00');
		INSERT INTO podcast_episodes VALUES (100, 10, 'ep-1', 'Ep 1', 'https://example.com/ep1.mp3');
	`)
	if err != nil {
		t.Fatalf("failed to setup test db: %v", err)
	}

	be := &PodFetchBackend{DBPath: dbPath}
	if err := be.DeletePodcast("10"); err != nil {
		t.Fatalf("DeletePodcast failed: %v", err)
	}

	var podCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM podcasts WHERE id = 10").Scan(&podCount)
	if podCount != 0 {
		t.Errorf("expected podcast to be deleted from db, found count %d", podCount)
	}

	var epCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM podcast_episodes WHERE podcast_id = 10").Scan(&epCount)
	if epCount != 0 {
		t.Errorf("expected episodes to be deleted from db, found count %d", epCount)
	}
}
