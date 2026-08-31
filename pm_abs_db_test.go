package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestResetPodcastDateCheckInDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE podcasts (
			id UUID PRIMARY KEY,
			title VARCHAR(255),
			lastEpisodeCheck DATETIME,
			maxNewEpisodesToDownload INTEGER
		);
		INSERT INTO podcasts (id, title, lastEpisodeCheck, maxNewEpisodesToDownload)
		VALUES ('pod-1', 'History Podcast', '2026-08-30 12:00:00', 3);
	`)
	if err != nil {
		t.Fatalf("failed to init db tables: %v", err)
	}

	err = resetPodcastDateCheckInDB(dbPath, "pod-1", "History Podcast")
	if err != nil {
		t.Fatalf("resetPodcastDateCheckInDB failed: %v", err)
	}

	var lastCheck string
	var maxDL int
	err = db.QueryRow("SELECT lastEpisodeCheck, maxNewEpisodesToDownload FROM podcasts WHERE id = 'pod-1'").Scan(&lastCheck, &maxDL)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !strings.HasPrefix(lastCheck, "1970-01-01") {
		t.Errorf("expected lastEpisodeCheck to start with 1970-01-01, got %s", lastCheck)
	}
	if maxDL != 0 {
		t.Errorf("expected maxNewEpisodesToDownload to be 0, got %d", maxDL)
	}
}

func TestResetPodcastDateCheckAPI(t *testing.T) {
	var receivedPatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" && (r.URL.Path == "/api/items/pod-1/media" || r.URL.Path == "/api/items/pod-1") {
			receivedPatch = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewABSClient(server.URL, "token")
	err := client.ResetPodcastDateCheckAPI("pod-1")
	if err != nil {
		t.Fatalf("ResetPodcastDateCheckAPI failed: %v", err)
	}
	if !receivedPatch {
		t.Errorf("expected PATCH request to be sent to ABS API")
	}
}
