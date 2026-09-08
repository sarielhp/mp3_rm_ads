package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func createTestModernPodfetchDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "modern_podfetch.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	schema := `
		CREATE TABLE podcasts (
			id TEXT PRIMARY KEY,
			name TEXT,
			directory_name TEXT,
			rssfeed TEXT,
			image_url TEXT,
			summary TEXT,
			author TEXT,
			created_at TEXT
		);
		CREATE TABLE episodes (
			id TEXT PRIMARY KEY,
			podcast_id TEXT,
			title TEXT,
			file_episode_path TEXT,
			episode_number INTEGER,
			created_at TEXT
		);
		INSERT INTO podcasts (id, name, directory_name, rssfeed, created_at)
		VALUES
			('01a05b11-c0fc-7211-87be-6d9cfdb4bd83', 'Podcast Alpha', 'podcasts/alpha', 'https://feeds.example.com/alpha.xml', '2026-09-08 00:00:00'),
			('01a05b11-d47d-70b1-87be-6d9cfdb4bd84', 'Podcast Beta & Special <Chars>', 'podcasts/beta', 'https://feeds.example.com/beta.xml', '2026-09-08 00:00:00');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to initialize modern schema: %v", err)
	}
	return dbPath
}

func createTestLegacyPodfetchDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "legacy_podfetch.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	schema := `
		CREATE TABLE podcasts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			directory TEXT,
			rssfeed TEXT,
			created_at TEXT
		);
		INSERT INTO podcasts (id, name, directory, rssfeed, created_at)
		VALUES
			(1, 'Legacy Show', 'legacy_show', 'https://feeds.example.com/legacy.xml', '2026-09-08 00:00:00');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to initialize legacy schema: %v", err)
	}
	return dbPath
}

func setupMockABSServerForOPML(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{"id": "lib-opml", "name": "Podcasts", "mediaType": "podcast"},
				},
			})
		case "/api/libraries/lib-opml/items":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "item-opml-1", "media": map[string]interface{}{"metadata": map[string]interface{}{"title": "ABS Feed Show"}}},
				},
			})
		case "/api/items/item-opml-1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "item-opml-1",
				"media":   map[string]interface{}{"metadata": map[string]interface{}{"title": "ABS Feed Show"}},
				"rssFeed": map[string]interface{}{"id": "feed-abs-1", "slug": "abs-show-slug"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestSyncOPMLExport_CLICommandParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	args := []string{"sync", "opml", "export", "file.opml"}
	if err := app.Execute(args); err != nil {
		t.Fatalf("Execute(%v) returned error: %v", args, err)
	}

	if action != "sync" {
		t.Errorf("expected action 'sync', got %q", action)
	}
	if opts.SyncSubcmd != "opml" {
		t.Errorf("expected SyncSubcmd 'opml', got %q", opts.SyncSubcmd)
	}
	if opts.ServerSubcmd != "opml" {
		t.Errorf("expected ServerSubcmd 'opml', got %q", opts.ServerSubcmd)
	}
	if opts.OPMLSubcmd != "export" {
		t.Errorf("expected OPMLSubcmd 'export', got %q", opts.OPMLSubcmd)
	}
	if opts.OPMLFile != "file.opml" {
		t.Errorf("expected OPMLFile 'file.opml', got %q", opts.OPMLFile)
	}
}

func TestSyncOPMLExport_CLICommandParsingWithFlags(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	args := []string{"sync", "opml", "export", "custom.opml", "--quiet", "--verbose"}
	if err := app.Execute(args); err != nil {
		t.Fatalf("Execute(%v) returned error: %v", args, err)
	}

	if opts.OPMLFile != "custom.opml" {
		t.Errorf("expected OPMLFile 'custom.opml', got %q", opts.OPMLFile)
	}
	if !opts.Quiet {
		t.Errorf("expected Quiet=true")
	}
	if !opts.Verbose {
		t.Errorf("expected Verbose=true")
	}
}

func TestSyncOPMLExport_ParseFlags(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"abs", "sync", "opml", "export", "file.opml"}
	action, opts := parseFlags()
	if action != "sync" {
		t.Errorf("expected action 'sync', got %q", action)
	}
	if opts.SyncSubcmd != "opml" {
		t.Errorf("expected SyncSubcmd 'opml', got %q", opts.SyncSubcmd)
	}
	if opts.OPMLSubcmd != "export" {
		t.Errorf("expected OPMLSubcmd 'export', got %q", opts.OPMLSubcmd)
	}
	if opts.OPMLFile != "file.opml" {
		t.Errorf("expected OPMLFile 'file.opml', got %q", opts.OPMLFile)
	}
}

