package podcast

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"abs/pkg/backend"
	"abs/pkg/config"
)

func TestFeedCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "feed_cache.json")
	mgr := NewFeedCacheManager(cachePath)

	feedURL := "https://example.com/feed.xml"
	entry := &FeedCacheEntry{
		FeedURL:     feedURL,
		ETag:        "\"12345\"",
		LastChecked: time.Now(),
		Episodes: []backend.FeedEpisode{
			{Title: "Ep 1", GUID: "guid-1"},
		},
	}

	mgr.Put(feedURL, entry)
	if err := mgr.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got := mgr.Get(feedURL)
	if got == nil || got.ETag != "\"12345\"" || len(got.Episodes) != 1 {
		t.Fatalf("Unexpected cached entry: %+v", got)
	}

	mgr2 := NewFeedCacheManager(cachePath)
	got2 := mgr2.Get(feedURL)
	if got2 == nil || got2.ETag != "\"12345\"" {
		t.Fatalf("Reloaded cache missing entry")
	}

	if entry.IsExpired(24 * time.Hour) {
		t.Errorf("expected entry not to be expired")
	}
	oldEntry := &FeedCacheEntry{LastChecked: time.Now().Add(-48 * time.Hour)}
	if !oldEntry.IsExpired(24 * time.Hour) {
		t.Errorf("expected old entry to be expired")
	}
}

func TestParseRSSXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Sample Podcast</title>
    <item>
      <title>Episode 1</title>
      <description>First episode description</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
      <guid>ep1-guid</guid>
      <enclosure url="http://example.com/ep1.mp3" type="audio/mpeg"/>
    </item>
    <item>
      <title>Episode 2 (No Enclosure)</title>
      <guid>ep2-guid</guid>
    </item>
  </channel>
