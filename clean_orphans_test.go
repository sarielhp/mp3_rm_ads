package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sariel/abs/pkg/backend"
)

type mockOrphanBackend struct {
	podcasts     []PodcastItem
	deletedIDs   []string
	fetchErr     error
	deleteErrors map[string]error
}

func (m *mockOrphanBackend) Name() string                               { return "mock" }
func (m *mockOrphanBackend) TestConnection(quiet bool) (bool, error)    { return true, nil }
func (m *mockOrphanBackend) Login() (string, error)                     { return "token", nil }
func (m *mockOrphanBackend) Libraries() ([]Library, error)              { return nil, nil }
func (m *mockOrphanBackend) PodcastLibraries() ([]Library, error)       { return nil, nil }
func (m *mockOrphanBackend) Podcasts() ([]PodcastItem, error)           { return m.podcasts, m.fetchErr }
func (m *mockOrphanBackend) GetPodcast(id string) (*PodcastItem, error) { return nil, nil }
func (m *mockOrphanBackend) CreatePodcast(l, f, p, t, u string) (*PodcastItem, error) {
	return nil, nil
}
func (m *mockOrphanBackend) DeletePodcast(id string) error {
	if m.deleteErrors != nil && m.deleteErrors[id] != nil {
		return m.deleteErrors[id]
	}
	m.deletedIDs = append(m.deletedIDs, id)
	return nil
}
func (m *mockOrphanBackend) DeleteItem(id string) error { return m.DeletePodcast(id) }
func (m *mockOrphanBackend) PodcastFeedEpisodes(feedURL string) ([]FeedEpisode, error) {
	return nil, nil
}
func (m *mockOrphanBackend) DownloadEpisodes(pID string, eps []FeedEpisode) error { return nil }
func (m *mockOrphanBackend) DeletePodcastEpisode(pID, epID string) error          { return nil }
func (m *mockOrphanBackend) ActiveDownloads(pID string) ([]ActiveDownload, error) { return nil, nil }
func (m *mockOrphanBackend) OpenRSSFeed(pID, baseURL string) (string, error)      { return "", nil }
func (m *mockOrphanBackend) DownloadCover(pID, destPath string) error             { return nil }
func (m *mockOrphanBackend) ResetPodcastDateCheck(itemID, title string) error     { return nil }
func (m *mockOrphanBackend) ResetPodcastDateCheckAPI(itemID string) error         { return nil }
func (m *mockOrphanBackend) Scan(opts ScanOptions) (ScanResult, error)            { return ScanResult{}, nil }
func (m *mockOrphanBackend) Rescan(opts RescanOptions) (RescanResult, error) {
	return RescanResult{}, nil
}
func (m *mockOrphanBackend) ExportOPML(opts OPMLExportOptions) ([]byte, error) { return nil, nil }
func (m *mockOrphanBackend) ImportOPML(data []byte, opts OPMLImportOptions) (OPMLImportResult, error) {
	return OPMLImportResult{}, nil
}
func (m *mockOrphanBackend) FetchPodcastFeeds(s, v bool) ([]backend.OPMLFeed, error) { return nil, nil }
func (m *mockOrphanBackend) SyncDuration(filePath string, duration float64) error    { return nil }
func (m *mockOrphanBackend) ApplyKeepPolicy(pID, t string, k int, dr, v, q bool) (int, error) {
	return 0, nil
}
func (m *mockOrphanBackend) WaitForActiveDownloads(p []PodcastItem, q bool, to time.Duration) error {
	return nil
}

func makeTestPod(id, title, feedURL string, epCount int) PodcastItem {
	eps := make([]Episode, epCount)
	for i := 0; i < epCount; i++ {
		eps[i] = Episode{ID: fmt.Sprintf("%s-ep-%d", id, i+1)}
	}
	return PodcastItem{
		ID: id,
		Media: PodcastMedia{
			Metadata: PodcastMetadata{Title: title, FeedURL: feedURL},
			Episodes: eps,
		},
	}
}

func TestFindOrphanPodcasts(t *testing.T) {
	items := []PodcastItem{
		makeTestPod("pod-1", "Valid Podcast", "https://example.com/feed.xml", 2),
		makeTestPod("pod-2", "Fake Empty URL", "", 0),
		makeTestPod("pod-3", "Fake Whitespace URL", "   ", 0),
		makeTestPod("pod-4", "Duplicate Podcast (Empty)", "https://example.com/feed.xml", 0),
		makeTestPod("pod-5", "Duplicate Podcast Normalized", "HTTPS://EXAMPLE.COM/feed.xml/", 0),
	}

	orphans := FindOrphanPodcasts(items)
	if len(orphans) != 4 {
		t.Fatalf("expected 4 orphans, got %d", len(orphans))
	}

	orphanMap := make(map[string]OrphanPodcast)
	for _, o := range orphans {
		orphanMap[o.Item.ID] = o
	}

	if o, ok := orphanMap["pod-2"]; !ok || o.Reason != "Missing or empty RSS feed URL" {
		t.Errorf("pod-2 not detected as missing feed url: %+v", o)
	}
	if o, ok := orphanMap["pod-3"]; !ok || o.Reason != "Missing or empty RSS feed URL" {
		t.Errorf("pod-3 not detected as missing feed url: %+v", o)
	}
	if o, ok := orphanMap["pod-4"]; !ok || !strings.Contains(o.Reason, "Duplicate feed URL") || o.DuplicateOfID != "pod-1" {
		t.Errorf("pod-4 not detected as duplicate of pod-1: %+v", o)
	}
	if o, ok := orphanMap["pod-5"]; !ok || !strings.Contains(o.Reason, "Duplicate feed URL") || o.DuplicateOfID != "pod-1" {
		t.Errorf("pod-5 not detected as duplicate of pod-1: %+v", o)
	}
}

