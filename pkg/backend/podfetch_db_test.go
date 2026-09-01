package backend

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestPodFetchDB(t *testing.T) string {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "podcast.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite test db: %v", err)
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
			keywords TEXT,
			explicit INTEGER,
			created_at TEXT
		);
		CREATE TABLE podcast_episodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			podcast_id INTEGER,
			episode_id TEXT,
			name TEXT,
			url TEXT,
			date_of_recording TEXT,
			image_url TEXT,
			total_time INTEGER,
			local_url TEXT,
			local_image_url TEXT,
			description TEXT,
			status TEXT,
			download_time TEXT
		);
		INSERT INTO podcasts (id, name, directory, rssfeed, image_url, summary, author, created_at)
		VALUES (1, 'Show One', 'show_one', 'https://example.com/show1.xml', 'https://example.com/cover.jpg', 'Summary 1', 'Author 1', '2026-08-30 00:00:00');

		INSERT INTO podcast_episodes (id, podcast_id, episode_id, name, url, date_of_recording, total_time, local_url, description, status)
		VALUES 
			(10, 1, 'guid-10', 'Episode 10', 'https://example.com/10.mp3', '2026-08-28 10:00:00', 3600, 'ep10.mp3', 'Desc 10', 'D'),
			(11, 1, 'guid-11', 'Episode 11', 'https://example.com/11.mp3', '2026-08-29 10:00:00', 3600, 'ep11.mp3', 'Desc 11', 'D'),
			(12, 1, 'guid-12', 'Episode 12', 'https://example.com/12.mp3', '2026-08-30 10:00:00', 3600, 'ep12.mp3', 'Desc 12', 'P');
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to execute schema: %v", err)
	}

	return dbPath
}

func TestPodFetchDBDirectSync(t *testing.T) {
	dbPath := setupTestPodFetchDB(t)
	be := NewPodFetch(Config{DBPath: dbPath})

	ok, err := be.TestConnection(true)
	if !ok || err != nil {
		t.Fatalf("TestConnection on DB failed: %v", err)
	}

	podcasts, err := be.Podcasts()
	if err != nil || len(podcasts) != 1 {
		t.Fatalf("Podcasts from DB failed: %v, count: %d", err, len(podcasts))
	}
	if podcasts[0].Media.Metadata.Title != "Show One" {
		t.Errorf("expected 'Show One', got %s", podcasts[0].Media.Metadata.Title)
	}
	if len(podcasts[0].Media.Episodes) != 3 {
		t.Errorf("expected 3 episodes, got %d", len(podcasts[0].Media.Episodes))
	}

	pod, err := be.GetPodcast("1")
	if err != nil || pod == nil || pod.ID != "1" {
		t.Fatalf("GetPodcast from DB failed: %v", err)
	}

	created, err := be.CreatePodcast("", "", "show_two", "Show Two", "https://example.com/show2.xml")
	if err != nil || created == nil {
		t.Fatalf("CreatePodcast in DB failed: %v", err)
	}

	dls, err := be.ActiveDownloads("1")
	if err != nil || len(dls) != 1 || dls[0].Title != "Episode 12" {
		t.Fatalf("ActiveDownloads from DB failed: %v, dls: %+v", err, dls)
	}

	if err := be.DeletePodcastEpisode("1", "10"); err != nil {
		t.Fatalf("DeletePodcastEpisode from DB failed: %v", err)
	}

	podAfterDel, _ := be.GetPodcast("1")
	if len(podAfterDel.Media.Episodes) != 2 {
		t.Errorf("expected 2 episodes after deletion, got %d", len(podAfterDel.Media.Episodes))
	}
}

