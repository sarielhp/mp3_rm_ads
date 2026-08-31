package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestTUIFeedFetchAndDownloadAll(t *testing.T) {
	tempDir := t.TempDir()

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Podcast</title>
    <item>
      <title>Episode 1</title>
      <guid>guid-1</guid>
      <pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
      <enclosure url="http://example.com/ep1.mp3" type="audio/mpeg" length="12345"/>
    </item>
    <item>
      <title>Episode 2</title>
      <guid>guid-2</guid>
      <pubDate>Tue, 03 Jan 2006 15:04:05 GMT</pubDate>
      <enclosure url="http://example.com/ep2.mp3" type="audio/mpeg" length="23456"/>
    </item>
  </channel>
</rss>`
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer feedServer.Close()

	pod := tuiPodcast{
		name:    "Test Podcast",
		dir:     tempDir,
		feedURL: feedServer.URL,
	}

	model := &tuiModel{
		podcasts: []tuiPodcast{pod},
		podIdx:   0,
	}

	model.fetchPodcastFullFeed()

	if len(model.podcasts[0].episodes) != 2 {
		t.Fatalf("expected 2 episodes populated from feed, got %d", len(model.podcasts[0].episodes))
	}
	if !model.podcasts[0].episodes[0].isFeedOnly {
		t.Errorf("expected episode to have isFeedOnly true")
	}

	testDownloadQueuePath = filepath.Join(tempDir, "download_queue.json")
	defer func() { testDownloadQueuePath = "" }()

	model.downloadAllForSelectedPodcast()
	items := GetDownloadQueueItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 items in download queue, got %d", len(items))
	}
}