func TestSyncOPMLExport_ExecutePodFetchModernDB(t *testing.T) {
	resetBackendState(t)
	dbPath := createTestModernPodfetchDB(t)

	targetFile := filepath.Join(t.TempDir(), "file.opml")
	cfg := Config{
		BackendType:    "podfetch",
		PodfetchDBPath: dbPath,
	}
	cli := CLIOptions{
		SyncSubcmd: "opml",
		OPMLSubcmd: "export",
		OPMLFile:   targetFile,
		Quiet:      true,
	}

	handleSyncCommand(cfg, cli)

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read generated OPML file %s: %v", targetFile, err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n") {
		t.Errorf("missing xml declaration in output: %s", content)
	}
	if !strings.Contains(content, `<opml version="2.0">`) {
		t.Errorf("missing opml version 2.0 root in output: %s", content)
	}
	if !strings.Contains(content, `https://feeds.example.com/alpha.xml`) {
		t.Errorf("missing alpha feed url in output: %s", content)
	}
	if !strings.Contains(content, `Podcast Beta &amp; Special &lt;Chars&gt;`) {
		t.Errorf("missing escaped title in output: %s", content)
	}
}

func TestSyncOPMLExport_ExecutePodFetchLegacyDB(t *testing.T) {
	resetBackendState(t)
	dbPath := createTestLegacyPodfetchDB(t)

	targetFile := filepath.Join(t.TempDir(), "file.opml")
	cfg := Config{
		BackendType:    "podfetch",
		PodfetchDBPath: dbPath,
	}
	cli := CLIOptions{
		SyncSubcmd: "opml",
		OPMLSubcmd: "export",
		OPMLFile:   targetFile,
		Quiet:      true,
	}

	handleSyncCommand(cfg, cli)

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read generated OPML file %s: %v", targetFile, err)
	}
	content := string(data)

	if !strings.Contains(content, `<opml version="2.0">`) {
		t.Errorf("missing opml version 2.0 in output: %s", content)
	}
	if !strings.Contains(content, `https://feeds.example.com/legacy.xml`) {
		t.Errorf("missing legacy feed url in output: %s", content)
	}
}

func TestSyncOPMLExport_ExecuteAudiobookshelfBackend(t *testing.T) {
	resetBackendState(t)
	server := setupMockABSServerForOPML(t)
	defer server.Close()

	targetFile := filepath.Join(t.TempDir(), "file.opml")
	cfg := Config{
		BackendType:         "audiobookshelf",
		AudiobookshelfURL:   server.URL,
		AudiobookshelfToken: "mock-token",
	}
	cli := CLIOptions{
		SyncSubcmd: "opml",
		OPMLSubcmd: "export",
		OPMLFile:   targetFile,
		Quiet:      true,
	}

	handleSyncCommand(cfg, cli)

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read generated OPML file %s: %v", targetFile, err)
	}
	content := string(data)

	if !strings.Contains(content, `<opml version="2.0">`) {
		t.Errorf("missing opml version 2.0 in output: %s", content)
	}
	if !strings.Contains(content, "ABS Feed Show") {
		t.Errorf("missing ABS podcast title in output: %s", content)
	}
}

func resolveAbsBinaryForTest(t *testing.T) string {
	t.Helper()
	localBin := filepath.Join(".", "abs")
	if fi, err := os.Stat(localBin); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
		absPath, err := filepath.Abs(localBin)
		if err == nil {
			return absPath
		}
		return localBin
	}

	tmpBin := filepath.Join(t.TempDir(), "abs_test_bin")
	cmd := exec.Command("go", "build", "-o", tmpBin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build abs binary for test: %v, output: %s", err, string(out))
	}
	return tmpBin
}

func TestSyncOPMLExport_SubprocessBinary(t *testing.T) {
	binPath := resolveAbsBinaryForTest(t)
	dbPath := createTestModernPodfetchDB(t)

	targetFile := filepath.Join(t.TempDir(), "file.opml")
	cmd := exec.Command(binPath, "sync", "opml", "export", targetFile)
	cmd.Env = append(os.Environ(),
		"BACKEND_TYPE=podfetch",
		"PODFETCH_DB_PATH="+dbPath,
		"PODFETCH_URL=",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("abs sync opml export failed: %v, output: %s", err, string(out))
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `<opml version="2.0">`) {
		t.Errorf("expected OPML 2.0 content from subprocess, got: %s", content)
	}
	if !strings.Contains(content, `https://feeds.example.com/alpha.xml`) {
		t.Errorf("expected feed URL in subprocess output, got: %s", content)
	}
}
