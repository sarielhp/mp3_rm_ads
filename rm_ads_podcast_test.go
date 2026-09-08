package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sariel/abs/pkg/backend"
)

func createTestPodcastWithEpisodes(t *testing.T, root, podName string, titles []string) (string, []string) {
	podDir := filepath.Join(root, podName)
	if err := os.MkdirAll(podDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := defaultPodcastConfig()
	cfg.ID = generatePodcastShortID(podName)
	if err := savePodcastConfig(podDir, cfg); err != nil {
		t.Fatal(err)
	}

	var paths []string
	for i, title := range titles {
		filename := sanitizePodcastTitle(title) + ".mp3"
		p := filepath.Join(podDir, filename)
		if err := os.WriteFile(p, []byte("fake mp3 data "+title), 0644); err != nil {
			t.Fatal(err)
		}
		st := getOrCreateEpisodeStatus(p)
		st.PublishedAt = time.Now().Add(-time.Duration(len(titles)-i) * 24 * time.Hour).Format(time.RFC3339)
		if err := saveEpisodeStatus(statusPathFor(p), st); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return podDir, paths
}

func markEpisodeClean(t *testing.T, mp3Path string) {
	st := getOrCreateEpisodeStatus(mp3Path)
	st.Status = StateDone
	st.Original.DurationSec = 60.0
	st.Cleaned.DurationSec = 50.0
	if err := saveEpisodeStatus(statusPathFor(mp3Path), st); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePodcastTarget(t *testing.T) {
	tmp := t.TempDir()
	podDir, _ := createTestPodcastWithEpisodes(t, tmp, "Daily Tech", []string{"Ep 1"})
	cfg := loadPodcastConfig(podDir)

	cliShort := CLIOptions{Podcast: cfg.ID}
	p1, ok1 := resolvePodcastTarget(tmp, cliShort)
	if !ok1 || p1 == nil || p1.Dir != podDir {
		t.Fatalf("expected to resolve podcast by short ID %s, got %v", cfg.ID, p1)
	}

	cliArgShort := CLIOptions{Args: []string{cfg.ID}}
	p2, ok2 := resolvePodcastTarget(tmp, cliArgShort)
	if !ok2 || p2 == nil || p2.Dir != podDir {
		t.Fatalf("expected to resolve podcast by arg short ID, got %v", p2)
	}

	cliArgTitle := CLIOptions{Args: []string{"Daily Tech"}}
	p3, ok3 := resolvePodcastTarget(tmp, cliArgTitle)
	if !ok3 || p3 == nil || p3.Dir != podDir {
		t.Fatalf("expected to resolve podcast by title, got %v", p3)
	}

	cliArgIndex := CLIOptions{Args: []string{"1"}}
	p4, ok4 := resolvePodcastTarget(tmp, cliArgIndex)
	if !ok4 || p4 == nil || p4.Dir != podDir {
		t.Fatalf("expected to resolve podcast by index 1, got %v", p4)
	}

	cliMP3 := CLIOptions{Args: []string{"ep1.mp3"}}
	if _, ok := resolvePodcastTarget(tmp, cliMP3); ok {
		t.Errorf("expected .mp3 arg to NOT resolve as podcast")
	}

	cliJSON := CLIOptions{Args: []string{"ep1.transcript.json"}}
	if _, ok := resolvePodcastTarget(tmp, cliJSON); ok {
		t.Errorf("expected .json arg to NOT resolve as podcast")
	}

	cliRoot := CLIOptions{Args: []string{tmp}}
	if _, ok := resolvePodcastTarget(tmp, cliRoot); ok {
		t.Errorf("expected root podcasts directory to NOT resolve as a single podcast")
	}
}

func TestFindLatestUncleanedLocalEpisode(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Science Hour", []string{
		"Old Ep",
		"Middle Ep",
		"Newest Ep",
	})

	oldEp := paths[0]
	middleEp := paths[1]
	newestEp := paths[2]

	target, found := findLatestUncleanedLocalEpisode(podDir, "Science Hour", true)
	if !found || target != newestEp {
		t.Fatalf("expected newest uncleaned episode %s, got %s (found=%v)", newestEp, target, found)
	}

	markEpisodeClean(t, newestEp)
	target, found = findLatestUncleanedLocalEpisode(podDir, "Science Hour", true)
	if !found || target != middleEp {
		t.Fatalf("expected middle uncleaned episode %s, got %s (found=%v)", middleEp, target, found)
	}

	markEpisodeClean(t, middleEp)
	markEpisodeClean(t, oldEp)
	target, found = findLatestUncleanedLocalEpisode(podDir, "Science Hour", true)
	if found || target != "" {
		t.Fatalf("expected no uncleaned episode when all clean, got %s (found=%v)", target, found)
	}
}

func TestHandlePodcastRmAdsWorkflow_MultiItemQueueSkip(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "History Show", []string{
		"Napoleon Part 1",
		"Napoleon Part 2",
	})

	otherPodDir, otherPaths := createTestPodcastWithEpisodes(t, tmp, "Nature Show", []string{
		"Birds",
	})
	addEpisodeToQueueFile(otherPodDir, filepath.Base(otherPaths[0]))

	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "History Show",
		ShortID:    podCfg.ID,
		FolderName: "History Show",
		UUID:       podCfg.ID,
		Config:     podCfg,
	}

	cli := CLIOptions{Quiet: true}
	config := Config{PodcastsDir: tmp}

	err := handlePodcastRmAdsWorkflow(resolved, cli, config, "rm_ads")
	if err != nil {
		t.Fatalf("handlePodcastRmAdsWorkflow failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err != nil {
		t.Fatalf("expected queue.json in %s: %v", podDir, err)
	}
	var queued []string
	if err := json.Unmarshal(data, &queued); err != nil {
		t.Fatalf("failed to unmarshal queue: %v", err)
	}
	if len(queued) != 1 || queued[0] != filepath.Base(paths[1]) {
		t.Fatalf("expected %s in queue, got %v", filepath.Base(paths[1]), queued)
	}

	if isEpisodeClean(paths[1]) {
		t.Fatalf("episode should not have been cleaned immediately when multiple items in queue")
	}
}

