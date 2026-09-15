package podcast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCacheWireFormat pins the on-disk shape of the podcast cache. The cache
// is written by one version of pod and read by the next, so a renamed field or
// a reordered embed silently orphans every cache already on disk. Regenerate
// deliberately with: go test ./pkg/podcast -update-cache
func TestCacheWireFormat(t *testing.T) {
	idx := CachedPodcastIndex{
		PodcastName: "Show & Co",
		PodcastDir:  "/lib/Show",
		ABSItemID:   "abs-1",
		Author:      "An Author",
		Description: "Desc",
		FeedURL:     "https://e.com/f.xml",
		CoverPath:   "/lib/Show/cover.jpg",
		UpdatedAt:   time.Unix(1700000000, 0).UTC(),
		Episodes: []CachedEpisodeSummary{
			{
				EpisodeFile: EpisodeFile{
					Path: "/lib/Show/a.mp3", Filename: "a.mp3", Title: "Ep A",
					PublishedAt: 1600000000000, DurationSec: 1234.5, SizeBytes: 999,
				},
				Season: "1", Episode: "2", HasAdsRemoved: true, HasTranscript: true,
			},
			{EpisodeFile: EpisodeFile{Path: "/lib/Show/b.mp3", Filename: "b.mp3"}},
		},
	}
	assertGoldenJSON(t, "cache_index.json", idx)

	det := CachedEpisodeDetails{
		Path: "/lib/Show/a.mp3", Filename: "a.mp3", Title: "Ep A",
		Description: "D", Subtitle: "S", EpisodeType: "full",
		Genres: []string{"news"}, Author: "A", FeedURL: "https://e.com/f.xml",
	}
	assertGoldenJSON(t, "cache_details.json", det)
}

// A cache written by an older pod carries an episode short ID in "id". Nothing
// writes that field any more, but episodeShortID still reads it, so it has to
// survive a decode.
func TestCacheIDFieldStillDecodes(t *testing.T) {
	raw := []byte(`{"episodes":[{"id":"e12345","path":"/lib/a.mp3","filename":"a.mp3"}]}`)
	var idx CachedPodcastIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatal(err)
	}
	if len(idx.Episodes) != 1 || idx.Episodes[0].ID != "e12345" {
		t.Fatalf("legacy id did not decode: %+v", idx.Episodes)
	}
	if idx.Episodes[0].Path != "/lib/a.mp3" {
		t.Errorf("embedded EpisodeFile did not decode: %+v", idx.Episodes[0])
	}
}

func assertGoldenJSON(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_CACHE_GOLDEN") != "" {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_CACHE_GOLDEN=1)", name, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s changed; on-disk caches would be misread.\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
