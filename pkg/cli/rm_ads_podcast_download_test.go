package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sariel/abs/pkg/backend"
)

func TestFindTargetEpisodeFromBackend_OnlyDownloadsLatest(t *testing.T) {
	tmp := t.TempDir()
	podDir := filepath.Join(tmp, "My Show")
	if err := os.MkdirAll(podDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	var downloadCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/podcasts/feed":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"podcast": map[string]interface{}{
					"episodes": []map[string]interface{}{
						{"title": "Ep 3", "pubDate": "Wed, 03 Sep 2026 12:00:00 GMT", "guid": "g-3", "enclosureUrl": "https://example.com/ep3.mp3"},
						{"title": "Ep 2", "pubDate": "Tue, 02 Sep 2026 12:00:00 GMT", "guid": "g-2", "enclosureUrl": "https://example.com/ep2.mp3"},
						{"title": "Ep 1", "pubDate": "Mon, 01 Sep 2026 12:00:00 GMT", "guid": "g-1", "enclosureUrl": "https://example.com/ep1.mp3"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/download-episodes"):
			atomic.AddInt32(&downloadCalls, 1)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "simulated bad request"})
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"libraries": []interface{}{map[string]interface{}{"id": "lib-1", "mediaType": "podcast"}}})
		case r.URL.Path == "/api/libraries/lib-1/items", r.URL.Path == "/api/items/item-1":
			itemMap := map[string]interface{}{
				"id": "item-1",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{"title": "My Show", "feedUrl": "https://example.com/feed.xml"},
					"episodes": []interface{}{},
				},
			}
			if r.URL.Path == "/api/items/item-1" {
				_ = json.NewEncoder(w).Encode(itemMap)
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{itemMap}})
			}
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	b := backend.NewAudiobookshelf(backend.Config{
		Host:        srv.URL,
		Token:       "test-tok",
		PodcastsDir: tmp,
		Quiet:       true,
	})

	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "My Show",
		ShortID:    "ms",
		FolderName: "My Show",
		UUID:       "item-1",
	}

	cfg := Config{PodcastsDir: tmp, AudiobookshelfURL: srv.URL, AudiobookshelfToken: "test-tok"}
	targetPath, handled := findTargetEpisodeFromBackend(b, resolved, cfg, true)

	if targetPath != "" {
		t.Errorf("expected targetPath empty on download failure, got %s", targetPath)
	}
	if !handled {
		t.Errorf("expected handled=true, got false")
	}
	if calls := atomic.LoadInt32(&downloadCalls); calls != 1 {
		t.Errorf("expected exactly 1 download attempt (for latest episode Ep 3), got %d attempts", calls)
	}
}

func TestFindLocalPathForFeedEpisode_ShowPrefixAndSubdir(t *testing.T) {
	tmp := t.TempDir()
	podDir := filepath.Join(tmp, "The Show")

	subDir := filepath.Join(podDir, "Episode 1")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	ep1Mp3 := filepath.Join(subDir, "podcast.mp3")
	if err := os.WriteFile(ep1Mp3, []byte("audio1"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ep2Mp3 := filepath.Join(podDir, "The Show - Episode 2.mp3")
	if err := os.WriteFile(ep2Mp3, []byte("audio2"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	fe1 := backend.FeedEpisode{Title: "Episode 1"}
	path1, ok1 := findLocalPathForFeedEpisode(podDir, fe1, nil)
	if !ok1 || path1 != ep1Mp3 {
		t.Fatalf("expected to resolve subfolder episode %s, got (%s, %v)", ep1Mp3, path1, ok1)
	}

	fe2 := backend.FeedEpisode{Title: "Episode 2"}
	path2, ok2 := findLocalPathForFeedEpisode(podDir, fe2, nil)
	if !ok2 || path2 != ep2Mp3 {
		t.Fatalf("expected to resolve show-prefixed episode %s, got (%s, %v)", ep2Mp3, path2, ok2)
	}
}

func TestResolveMatchingEpisodeAudioFile_ABSContainerPath(t *testing.T) {
	tmp := t.TempDir()
	podDir := filepath.Join(tmp, "The News")
	if err := os.MkdirAll(podDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	mp3Path := filepath.Join(podDir, "ep.mp3")
	if err := os.WriteFile(mp3Path, []byte("audio"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ep := backend.Episode{
		AudioFile: &backend.PodcastAudioFile{
			Metadata: &backend.AudioFileMetadata{
				Path: "/podcasts/The News/ep.mp3",
			},
		},
	}

	got, ok := resolveMatchingEpisodeAudioFile(podDir, ep)
	if !ok || got != mp3Path {
		t.Fatalf("expected (%s, true), got (%s, %v)", mp3Path, got, ok)
	}
}
