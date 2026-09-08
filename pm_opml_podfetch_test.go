package main

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
	"github.com/sariel/abs/pkg/backend"
)

func resetBackendState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		backend.SetAudiobookshelfDisabled(false)
		backend.SetPodfetchDisabled(false)
	})
}

func setupMockPodFetchServer(t *testing.T) *httptest.Server {
	podcastList := []map[string]interface{}{
		{
			"id":        "1",
			"name":      "Tech & Gadgets <Daily>",
			"directory": "tech_gadgets",
			"rssfeed":   "https://example.com/tech.xml",
		},
		{
			"id":        "2",
			"name":      `History "Uncensored"`,
			"directory": "history_show",
			"rssfeed":   "https://example.com/history.xml",
		},
		{
			"id":        "3",
			"name":      "Science Hour",
			"directory": "science_hour",
			"rssfeed":   "https://example.com/science.xml",
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/podcasts":
			_ = json.NewEncoder(w).Encode(podcastList)
		case strings.HasPrefix(r.URL.Path, "/api/v1/podcasts/"):
			idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/podcasts/")
			for _, p := range podcastList {
				if p["id"] == idStr {
					_ = json.NewEncoder(w).Encode(p)
					return
				}
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func setupTestPodFetchSQLiteDB(t *testing.T) string {
	dbPath := filepath.Join(t.TempDir(), "podfetch_test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test sqlite db: %v", err)
	}
	defer db.Close()

	schema := `
		CREATE TABLE podcasts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			directory TEXT,
			rssfeed TEXT,
			image_url TEXT,
			summary TEXT,
			author TEXT,
			created_at TEXT
		);
		INSERT INTO podcasts (id, name, directory, rssfeed, created_at)
		VALUES 
			(101, 'DB Alpha Show', 'db_alpha', 'https://feeds.example.com/alpha.xml', '2026-08-01 00:00:00'),
			(102, 'DB Beta Show', 'db_beta', 'https://feeds.example.com/beta.xml', '2026-08-02 00:00:00'),
			(103, 'DB Gamma & Delta', 'db_gamma', 'https://feeds.example.com/gamma.xml', '2026-08-03 00:00:00');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to initialize sqlite schema: %v", err)
	}
	return dbPath
}

func TestPodFetchExportOPML_HTTPAPI(t *testing.T) {
	resetBackendState(t)
	srv := setupMockPodFetchServer(t)
	defer srv.Close()

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "exported_podfetch.opml")

	cfg := Config{
		BackendType: "podfetch",
		PodfetchURL: srv.URL,
	}

	exportOPML(cfg, outPath, true, false)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read exported opml: %v", err)
	}
	xmlStr := string(data)

	if !strings.HasPrefix(xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n") {
		t.Errorf("missing xml declaration header: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<opml version="2.0">`) {
		t.Errorf("expected opml version 2.0, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<title>PodFetch Podcast Feeds</title>`) {
		t.Errorf("expected PodFetch title, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<outline text="PodFetch Podcasts">`) {
		t.Errorf("expected PodFetch group outline, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="`+srv.URL+`/rss/1"`) {
		t.Errorf("expected feed 1 url in opml, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="`+srv.URL+`/rss/2"`) {
		t.Errorf("expected feed 2 url in opml, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="`+srv.URL+`/rss/3"`) {
		t.Errorf("expected feed 3 url in opml, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `text="Tech &amp; Gadgets &lt;Daily&gt;"`) {
		t.Errorf("expected properly escaped title with & and <>, got: %s", xmlStr)
	}
}

func TestPodFetchExportOPML_SQLiteDB(t *testing.T) {
	resetBackendState(t)
	dbPath := setupTestPodFetchSQLiteDB(t)
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "db_podfetch.opml")

	cfg := Config{
		BackendType:    "podfetch",
		PodfetchDBPath: dbPath,
	}

	exportOPML(cfg, outPath, true, false)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read db exported opml: %v", err)
	}
	xmlStr := string(data)

	if !strings.Contains(xmlStr, `xmlUrl="https://feeds.example.com/alpha.xml"`) {
		t.Errorf("expected DB alpha feed URL in opml, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `xmlUrl="https://feeds.example.com/beta.xml"`) {
		t.Errorf("expected DB beta feed URL in opml, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `text="DB Gamma &amp; Delta"`) {
		t.Errorf("expected DB gamma escaped title in opml, got: %s", xmlStr)
	}
}

func TestPodFetchExportOPML_RoundTrip(t *testing.T) {
	resetBackendState(t)
	srv := setupMockPodFetchServer(t)
	defer srv.Close()

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "roundtrip.opml")

	cfg := Config{
		BackendType: "podfetch",
		PodfetchURL: srv.URL,
	}

	exportOPML(cfg, outPath, true, false)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read opml file: %v", err)
	}

	feeds, err := parseOPMLXML(data)
	if err != nil {
		t.Fatalf("failed to parse generated OPML: %v", err)
	}

	if len(feeds) != 3 {
		t.Fatalf("expected 3 parsed feeds, got %d", len(feeds))
	}
	if feeds[0].Title != "Tech & Gadgets <Daily>" || feeds[0].URL != srv.URL+"/rss/1" {
		t.Errorf("unexpected feed 0: %+v", feeds[0])
	}
	if feeds[1].Title != `History "Uncensored"` || feeds[1].URL != srv.URL+"/rss/2" {
		t.Errorf("unexpected feed 1: %+v", feeds[1])
	}
	if feeds[2].Title != "Science Hour" || feeds[2].URL != srv.URL+"/rss/3" {
		t.Errorf("unexpected feed 2: %+v", feeds[2])
	}
}

func TestPodFetchSyncOPML_CLIExecution(t *testing.T) {
	resetBackendState(t)
	srv := setupMockPodFetchServer(t)
	defer srv.Close()

	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "cli_export", "podfetch.opml")

	cfg := Config{
		BackendType: "podfetch",
		PodfetchURL: srv.URL,
	}

	cli := CLIOptions{
		SyncSubcmd: "opml",
		OPMLSubcmd: "export",
		OPMLFile:   outPath,
		Quiet:      true,
	}

	handleServerOPML(cfg, cli)

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected exported opml file at %s: %v", outPath, err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read exported opml: %v", err)
	}
	if !strings.Contains(string(data), srv.URL+"/rss/1") {
		t.Errorf("exported file missing feed url 1: %s", string(data))
	}
}