func TestHandlePodcastRmAdsWorkflow_DryRun(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Coding Talk", []string{
		"Go 1.26",
	})
	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "Coding Talk",
		ShortID:    podCfg.ID,
		FolderName: "Coding Talk",
		UUID:       podCfg.ID,
		Config:     podCfg,
	}

	cli := CLIOptions{Quiet: true, DryRun: true}
	config := Config{PodcastsDir: tmp}

	err := handlePodcastRmAdsWorkflow(resolved, cli, config, "rm_ads")
	if err != nil {
		t.Fatalf("dry run workflow failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	if _, err := os.Stat(qFile); err == nil {
		t.Fatalf("expected queue.json not to be created in dry run")
	}
	if isEpisodeClean(paths[0]) {
		t.Fatalf("episode should not be cleaned in dry run")
	}
}

func TestFindTargetEpisodeFromBackend_FeedCatalog(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Show A", []string{
		"Episode 1",
	})
	markEpisodeClean(t, paths[0])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/podcasts/feed":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"podcast": map[string]interface{}{
					"episodes": []map[string]interface{}{
						{"title": "Episode 2", "pubDate": "Mon, 01 Sep 2026 12:00:00 GMT", "guid": "guid-2", "enclosureUrl": "https://example.com/ep2.mp3"},
						{"title": "Episode 1", "pubDate": "Sun, 31 Aug 2026 12:00:00 GMT", "guid": "guid-1", "enclosureUrl": "https://example.com/ep1.mp3"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/download-episodes"):
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			_ = os.WriteFile(filepath.Join(podDir, "Episode_2.mp3"), []byte("new ep 2"), 0644)
		case strings.HasSuffix(r.URL.Path, "/downloads"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"downloads": []interface{}{}})
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"libraries": []interface{}{map[string]interface{}{"id": "lib-1", "mediaType": "podcast"}}})
		case r.URL.Path == "/api/libraries/lib-1/items", strings.HasPrefix(r.URL.Path, "/api/items/"):
			itemMap := map[string]interface{}{
				"id": "item-1",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{"title": "Show A", "feedUrl": "https://example.com/feed.xml"},
					"episodes": []interface{}{
						map[string]interface{}{"title": "Episode 1", "guid": "guid-1", "enclosureURL": "https://example.com/ep1.mp3"},
					},
				},
			}
			if strings.HasPrefix(r.URL.Path, "/api/items/") {
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

	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "Show A",
		ShortID:    podCfg.ID,
		FolderName: "Show A",
		UUID:       "item-1",
		Config:     podCfg,
	}

	cfg := Config{PodcastsDir: tmp, AudiobookshelfURL: srv.URL, AudiobookshelfToken: "test-tok"}
	targetPath, ok := findTargetEpisodeFromBackend(b, resolved, cfg, true)
	if !ok || targetPath == "" {
		t.Fatalf("expected to download and find target episode 2, got %s (ok=%v)", targetPath, ok)
	}
	if filepath.Base(targetPath) != "Episode_2.mp3" && !strings.Contains(targetPath, "Episode") {
		t.Fatalf("unexpected target path: %s", targetPath)
	}
}