func TestPodFetchDurationSyncAndDateReset(t *testing.T) {
	dbPath := setupTestPodFetchDB(t)
	be := NewPodFetch(Config{DBPath: dbPath})

	if err := be.SyncDuration("ep11.mp3", 2450.5); err != nil {
		t.Fatalf("SyncDuration failed: %v", err)
	}

	db, _ := sql.Open("sqlite3", dbPath)
	defer db.Close()

	var totalTime int
	_ = db.QueryRow("SELECT total_time FROM podcast_episodes WHERE id = 11").Scan(&totalTime)
	if totalTime != 2450 {
		t.Errorf("expected duration 2450 in DB, got %d", totalTime)
	}

	if err := be.ResetPodcastDateCheck("1", "Show One"); err != nil {
		t.Fatalf("ResetPodcastDateCheck failed: %v", err)
	}

	var createdAt string
	_ = db.QueryRow("SELECT created_at FROM podcasts WHERE id = 1").Scan(&createdAt)
	if !strings.HasPrefix(createdAt, "1970-01-01") {
		t.Errorf("expected 1970 date check reset, got %s", createdAt)
	}
}

func TestPodFetchOPMLAndKeepPolicy(t *testing.T) {
	dbPath := setupTestPodFetchDB(t)
	be := NewPodFetch(Config{DBPath: dbPath})

	opmlBytes, err := be.ExportOPML(OPMLExportOptions{})
	if err != nil || !strings.Contains(string(opmlBytes), "https://example.com/show1.xml") {
		t.Fatalf("ExportOPML failed: %v, output: %s", err, string(opmlBytes))
	}

	importXML := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Feeds</title></head>
  <body>
    <outline text="New Import" type="rss" xmlUrl="https://example.com/import.xml"/>
  </body>
</opml>`
	res, err := be.ImportOPML([]byte(importXML), OPMLImportOptions{})
	if err != nil || res.Subscribed != 1 {
		t.Fatalf("ImportOPML failed: %v, res: %+v", err, res)
	}

	deleted, err := be.ApplyKeepPolicy("1", "Show One", 1, false, false, true)
	if err != nil {
		t.Fatalf("ApplyKeepPolicy failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted episodes under keep=1 policy, got %d", deleted)
	}
}

func TestPodFetchScanAndRescan(t *testing.T) {
	dbPath := setupTestPodFetchDB(t)
	tempDir := t.TempDir()
	podcastsDir := filepath.Join(tempDir, "podcasts")
	_ = os.MkdirAll(podcastsDir, 0755)

	be := NewPodFetch(Config{
		DBPath:      dbPath,
		PodcastsDir: podcastsDir,
	})

	scanRes, err := be.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if scanRes.NewPodcasts != 1 {
		t.Errorf("expected 1 new podcast scanned, got %d", scanRes.NewPodcasts)
	}
	if _, err := os.Stat(filepath.Join(podcastsDir, "Show One")); os.IsNotExist(err) {
		t.Errorf("expected Show One directory created in podcastsDir")
	}

	rescanRes, err := be.Rescan(RescanOptions{PodcastsDir: podcastsDir})
	if err != nil {
		t.Fatalf("Rescan failed: %v", err)
	}
	if rescanRes.CheckedCount == 0 {
		t.Errorf("expected CheckedCount > 0")
	}
}

func TestParseRSSFeedXML(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>RSS Test Podcast</title>
    <item>
      <title>Episode 1</title>
      <description>Description of Episode 1</description>
      <pubDate>Wed, 15 Jul 2026 19:00:00 GMT</pubDate>
      <guid>guid-ep-1</guid>
      <itunes:duration>01:15:30</itunes:duration>
      <itunes:season>2</itunes:season>
      <itunes:episode>10</itunes:episode>
      <enclosure url="https://example.com/audio.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`

	eps, err := parseRSSFeedXML([]byte(xmlData))
	if err != nil || len(eps) != 1 {
		t.Fatalf("parseRSSFeedXML failed: %v, eps: %+v", err, eps)
	}
	if eps[0].Title != "Episode 1" {
		t.Errorf("expected title 'Episode 1', got %s", eps[0].Title)
	}
	if eps[0].DurationSeconds != 4530 {
		t.Errorf("expected duration 4530s, got %f", eps[0].DurationSeconds)
	}
	if eps[0].Enclosure == nil || eps[0].Enclosure.URL != "https://example.com/audio.mp3" {
		t.Errorf("unexpected enclosure: %+v", eps[0].Enclosure)
	}
}