func TestRunCleanOrphansDryRun(t *testing.T) {
	mb := &mockOrphanBackend{
		podcasts: []PodcastItem{
			makeTestPod("pod-1", "Good Podcast", "https://example.com/rss", 1),
			makeTestPod("pod-fake", "Fake", "", 0),
		},
	}

	var buf bytes.Buffer
	res, err := RunCleanOrphans(mb, CleanOrphansOptions{DryRun: true, Out: &buf})
	if err != nil {
		t.Fatalf("RunCleanOrphans returned error: %v", err)
	}
	if res.ScannedCount != 2 || res.OrphanCount != 1 || res.DeletedCount != 0 {
		t.Errorf("unexpected result: %+v", res)
	}
	if len(mb.deletedIDs) != 0 {
		t.Errorf("expected no deletions in dry-run mode, got %v", mb.deletedIDs)
	}
	if !strings.Contains(buf.String(), "Dry run enabled") {
		t.Errorf("expected dry run message in output, got:\n%s", buf.String())
	}
}

func TestRunCleanOrphansForce(t *testing.T) {
	mb := &mockOrphanBackend{
		podcasts: []PodcastItem{
			makeTestPod("pod-1", "Good Podcast", "https://example.com/rss", 1),
			makeTestPod("pod-fake", "Fake", "", 0),
		},
	}

	var buf bytes.Buffer
	res, err := RunCleanOrphans(mb, CleanOrphansOptions{Force: true, Out: &buf})
	if err != nil {
		t.Fatalf("RunCleanOrphans returned error: %v", err)
	}
	if res.DeletedCount != 1 || res.FailedCount != 0 {
		t.Errorf("expected 1 deletion, got %+v", res)
	}
	if len(mb.deletedIDs) != 1 || mb.deletedIDs[0] != "pod-fake" {
		t.Errorf("unexpected deleted IDs: %v", mb.deletedIDs)
	}
	if !strings.Contains(buf.String(), "Successfully deleted 1 orphaned podcast(s)") {
		t.Errorf("expected success message in output, got:\n%s", buf.String())
	}
}

func TestRunCleanOrphansInteractive(t *testing.T) {
	mb := &mockOrphanBackend{
		podcasts: []PodcastItem{makeTestPod("pod-fake", "Fake", "", 0)},
	}

	// Case 1: User says 'y'
	var bufY bytes.Buffer
	resY, err := RunCleanOrphans(mb, CleanOrphansOptions{In: strings.NewReader("y\n"), Out: &bufY})
	if err != nil || resY.DeletedCount != 1 {
		t.Errorf("expected deletion on 'y', got res=%+v err=%v", resY, err)
	}

	// Case 2: User says 'n'
	mb.deletedIDs = nil
	var bufN bytes.Buffer
	resN, err := RunCleanOrphans(mb, CleanOrphansOptions{In: strings.NewReader("n\n"), Out: &bufN})
	if err != nil || resN.DeletedCount != 0 {
		t.Errorf("expected no deletion on 'n', got res=%+v err=%v", resN, err)
	}
	if !strings.Contains(bufN.String(), "Aborted") {
		t.Errorf("expected Aborted message on 'n', got:\n%s", bufN.String())
	}
}

func TestRunCleanOrphansErrorsAndQuiet(t *testing.T) {
	mb := &mockOrphanBackend{
		podcasts: []PodcastItem{makeTestPod("pod-err", "Failing", "", 0)},
		deleteErrors: map[string]error{
			"pod-err": fmt.Errorf("backend deletion failed"),
		},
	}

	var buf bytes.Buffer
	res, err := RunCleanOrphans(mb, CleanOrphansOptions{Force: true, Quiet: true, Verbose: true, Out: &buf})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FailedCount != 1 || res.DeletedCount != 0 {
		t.Errorf("expected 1 failed deletion, got %+v", res)
	}
	if buf.Len() != 0 {
		t.Errorf("expected quiet mode to produce no output, got:\n%s", buf.String())
	}
}

func TestRunCleanOrphansNoOrphans(t *testing.T) {
	mb := &mockOrphanBackend{
		podcasts: []PodcastItem{makeTestPod("pod-1", "Good Podcast", "https://example.com/rss", 1)},
	}

	var buf bytes.Buffer
	res, err := RunCleanOrphans(mb, CleanOrphansOptions{Out: &buf})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrphanCount != 0 || res.DeletedCount != 0 {
		t.Errorf("expected 0 orphans, got %+v", res)
	}
	if !strings.Contains(buf.String(), "No orphaned or fake podcast entries found") {
		t.Errorf("expected clean message, got:\n%s", buf.String())
	}
}