func TestFindTargetEpisodeFromBackend_AllClean(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Show B", []string{
		"Episode 1",
	})
	markEpisodeClean(t, paths[0])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/podcasts/feed":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"podcast": map[string]interface{}{
					"episodes": []map[string]interface{}{
						{"title": "Episode 1", "pubDate": "Sun, 31 Aug 2026 12:00:00 GMT", "guid": "guid-1", "enclosureUrl": "https://example.com/ep1.mp3"},
					},
				},
			})
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"libraries": []interface{}{map[string]interface{}{"id": "lib-1", "mediaType": "podcast"}}})
		case r.URL.Path == "/api/libraries/lib-1/items", strings.HasPrefix(r.URL.Path, "/api/items/"):
			itemMap := map[string]interface{}{
				"id": "item-b",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{"title": "Show B", "feedUrl": "https://example.com/feed.xml"},
					"episodes": []interface{}{
						map[string]interface{}{"title": "Episode 1", "guid": "guid-1", "enclosureURL": "https://example.com/ep1.mp3"},
					},
				},
			}
			if strings.HasPrefix(r.URL.Path, "/api/items/") {
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

	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "Show B",
		ShortID:    podCfg.ID,
		FolderName: "Show B",
		UUID:       "item-b",
		Config:     podCfg,
	}

	cfg := Config{PodcastsDir: tmp, AudiobookshelfURL: srv.URL, AudiobookshelfToken: "test-tok"}
	targetPath, handled := findTargetEpisodeFromBackend(b, resolved, cfg, true)
	if !handled || targetPath != "" {
		t.Fatalf("expected handled=true and targetPath='' when all episodes clean, got %s (handled=%v)", targetPath, handled)
	}
}

func TestProcessSingleQueuedTarget_LocalCompletion(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Clean Show", []string{
		"Episode Test",
	})
	targetAudio := paths[0]
	epFilename := filepath.Base(targetAudio)
	addEpisodeToQueueFile(podDir, epFilename)

	markEpisodeClean(t, targetAudio)

	err := processSingleQueuedTarget(podDir, targetAudio, "rm_ads", CLIOptions{Quiet: true, Local: true}, Config{PodcastsDir: tmp})
	if err != nil {
		t.Fatalf("processSingleQueuedTarget failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, _ := os.ReadFile(qFile)
	var entries []string
	_ = json.Unmarshal(data, &entries)
	for _, e := range entries {
		if strings.EqualFold(e, epFilename) {
			t.Fatalf("expected %s to be removed from queue.json, but was still present", epFilename)
		}
	}
}

func TestHandlePodcastRmAdsWorkflow_OfflineBackendFallback(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Offline Show", []string{
		"Episode 1",
		"Episode 2",
	})
	markEpisodeClean(t, paths[0])

	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "Offline Show",
		ShortID:    podCfg.ID,
		FolderName: "Offline Show",
		UUID:       podCfg.ID,
		Config:     podCfg,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := Config{
		PodcastsDir:         tmp,
		AudiobookshelfURL:   srv.URL,
		AudiobookshelfToken: "invalid",
	}

	targetPath, err := resolveTargetEpisodeForRmAds(resolved, CLIOptions{Quiet: true}, cfg)
	if err != nil {
		t.Fatalf("unexpected error during offline fallback: %v", err)
	}
	if targetPath != paths[1] {
		t.Fatalf("expected offline fallback to select %s, got %s", paths[1], targetPath)
	}
}