</rss>`)

	eps, err := ParseRSSXML(xmlData)
	if err != nil {
		t.Fatalf("ParseRSSXML failed: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 episode with enclosure, got %d", len(eps))
	}
	if eps[0].Title != "Episode 1" || eps[0].GUID != "ep1-guid" {
		t.Errorf("unexpected episode data: %+v", eps[0])
	}
	if eps[0].EnclosureURL != "https://example.com/ep1.mp3" {
		t.Errorf("expected https upgrade for enclosure URL, got %s", eps[0].EnclosureURL)
	}
}

func TestPodcastShortID(t *testing.T) {
	tests := []struct {
		title    string
		want5Len bool
	}{
		{"The Daily", true},
		{"Planet Money", true},
		{"Radiolab", true},
		{"99% Invisible", true},
		{"", true},
	}

	for _, tt := range tests {
		id := GeneratePodcastShortID(tt.title)
		if len(id) != 5 {
			t.Errorf("GeneratePodcastShortID(%q) = %q (len %d, want 5)", tt.title, id, len(id))
		}
	}
}

func TestEpisodeShortID(t *testing.T) {
	podShort := "plntm"
	epKey := "123_Inflation.mp3"
	id := GenerateEpisodeShortID(podShort, epKey)
	if len(id) != 6 || id[0] != 'e' {
		t.Fatalf("unexpected episode short ID: %s", id)
	}

	title := EpisodeTitleFromPath("/tmp/podcast/Episode 42.mp3")
	if title != "Episode 42" {
		t.Errorf("expected 'Episode 42', got %s", title)
	}
}

func TestDownloadQueue(t *testing.T) {
	tmpDir := t.TempDir()
	qPath := filepath.Join(tmpDir, "dl_queue.json")
	q := NewDownloadQueue(qPath)

	item := DownloadQueueItem{
		PodcastTitle: "My Podcast",
		EpisodeTitle: "Ep 1",
		GUID:         "guid-101",
		EnclosureURL: "https://example.com/101.mp3",
	}

	ok, status := q.Enqueue(item)
	if !ok || status != "queued" {
		t.Fatalf("Enqueue failed: ok=%v, status=%s", ok, status)
	}

	if !q.IsEpisodeInQueue("guid-101", "", "Ep 1") {
		t.Errorf("expected episode to be in queue")
	}

	items := q.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	claimed, ok, err := q.Claim()
	if err != nil || !ok {
		t.Fatalf("Claim failed: %v", err)
	}
	if claimed.Status != "downloading" {
		t.Errorf("expected claimed status to be downloading, got %s", claimed.Status)
	}

	if err := q.Finalize(claimed.ID, nil); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
	items = q.Items()
	if items[0].Status != "completed" {
		t.Errorf("expected status completed, got %s", items[0].Status)
	}

	if !q.Remove(claimed.ID) {
		t.Errorf("expected item to be removed")
	}
	if len(q.Items()) != 0 {
		t.Errorf("expected queue to be empty")
	}
}

func TestDownloadPolicy(t *testing.T) {
	catalog := []backend.FeedEpisode{
		{Title: "Ep 1", EnclosureURL: "https://example.com/1.mp3"},
		{Title: "Ep 2", EnclosureURL: "https://example.com/2.mp3"},
		{Title: "Ep 3", EnclosureURL: "https://example.com/3.mp3"},
	}

	isDl := func(ep backend.FeedEpisode) bool {
		return ep.Title == "Ep 1"
	}

	eps, _ := SelectEpisodesByDownloadPolicy(catalog, isDl, config.DownloadPolicyNone, 0, false)
	if len(eps) != 0 {
		t.Errorf("expected 0 episodes for none policy, got %d", len(eps))
	}

	eps, _ = SelectEpisodesByDownloadPolicy(catalog, isDl, config.DownloadPolicyLatest, 0, false)
	if len(eps) != 1 || eps[0].Title != "Ep 3" {
		t.Errorf("expected Ep 3 for latest policy, got %+v", eps)
	}

	eps, _ = SelectEpisodesByDownloadPolicy(catalog, isDl, config.DownloadPolicyAll, 0, false)
	if len(eps) != 2 {
		t.Errorf("expected 2 undownloaded episodes for all policy, got %d", len(eps))
	}
}

func TestOrphanPodcasts(t *testing.T) {
	podcasts := []backend.Podcast{
		{
			ID: "p1",
			Media: backend.PodcastMedia{
				Metadata: backend.PodcastMetadata{Title: "Orphan No Feed", FeedURL: ""},
			},
		},
		{
			ID: "p2",
			Media: backend.PodcastMedia{
				Metadata: backend.PodcastMetadata{Title: "Duplicate Feed A", FeedURL: "https://example.com/rss/"},
			},
		},
		{
			ID: "p3",
			Media: backend.PodcastMedia{
				Metadata: backend.PodcastMetadata{Title: "Duplicate Feed B", FeedURL: "https://example.com/rss"},
			},
		},
	}

	orphans := FindOrphanPodcasts(podcasts)
	if len(orphans) != 2 {
		t.Fatalf("expected 2 orphans (1 empty feed + 1 duplicate), got %d", len(orphans))
	}
}

func TestPodcastCache(t *testing.T) {
	tmpDir := t.TempDir()
	podDir := filepath.Join(tmpDir, "TestPodcast")
	_ = os.MkdirAll(podDir, 0755)

	index := &CachedPodcastIndex{
		PodcastName: "Test Podcast",
		PodcastDir:  podDir,
		Episodes: []CachedEpisodeSummary{
			{Title: "Ep 1", Filename: "ep1.mp3"},
		},
	}

	if err := SavePodcastCache(podDir, index); err != nil {
		t.Fatalf("SavePodcastCache failed: %v", err)
	}

	loaded, err := LoadPodcastCache(podDir)
	if err != nil {
		t.Fatalf("LoadPodcastCache failed: %v", err)
	}
	if loaded.PodcastName != "Test Podcast" || len(loaded.Episodes) != 1 {
		t.Errorf("unexpected loaded cache: %+v", loaded)
	}
}
