package podcast

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Moved here from pkg/cli with the planner it covers.
func TestPlanSubscriptionDownloadsInParallel(t *testing.T) {
	t.Parallel()
	feed := func(title, ep, url string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>` + title + `</title>
<item><title>` + ep + `</title><enclosure url="` + url + `" type="audio/mpeg"/></item>
</channel></rss>`))
		}))
	}
	ts1 := feed("Show 1", "Ep 1", "http://example.com/1.mp3")
	defer ts1.Close()
	ts2 := feed("Show 2", "Ep 2", "http://example.com/2.mp3")
	defer ts2.Close()

	root := t.TempDir()
	show1 := filepath.Join(root, "Show 1")
	if err := os.MkdirAll(show1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show1, "Ep 1.mp3"), []byte("mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	targets := []Subscription{
		{ID: "p1", Title: "Show 1", Folder: "Show 1", FeedURL: ts1.URL, DownloadPolicy: "latest"},
		{ID: "p2", Title: "Show 2", Folder: "Show 2", FeedURL: ts2.URL, DownloadPolicy: "latest"},
	}

	lib := Open(Config{PodcastsDir: root}, nil, nil)
	var seen int
	plans := lib.PlanSubscriptionDownloads(targets, SubscriptionDownloadOptions{}, func(done, total int) {
		seen = done
	})

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if seen != 2 {
		t.Errorf("progress callback reported %d of 2 completions", seen)
	}
	if len(plans[0].ToDownload) != 0 {
		t.Errorf("Show 1 is already on disk; got %d to download", len(plans[0].ToDownload))
	}
	if len(plans[1].ToDownload) != 1 {
		t.Errorf("Show 2 should have 1 to download, got %d", len(plans[1].ToDownload))
	}
}

// A plan with nothing to download still republishes the podcast, so an edited
// title or a newly downloaded cover reaches feed.xml without waiting for the
// next episode.
func TestExecuteSubscriptionDownloadsPublishesWithNothingToDownload(t *testing.T) {
	t.Parallel()
	podDir := t.TempDir()
	lib := Open(Config{PodcastsDir: t.TempDir()}, nil, nil)

	res := lib.ExecuteSubscriptionDownloads([]SubscriptionPlan{{
		Sub:    Subscription{ID: "p1", Title: "P1", Folder: "P1"},
		PodDir: podDir,
	}}, nil, SubscriptionDownloadOptions{})

	if res.Downloaded != 0 || len(res.Failures) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(podDir, "feed.xml")); err != nil {
		t.Fatalf("expected feed.xml at %s: %v", podDir, err)
	}
}

func TestExecuteSubscriptionDownloadsUpdatesCatalogAndFeed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	podcastsDir := filepath.Join(root, "podcasts")
	showDir := filepath.Join(podcastsDir, "Show")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(showDir, "Ep1.mp3"), []byte("fake mp3 data"), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := NewSubscriptionStore(filepath.Join(root, "podcasts.json"))
	if err != nil {
		t.Fatal(err)
	}
	sub := Subscription{ID: "sub1", Title: "Show", Folder: "Show", FeedURL: "https://example.com/feed.xml"}
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	lib := Open(Config{
		PodcastsDir:   podcastsDir,
		ServerBaseURL: "http://example.com/podcasts",
	}, nil, nil)

	lib.ExecuteSubscriptionDownloads(
		[]SubscriptionPlan{{Sub: sub, PodDir: showDir}}, store, SubscriptionDownloadOptions{})

	data, err := os.ReadFile(filepath.Join(showDir, "feed.xml"))
	if err != nil {
		t.Fatalf("expected feed.xml: %v", err)
	}
	if !strings.Contains(string(data), "http://example.com/podcasts/Show/Ep1.mp3") {
		t.Fatalf("feed.xml missing the enclosure URL:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(podcastsDir, "index.html")); err != nil {
		t.Fatalf("expected catalog index.html: %v", err)
	}
}