func TestHandlePodcastRmAdsWorkflow_QueueSingleAndRemove(t *testing.T) {
	tmp := t.TempDir()
	podDir, paths := createTestPodcastWithEpisodes(t, tmp, "Solo Show", []string{
		"Ep 1",
	})
	markEpisodeClean(t, paths[0])

	podCfg := loadPodcastConfig(podDir)
	resolved := &ResolvedPodcast{
		Dir:        podDir,
		Title:      "Solo Show",
		ShortID:    podCfg.ID,
		FolderName: "Solo Show",
		UUID:       podCfg.ID,
		Config:     podCfg,
	}

	cli := CLIOptions{Quiet: true, Local: true}
	config := Config{PodcastsDir: tmp}

	addEpisodeToQueueFile(resolved.Dir, filepath.Base(paths[0]))

	err := processSingleQueuedTarget(resolved.Dir, paths[0], "rm_ads", cli, config)
	if err != nil {
		t.Fatalf("processSingleQueuedTarget failed: %v", err)
	}

	qFile := filepath.Join(resolved.Dir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err == nil {
		var entries []string
		_ = json.Unmarshal(data, &entries)
		for _, e := range entries {
			if strings.EqualFold(e, filepath.Base(paths[0])) {
				t.Fatalf("expected episode to be removed from queue file after processing")
			}
		}
	}
}

func TestCountAllQueuedEpisodes(t *testing.T) {
	tmp := t.TempDir()
	p1, _ := createTestPodcastWithEpisodes(t, tmp, "Show 1", []string{"E1"})
	p2, _ := createTestPodcastWithEpisodes(t, tmp, "Show 2", []string{"E2"})

	if count := countAllQueuedEpisodes(tmp); count != 0 {
		t.Fatalf("expected 0 queued, got %d", count)
	}

	addEpisodeToQueueFile(p1, "E1.mp3")
	if count := countAllQueuedEpisodes(tmp); count != 1 {
		t.Fatalf("expected 1 queued, got %d", count)
	}

	addEpisodeToQueueFile(p2, "E2.mp3")
	if count := countAllQueuedEpisodes(tmp); count != 2 {
		t.Fatalf("expected 2 queued, got %d", count)
	}
}

func TestProcessSingleQueuedTarget_Remote(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	defer setRemoteTransport(&DefaultSSHTransport{})

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)

	podDir, paths := createTestPodcastWithEpisodes(t, localPodcasts, "Remote Show", []string{
		"Ep 1",
	})
	targetAudio := paths[0]
	epFilename := filepath.Base(targetAudio)
	addEpisodeToQueueFile(podDir, epFilename)

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	_ = os.MkdirAll(remoteWorkDir, 0755)

	cfg := Config{
		RemoteHost:    "mock-box",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}
	cli := CLIOptions{
		Quiet:      true,
		Remote:     true,
		RemoteHost: "mock-box",
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		relPath := filepath.Join("Remote Show", "Ep 1.mp3")
		remEpPath := filepath.Join(remoteWorkDir, relPath)
		_ = os.MkdirAll(filepath.Dir(remEpPath), 0755)
		_ = os.WriteFile(remEpPath, []byte("cleaned remote audio"), 0644)
		remStat := statusPathFor(remEpPath)
		_ = saveEpisodeStatus(remStat, &EpisodeStatusFile{
			MediaFile: "Ep 1.mp3",
			Status:    StateReadyForCopyBack,
			Original:  EpisodeAudioMeta{DurationSec: 100},
			Cleaned:   EpisodeAudioMeta{DurationSec: 80},
		})
		donePath := filepath.Join(remoteWorkDir, "done.json")
		_ = addDoneEpisode(donePath, RemoteDoneItem{
			RelPath:          relPath,
			Status:           StateReadyForCopyBack,
			CleanedSizeBytes: int64(len("cleaned remote audio")),
			CutDurationSec:   20,
		})
	}()

	err := processSingleQueuedTarget(podDir, targetAudio, "rm_ads", cli, cfg)
	if err != nil {
		t.Fatalf("processSingleQueuedTarget remote failed: %v", err)
	}

	qFile := filepath.Join(podDir, "queue.json")
	data, err := os.ReadFile(qFile)
	if err == nil {
		var entries []string
		_ = json.Unmarshal(data, &entries)
		for _, e := range entries {
			if strings.EqualFold(e, epFilename) {
				t.Fatalf("expected %s to be removed from queue.json", epFilename)
			}
		}
	}
}
