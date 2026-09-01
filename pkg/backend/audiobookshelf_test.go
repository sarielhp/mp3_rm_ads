package backend

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAudiobookshelfLoginAndTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"user": map[string]string{"accessToken": "test-token-123"},
			})
			return
		}
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	be := NewAudiobookshelf(Config{
		Host: srv.URL,
		User: "user",
		Pass: "pass",
	})

	ok, err := be.TestConnection(true)
	if !ok || err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if be.Token != "test-token-123" {
		t.Errorf("expected token test-token-123, got %s", be.Token)
	}
}

func TestAudiobookshelfLibrariesAndPodcasts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []Library{
					{ID: "lib-pod", Name: "Podcasts", MediaType: "podcast"},
					{ID: "lib-book", Name: "Audiobooks", MediaType: "book"},
				},
			})
		case r.URL.Path == "/api/libraries/lib-pod/items":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{
					{"id": "pod-1"},
				},
			})
		case r.URL.Path == "/api/items/pod-1":
			var p Podcast
			p.ID = "pod-1"
			p.Media.Metadata.Title = "Test Podcast"
			p.Media.Metadata.FeedURL = "https://example.com/feed.xml"
			p.Media.Episodes = []Episode{
				{ID: "ep-1", Title: "Episode 1"},
			}
			_ = json.NewEncoder(w).Encode(p)
		case r.URL.Path == "/api/podcasts" && r.Method == "POST":
			var p Podcast
			p.ID = "pod-new"
			p.Media.Metadata.Title = "New Podcast"
			_ = json.NewEncoder(w).Encode(p)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	be := NewAudiobookshelf(Config{
		Host:  srv.URL,
		Token: "test-token",
	})

	libs, err := be.PodcastLibraries()
	if err != nil {
		t.Fatalf("PodcastLibraries failed: %v", err)
	}
	if len(libs) != 1 || libs[0].ID != "lib-pod" {
		t.Errorf("unexpected podcast libraries: %+v", libs)
	}

	podcasts, err := be.Podcasts()
	if err != nil {
		t.Fatalf("Podcasts failed: %v", err)
	}
	if len(podcasts) != 1 || podcasts[0].ID != "pod-1" || podcasts[0].Media.Metadata.Title != "Test Podcast" {
		t.Errorf("unexpected podcasts: %+v", podcasts)
	}

	created, err := be.CreatePodcast("lib-pod", "folder-1", "/path", "New Podcast", "https://example.com/new.xml")
	if err != nil {
		t.Fatalf("CreatePodcast failed: %v", err)
	}
	if created.ID != "pod-new" {
		t.Errorf("expected ID pod-new, got %s", created.ID)
	}
}

func TestAudiobookshelfEpisodesOperations(t *testing.T) {
	var downloadedCount int
	var deletedEpisode string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/podcasts/feed":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"podcast": map[string]interface{}{
					"episodes": []FeedEpisode{
						{Title: "Ep 1", GUID: "g-1"},
					},
				},
			})
		case r.URL.Path == "/api/podcasts/pod-1/download-episodes":
			var eps []FeedEpisode
			_ = json.NewDecoder(r.Body).Decode(&eps)
			downloadedCount = len(eps)
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api/podcasts/pod-1/episode/"):
			deletedEpisode = filepath.Base(strings.TrimSuffix(r.URL.Path, "?hard=1"))
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/podcasts/pod-1/downloads":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"downloads": []ActiveDownload{
					{ID: "dl-1", Title: "Ep 1"},
				},
			})
		case r.URL.Path == "/api/feeds/item/pod-1/open":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"slug": "pod-1-slug",
			})
		case r.URL.Path == "/api/items/pod-1/cover":
			w.Write([]byte("fake-cover-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	be := NewAudiobookshelf(Config{
		Host:  srv.URL,
		Token: "test-token",
	})

	eps, err := be.PodcastFeedEpisodes("https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("PodcastFeedEpisodes failed: %v", err)
	}
	if len(eps) != 1 || eps[0].Title != "Ep 1" {
		t.Errorf("unexpected feed episodes: %+v", eps)
	}

	if err := be.DownloadEpisodes("pod-1", eps); err != nil {
		t.Fatalf("DownloadEpisodes failed: %v", err)
	}
	if downloadedCount != 1 {
		t.Errorf("expected 1 downloaded episode, got %d", downloadedCount)
	}

	if err := be.DeletePodcastEpisode("pod-1", "ep-99"); err != nil {
		t.Fatalf("DeletePodcastEpisode failed: %v", err)
	}
	if deletedEpisode != "ep-99" {
		t.Errorf("expected deleted episode ep-99, got %s", deletedEpisode)
	}

	dls, err := be.ActiveDownloads("pod-1")
	if err != nil {
		t.Fatalf("ActiveDownloads failed: %v", err)
	}
	if len(dls) != 1 || dls[0].ID != "dl-1" {
		t.Errorf("unexpected active downloads: %+v", dls)
	}

	slug, err := be.OpenRSSFeed("pod-1", srv.URL)
	if err != nil || slug != "pod-1-slug" {
		t.Errorf("unexpected open RSS feed slug: %s, err: %v", slug, err)
	}

	tempDir := t.TempDir()
	coverFile := filepath.Join(tempDir, "cover.jpg")
	if err := be.DownloadCover("pod-1", coverFile); err != nil {
		t.Fatalf("DownloadCover failed: %v", err)
	}
	data, _ := os.ReadFile(coverFile)
	if string(data) != "fake-cover-bytes" {
		t.Errorf("unexpected cover file data: %s", string(data))
	}
}

func TestAudiobookshelfDBOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, token TEXT, createdAt DATETIME);
		CREATE TABLE podcasts (id TEXT PRIMARY KEY, title TEXT, lastEpisodeCheck DATETIME, maxNewEpisodesToDownload INT);
		INSERT INTO users VALUES ('u1', 'db-token-xyz', '2026-08-30 00:00:00');
		INSERT INTO podcasts VALUES ('p1', 'Show 1', '2026-08-30 12:00:00', 5);
	`)
	if err != nil {
		t.Fatalf("failed to setup test db: %v", err)
	}

	token := GetTokenFromDB(dbPath)
	if token != "db-token-xyz" {
		t.Errorf("expected db-token-xyz, got %s", token)
	}

	be := NewAudiobookshelf(Config{DBPath: dbPath})
	if be.Token != "db-token-xyz" {
		t.Errorf("expected auto loaded token db-token-xyz, got %s", be.Token)
	}

	if err := be.ResetPodcastDateCheck("p1", "Show 1"); err != nil {
		t.Fatalf("ResetPodcastDateCheck failed: %v", err)
	}

	var lastCheck string
	var maxDL int
	_ = db.QueryRow("SELECT lastEpisodeCheck, maxNewEpisodesToDownload FROM podcasts WHERE id = 'p1'").Scan(&lastCheck, &maxDL)
	if !strings.HasPrefix(lastCheck, "1970-01-01") || maxDL != 0 {
		t.Errorf("reset failed in DB: lastCheck=%s maxDL=%d", lastCheck, maxDL)
	}
}
