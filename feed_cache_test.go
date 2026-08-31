package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFeedDate(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"Mon, 02 Jan 2006 15:04:05 -0700", 1136239445000},
		{"Mon, 02 Jan 2006 15:04:05 MST", 1136214245000},
		{"2006-01-02T15:04:05Z", 1136214245000},
		{"", 0},
	}

	for _, tt := range tests {
		ms, _ := parseFeedDate(tt.input)
		if tt.input == "" && ms != 0 {
			t.Errorf("expected 0 for empty date, got %d", ms)
		}
		if tt.input != "" && ms == 0 {
			t.Errorf("failed to parse valid date: %s", tt.input)
		}
	}
}

func TestParseRSSXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Sample Podcast</title>
    <item>
      <title>Episode 1: Pilot</title>
      <description>First episode description</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
      <guid isPermaLink="false">guid-ep-1</guid>
      <enclosure url="https://example.com/ep1.mp3" length="1234567" type="audio/mpeg"/>
    </item>
    <item>
      <title>Episode 2: Sequel</title>
      <description>Second episode description</description>
      <pubDate>Tue, 03 Jan 2006 15:04:05 -0700</pubDate>
      <guid>guid-ep-2</guid>
      <enclosure url="https://example.com/ep2.mp3" length="2345678" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`)

	episodes, err := parseRSSXML(xmlData)
	if err != nil {
		t.Fatalf("parseRSSXML failed: %v", err)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}

	if episodes[0].Title != "Episode 1: Pilot" || episodes[0].GUID != "guid-ep-1" {
		t.Errorf("episode 0 mismatch: %+v", episodes[0])
	}
	if episodes[0].Enclosure == nil || episodes[0].Enclosure.URL != "https://example.com/ep1.mp3" {
		t.Errorf("episode 0 enclosure mismatch: %+v", episodes[0].Enclosure)
	}
	if episodes[1].GUID != "guid-ep-2" {
		t.Errorf("episode 1 GUID mismatch: %+v", episodes[1])
	}
}

func TestFeedCacheManager(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "feed_cache.json")

	mgr := &FeedCacheManager{
		cacheFile: cachePath,
		entries:   make(map[string]*FeedCacheEntry),
	}

	feedURL := "https://example.com/feed.xml"
	entry := &FeedCacheEntry{
		FeedURL:      feedURL,
		ETag:         "\"etag-123\"",
		LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
		LastChecked:  time.Now(),
		LatestGUID:   "ep-1",
		Episodes: []FeedEpisode{
			{Title: "Ep 1", GUID: "ep-1"},
		},
	}

	mgr.Put(feedURL, entry)
	if err := mgr.Save(); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	// Reload in new manager
	mgr2 := &FeedCacheManager{
		cacheFile: cachePath,
		entries:   make(map[string]*FeedCacheEntry),
	}
	mgr2.load()

	got := mgr2.Get(feedURL)
	if got == nil {
		t.Fatal("expected cached entry after load")
	}
	if got.ETag != "\"etag-123\"" || len(got.Episodes) != 1 {
		t.Errorf("unexpected cached entry: %+v", got)
	}
}

func TestFetchFeedDirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == "\"etag-abc\"" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", "\"etag-abc\"")
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `<rss version="2.0"><channel><title>T</title><item><title>E1</title><guid>g1</guid><enclosure url="https://example.com/e1.mp3" type="audio/mpeg"/></item></channel></rss>`)
	}))
	defer ts.Close()

	// 1. Cold fetch (200 OK)
	eps, etag, lastMod, notMod, err := fetchFeedDirect(ts.URL, "", "")
	if err != nil {
		t.Fatalf("cold fetch failed: %v", err)
	}
	if notMod || len(eps) != 1 || etag != "\"etag-abc\"" || lastMod == "" {
		t.Errorf("unexpected cold fetch: eps=%d, etag=%s, notMod=%v", len(eps), etag, notMod)
	}

	// 2. Conditional fetch (304 Not Modified)
	_, _, _, notMod2, err2 := fetchFeedDirect(ts.URL, "\"etag-abc\"", lastMod)
	if err2 != nil {
		t.Fatalf("conditional fetch failed: %v", err2)
	}
	if !notMod2 {
		t.Error("expected 304 notModified")
	}
}

func TestFeedCacheExpiration(t *testing.T) {
	var nilEntry *FeedCacheEntry
	if !nilEntry.IsExpired(48 * time.Hour) {
		t.Error("expected nil entry to be expired")
	}

	zeroEntry := &FeedCacheEntry{}
	if !zeroEntry.IsExpired(48 * time.Hour) {
		t.Error("expected zero entry to be expired")
	}

	freshEntry := &FeedCacheEntry{
		LastChecked: time.Now().Add(-10 * time.Hour),
	}
	if freshEntry.IsExpired(48 * time.Hour) {
		t.Error("expected 10h old entry to not be expired with 48h TTL")
	}

	staleEntry := &FeedCacheEntry{
		LastChecked: time.Now().Add(-49 * time.Hour),
	}
	if !staleEntry.IsExpired(48 * time.Hour) {
		t.Error("expected 49h old entry to be expired with 48h TTL")
	}
}
