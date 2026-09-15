package podcast

import (
	"strings"
	"testing"
	"time"

	"pod/pkg/backend"
	"pod/pkg/progress"
)

type recordingBackend struct {
	backend.Backend
	downloaded []backend.FeedEpisode
}

func (b *recordingBackend) DownloadEpisodes(podcastID string, eps []backend.FeedEpisode) error {
	b.downloaded = append(b.downloaded, eps...)
	return nil
}

func (b *recordingBackend) WaitForActiveDownloads([]backend.Podcast, time.Duration) error {
	return nil
}

func downloadFixture() (backend.Podcast, []backend.FeedEpisode) {
	item := backend.Podcast{ID: "p1"}
	item.Media.Metadata.Title = "Test Show"
	eps := []backend.FeedEpisode{
		{Title: "Ep A", PubDate: "Mon, 01 Jan 2024 00:00:00 +0000", PublishedAt: 1704067200000,
			Enclosure: &backend.FeedEnclosure{URL: "https://example.com/a.mp3"}},
	}
	return item, eps
}

func TestExecuteEpisodeDownloadsReportsToReporter(t *testing.T) {
	t.Parallel()
	item, eps := downloadFixture()
	lines := &progress.Lines{}

	n, err := ExecuteEpisodeDownloads(&recordingBackend{}, item, eps,
		[]string{"1 new episode(s)"}, DownloadOptions{NoWait: true, Progress: lines})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("downloaded %d, want 1", n)
	}

	joined := strings.Join(lines.Info, "\n")
	for _, want := range []string{"Test Show", "1 new episode(s)", "Ep A", "Download request successfully sent!"} {
		if !strings.Contains(joined, want) {
			t.Errorf("reporter missing %q; got:\n%s", want, joined)
		}
	}
	// The enclosure URL is detail, shown only when the caller asks for it.
	if strings.Contains(joined, "example.com/a.mp3") {
		t.Errorf("URL leaked into Info; it belongs in Detail:\n%s", joined)
	}
	if !strings.Contains(strings.Join(lines.Detail, "\n"), "example.com/a.mp3") {
		t.Errorf("URL missing from Detail: %v", lines.Detail)
	}
}

// A nil Progress is the default for every non-interactive caller, including
// the TUI. It must be silent rather than panicking or falling back to stdout.
func TestExecuteEpisodeDownloadsSilentWithoutReporter(t *testing.T) {
	t.Parallel()
	item, eps := downloadFixture()
	b := &recordingBackend{}

	n, err := ExecuteEpisodeDownloads(b, item, eps, []string{"1 new"}, DownloadOptions{NoWait: true})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(b.downloaded) != 1 {
		t.Fatalf("download did not happen: n=%d recorded=%d", n, len(b.downloaded))
	}
}

func TestExecuteEpisodeDownloadsDryRunSkipsBackend(t *testing.T) {
	t.Parallel()
	item, eps := downloadFixture()
	b := &recordingBackend{}
	lines := &progress.Lines{}

	if _, err := ExecuteEpisodeDownloads(b, item, eps, nil,
		DownloadOptions{DryRun: true, NoWait: true, Progress: lines}); err != nil {
		t.Fatal(err)
	}
	if len(b.downloaded) != 0 {
		t.Errorf("dry run reached the backend: %v", b.downloaded)
	}
	if !strings.Contains(strings.Join(lines.Info, "\n"), "Dry run mode enabled") {
		t.Errorf("dry run not reported: %v", lines.Info)
	}
}
