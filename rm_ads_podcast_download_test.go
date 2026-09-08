package main

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

func TestFindLocalPathForFeedEpisode_SiblingClean(t *testing.T) {
	tmp := t.TempDir()
	podParent := filepath.Join(tmp, "podcasts")
	podfetchDir := filepath.Join(podParent, "podfetch", "The Show")
	cleanDir := filepath.Join(podParent, "clean", "The Show")

	if err := os.MkdirAll(podfetchDir, 0755); err != nil {
		t.Fatalf("mkdir podfetch failed: %v", err)
	}
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		t.Fatalf("mkdir clean failed: %v", err)
	}

	cleanMp3 := filepath.Join(cleanDir, "The Show - Episode 1.mp3")
	if err := os.WriteFile(cleanMp3, []byte("audio"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	fe := backend.FeedEpisode{
		Title: "Episode 1",
	}

	path, ok := findLocalPathForFeedEpisode(podfetchDir, fe, nil)
	if !ok || path != cleanMp3 {
		t.Fatalf("expected to resolve cleanMp3 in sibling directory %s, got (%s, %v)", cleanMp3, path, ok)
	}
}

func TestResolveMatchingEpisodeAudioFile_ABSContainerPath(t *testing.T) {
	tmp := t.TempDir()
	podParent := filepath.Join(tmp, "podcasts")
	cleanDir := filepath.Join(podParent, "clean", "The News")
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	mp3Path := filepath.Join(cleanDir, "ep.mp3")
	if err := os.WriteFile(mp3Path, []byte("audio"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ep := backend.Episode{
		AudioFile: &backend.PodcastAudioFile{
			Metadata: &backend.AudioFileMetadata{
				Path: "/podcasts/clean/The News/ep.mp3",
			},
		},
	}

	got, ok := resolveMatchingEpisodeAudioFile(cleanDir, ep)
	if !ok || got != mp3Path {
		t.Fatalf("expected (%s, true), got (%s, %v)", mp3Path, got, ok)
	}

	podfetchDir := filepath.Join(podParent, "podfetch", "The News")
	_ = os.MkdirAll(podfetchDir, 0755)
	got2, ok2 := resolveMatchingEpisodeAudioFile(podfetchDir, ep)
	if !ok2 || got2 != mp3Path {
		t.Fatalf("expected sibling resolution (%s, true), got (%s, %v)", mp3Path, got2, ok2)
	}
}
