package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedABSStatusLocalFallback(t *testing.T) {
	tempDir := t.TempDir()
	podDir1 := filepath.Join(tempDir, "Tech_Podcast")
	podDir2 := filepath.Join(tempDir, "History_Show")
	_ = os.MkdirAll(podDir1, 0755)
	_ = os.MkdirAll(podDir2, 0755)

	_ = os.WriteFile(filepath.Join(podDir1, "ep1.mp3"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(podDir1, "ep2.mp3"), []byte("audio"), 0644)
	_ = os.WriteFile(filepath.Join(podDir1, "ep2.cuts.json"), []byte(`{"cut_intervals":[]}`), 0644)
	_ = saveEpisodeStatus(statusPathFor(filepath.Join(podDir1, "ep2.mp3")), &EpisodeStatusFile{
		Status: StateDone,
	})

	_ = os.WriteFile(filepath.Join(podDir2, "ep10.mp3"), []byte("audio"), 0644)

	cfg := Config{
		PodcastsDir: tempDir,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	absStatus(cfg, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "LOCAL LIBRARY PODCAST STATUS REPORT") {
		t.Errorf("expected local status table header, got: %s", out)
	}
	if !strings.Contains(out, "History_Show") || !strings.Contains(out, "Tech_Podcast") {
		t.Errorf("expected both podcast names in table, got: %s", out)
	}
	if !strings.Contains(out, "TOTAL") {
		t.Errorf("expected TOTAL summary line, got: %s", out)
	}
}

func TestUnifiedABSStatusWithRemoteAndLocal(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	_ = os.MkdirAll(remoteWorkDir, 0755)
	donePath := filepath.Join(remoteWorkDir, "done.json")
	_ = addDoneEpisode(donePath, RemoteDoneItem{
		RelPath:            filepath.Join("Show", "ep1.mp3"),
		Status:             StateReadyForCopyBack,
		CleanedDurationSec: 120.0,
		CutDurationSec:     15.0,
	})

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	podDir := filepath.Join(localPodcasts, "Show")
	_ = os.MkdirAll(podDir, 0755)
	_ = os.WriteFile(filepath.Join(podDir, "ep1.mp3"), []byte("audio"), 0644)

	cfg := Config{
		RemoteHost:    "mock-host",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	absStatus(cfg, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Remote Server Status: mock-host") {
		t.Errorf("expected Section 1 Remote Server Status, got: %s", out)
	}
	if !strings.Contains(out, "Ready to Pull:") || !strings.Contains(out, "1 episode(s)") {
		t.Errorf("expected Ready to Pull in remote section, got: %s", out)
	}
	if !strings.Contains(out, "LOCAL LIBRARY PODCAST STATUS REPORT") {
		t.Errorf("expected Section 2 Local Library Status, got: %s", out)
	}
}

func TestUnifiedABSStatusRemoteUnreachable(t *testing.T) {
	tempDir := t.TempDir()
	mock := &MockRemoteTransport{
		RemoteRoot: tempDir,
		Reachable:  false,
	}
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	podDir := filepath.Join(localPodcasts, "Show")
	_ = os.MkdirAll(podDir, 0755)
	_ = os.WriteFile(filepath.Join(podDir, "ep1.mp3"), []byte("audio"), 0644)

	cfg := Config{
		RemoteHost:  "unreachable-server",
		PodcastsDir: localPodcasts,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	absStatus(cfg, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "Remote Host: unreachable-server [UNREACHABLE]") && !strings.Contains(out, "unreachable-server [UNREACHABLE]") {
		t.Errorf("expected [UNREACHABLE] in remote status, got: %s", out)
	}
	if !strings.Contains(out, "LOCAL LIBRARY PODCAST STATUS REPORT") {
		t.Errorf("expected Section 2 local status table even when remote is unreachable, got: %s", out)
	}
}

func TestUnifiedABSStatusWithLiveABS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"token": "test-token"},
			})
		case r.URL.Path == "/api/libraries":
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(absLibrariesResp{
				Libraries: []absLibrary{
					{ID: "lib-1", Name: "Podcasts", MediaType: "podcast"},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			w.WriteHeader(200)
			item := absItem{
				ID:      "item-1",
				RelPath: "Tech_Talk",
			}
			item.Media.Metadata.Title = "Tech Talk"
			item.Media.Episodes = []absEpisode{
				{ID: "ep-1", Title: "Episode 1"},
				{ID: "ep-2", Title: "Episode 2"},
			}
			_ = json.NewEncoder(w).Encode(absItemsResp{
				Results: []absItem{item},
			})
		case r.URL.Path == "/api/items/item-1":
			w.WriteHeader(200)
			item := absItem{
				ID:      "item-1",
				RelPath: "Tech_Talk",
			}
			item.Media.Metadata.Title = "Tech Talk"
			item.Media.Episodes = []absEpisode{
				{ID: "ep-1", Title: "Episode 1"},
				{ID: "ep-2", Title: "Episode 2"},
			}
			_ = json.NewEncoder(w).Encode(item)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Tech_Talk")
	_ = os.MkdirAll(podDir, 0755)
	_ = os.WriteFile(filepath.Join(podDir, "ep1.mp3"), []byte("audio"), 0644)

	cfg := Config{
		AudiobookshelfURL:  srv.URL,
		AudiobookshelfUser: "user",
		AudiobookshelfPass: "pass",
		PodcastsDir:        tempDir,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	absStatus(cfg, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "AUDIOBOOKSHELF DATABASE STATUS REPORT") {
		t.Errorf("expected ABS database status report header, got: %s", out)
	}
	if !strings.Contains(out, "Tech Talk") {
		t.Errorf("expected Tech Talk in ABS table, got: %s", out)
	}
}

func TestSyncAudiobookshelfDuration(t *testing.T) {
	scannedItem := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]string{"token": "test-token"},
			})
		case r.URL.Path == "/api/libraries":
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(absLibrariesResp{
				Libraries: []absLibrary{
					{ID: "lib-1", Name: "Podcasts", MediaType: "podcast"},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			w.WriteHeader(200)
			item := absItem{
				ID:      "item-123",
				RelPath: "Science_Show",
			}
			item.Media.Metadata.Title = "Science_Show"
			_ = json.NewEncoder(w).Encode(absItemsResp{
				Results: []absItem{item},
			})
		case r.URL.Path == "/api/items/item-123":
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "item-123",
				"media": map[string]any{
					"metadata": map[string]string{
						"title": "Science_Show",
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/api/items/") && strings.HasSuffix(r.URL.Path, "/scan"):
			parts := strings.Split(r.URL.Path, "/")
			if len(parts) >= 4 {
				scannedItem = parts[3]
			}
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	cfg := Config{
		AudiobookshelfURL:  srv.URL,
		AudiobookshelfUser: "user",
		AudiobookshelfPass: "pass",
	}

	syncAudiobookshelfDuration(&cfg, "/tmp/podcasts/Science_Show/episode1.mp3", 150.0)

	if scannedItem != "item-123" {
		t.Errorf("expected item-123 to be scanned on ABS, got %s", scannedItem)
	}
}
