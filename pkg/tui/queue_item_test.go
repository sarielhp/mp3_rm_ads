package tui

import (
	"testing"

	"pod/pkg/backend"
)

// Regression: the Latest Episodes screen built its own queue item and omitted
// the enclosure URL and the GUID. An item queued from there reached the worker
// with no URL; the standalone backend skips an episode with no enclosure and
// returns no error, so the queue marked it completed and nothing downloaded.
func TestEnqueueFromLatestCarriesTheEnclosureURL(t *testing.T) {
	q := testLibrary().Queue()
	q.SetFilePath(t.TempDir() + "/download_queue.json")
	// Enqueueing triggers the download worker in a goroutine. Wait for it
	// before releasing the path, or it saves to the empty default and drops a
	// lock file in the source tree.
	defer func() {
		q.WaitWorkerForTest()
		q.SetFilePath("")
	}()
	q.Clear()

	m := &tuiModel{lib: testLibrary()}
	m.enqueueDownloadForLatestItem(tuiLatestItem{
		podcastName: "Show",
		podcastDir:  "/lib/Show",
		podcastID:   "p1",
		episode: tuiEpisode{
			title:        "Ep One",
			guid:         "guid-1",
			enclosureURL: "https://example.com/ep1.mp3",
			publishedAt:  1700000000000,
			duration:     123,
		},
	})

	items := q.Items()
	if len(items) != 1 {
		t.Fatalf("queued %d items, want 1", len(items))
	}
	got := items[0]
	if got.EnclosureURL != "https://example.com/ep1.mp3" {
		t.Errorf("EnclosureURL = %q; without it the worker silently completes the item", got.EnclosureURL)
	}
	if got.GUID != "guid-1" {
		t.Errorf("GUID = %q, want guid-1", got.GUID)
	}
	if got.PublishedAt != 1700000000000 || got.DurationSec != 123 {
		t.Errorf("unexpected item: %+v", got)
	}
}

// Every screen must produce the same entry for the same episode; the four
// inline copies drifting apart is what caused the bug above.
func TestDownloadQueueItemForIsConsistent(t *testing.T) {
	ep := tuiEpisode{
		title:        "Ep One",
		guid:         "guid-1",
		enclosureURL: "https://example.com/ep1.mp3",
		publishedAt:  1700000000000,
		duration:     42,
	}
	got := downloadQueueItemFor("Show", "/lib/Show", "p1", ep)

	if got.EpisodeTitle != "Ep One" || got.GUID != "guid-1" ||
		got.EnclosureURL != "https://example.com/ep1.mp3" ||
		got.PublishedAt != 1700000000000 || got.DurationSec != 42 ||
		got.PodcastTitle != "Show" || got.PodcastDir != "/lib/Show" || got.PodcastID != "p1" {
		t.Errorf("unexpected item: %+v", got)
	}
}

// Backend metadata wins over the locally derived values, but only where it has
// something to say: an absent backend id must not blank out the feed's GUID.
func TestDownloadQueueItemForPrefersBackendMetadata(t *testing.T) {
	ep := tuiEpisode{
		guid:         "feed-guid",
		enclosureURL: "https://example.com/ep1.mp3",
		publishedAt:  1000,
		absData:      &backend.Episode{ID: "abs-id", PubDate: "Mon, 01 Jan 2024 00:00:00 +0000"},
	}
	got := downloadQueueItemFor("Show", "/lib/Show", "p1", ep)
	if got.GUID != "abs-id" {
		t.Errorf("GUID = %q, want the backend id", got.GUID)
	}
	if got.PubDate == "" {
		t.Error("PubDate should come from the backend metadata")
	}

	ep.absData = &backend.Episode{}
	got = downloadQueueItemFor("Show", "/lib/Show", "p1", ep)
	if got.GUID != "feed-guid" {
		t.Errorf("an empty backend id must not blank the feed GUID; got %q", got.GUID)
	}
	if got.PublishedAt != 1000 {
		t.Errorf("PublishedAt = %d, want the episode's own 1000", got.PublishedAt)
	}
}
