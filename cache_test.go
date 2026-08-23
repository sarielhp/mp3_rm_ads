package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCachePodcastIndex(t *testing.T) {
	tempDir := t.TempDir()
	origCache := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", origCache)
	os.Setenv("XDG_CACHE_HOME", tempDir)

	podDir := filepath.Join(tempDir, "podcasts", "Sample Podcast")
	os.MkdirAll(podDir, 0755)

	absPath, _ := filepath.Abs(filepath.Join(podDir, "ep1.mp3"))

	index := CachedPodcastIndex{
		PodcastName: "Sample Podcast",
		PodcastDir:  podDir,
		ABSItemID:   "item_test_123",
		UpdatedAt:   time.Now(),
		Episodes: []CachedEpisodeSummary{
			{
				Path:          absPath,
				Filename:      "ep1.mp3",
				Title:         "Episode 1: Awakening",
				PublishedAt:   1700000000000,
				Duration:      3600.0,
				FileSize:      50000000,
				Season:        "1",
				Episode:       "1",
				HasAdsRemoved: true,
			},
		},
	}

	if err := savePodcastCache(podDir, &index); err != nil {
		t.Fatalf("savePodcastCache failed: %v", err)
	}

	loaded, err := loadPodcastCache(podDir)
	if err != nil {
		t.Fatalf("loadPodcastCache failed: %v", err)
	}

	if loaded.PodcastName != "Sample Podcast" {
		t.Errorf("expected PodcastName 'Sample Podcast', got %q", loaded.PodcastName)
	}
	if len(loaded.Episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(loaded.Episodes))
	}
	if loaded.Episodes[0].Path != absPath {
		t.Errorf("expected Path %q, got %q", absPath, loaded.Episodes[0].Path)
	}
	if loaded.Episodes[0].Title != "Episode 1: Awakening" {
		t.Errorf("expected Title 'Episode 1: Awakening', got %q", loaded.Episodes[0].Title)
	}
	if loaded.Episodes[0].PublishedAt != 1700000000000 {
		t.Errorf("expected PublishedAt 1700000000000, got %d", loaded.Episodes[0].PublishedAt)
	}
}

func TestCacheEpisodeDetails(t *testing.T) {
	tempDir := t.TempDir()
	origCache := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", origCache)
	os.Setenv("XDG_CACHE_HOME", tempDir)

	podDir := filepath.Join(tempDir, "podcasts", "Sample Podcast")
	os.MkdirAll(podDir, 0755)

	absPath, _ := filepath.Abs(filepath.Join(podDir, "ep1.mp3"))

	details := CachedEpisodeDetails{
		Path:        absPath,
		Filename:    "ep1.mp3",
		Title:       "Episode 1: Awakening",
		Description: "<p>Full episode description...</p>",
		Subtitle:    "A grand journey begins",
		EpisodeType: "full",
		Genres:      []string{"Sci-Fi", "Drama"},
		Author:      "Jane Doe",
		FeedURL:     "https://example.com/feed.xml",
	}

	if err := saveEpisodeDetails(podDir, "ep1.mp3", &details); err != nil {
		t.Fatalf("saveEpisodeDetails failed: %v", err)
	}

	loaded, err := loadEpisodeDetails(podDir, "ep1.mp3")
	if err != nil {
		t.Fatalf("loadEpisodeDetails failed: %v", err)
	}

	if loaded.Path != absPath {
		t.Errorf("expected Path %q, got %q", absPath, loaded.Path)
	}
	if loaded.Description != "<p>Full episode description...</p>" {
		t.Errorf("expected Description '<p>Full episode description...</p>', got %q", loaded.Description)
	}
	if loaded.Subtitle != "A grand journey begins" {
		t.Errorf("expected Subtitle 'A grand journey begins', got %q", loaded.Subtitle)
	}
}

func TestLoadTUIPodcastsWithCache(t *testing.T) {
	tempDir := t.TempDir()
	origCache := os.Getenv("XDG_CACHE_HOME")
	defer os.Setenv("XDG_CACHE_HOME", origCache)
	os.Setenv("XDG_CACHE_HOME", tempDir)

	podcastsDir := filepath.Join(tempDir, "my_podcasts")
	podDir := filepath.Join(podcastsDir, "TechShow")
	os.MkdirAll(podDir, 0755)

	mp3File := filepath.Join(podDir, "ep10.mp3")
	os.WriteFile(mp3File, []byte("fake mp3 audio"), 0644)
	absPath, _ := filepath.Abs(mp3File)

	index := CachedPodcastIndex{
		PodcastName: "TechShow",
		PodcastDir:  podDir,
		UpdatedAt:   time.Now(),
		Episodes: []CachedEpisodeSummary{
			{
				Path:        absPath,
				Filename:    "ep10.mp3",
				Title:       "Rich Episode Title from Cache",
				PublishedAt: 1724000000000,
				Duration:    1800.0,
				Season:      "2",
				Episode:     "10",
			},
		},
	}
	_ = savePodcastCache(podDir, &index)

	pods, err := loadTUIPodcasts(podcastsDir)
	if err != nil {
		t.Fatalf("loadTUIPodcasts failed: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 podcast, got %d", len(pods))
	}
	if len(pods[0].episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(pods[0].episodes))
	}

	ep := pods[0].episodes[0]
	if ep.title != "Rich Episode Title from Cache" {
		t.Errorf("expected cached title, got %q", ep.title)
	}
	if ep.publishedAt != 1724000000000 {
		t.Errorf("expected cached publishedAt, got %d", ep.publishedAt)
	}
	if ep.displayTitle() != "Rich Episode Title from Cache" {
		t.Errorf("expected displayTitle to return cached title, got %q", ep.displayTitle())
	}
	expectedDate := time.UnixMilli(1724000000000)
	if !ep.displayDate().Equal(expectedDate) {
		t.Errorf("expected displayDate to match publishedAt %v, got %v", expectedDate, ep.displayDate())
	}
}

func TestResetCache(t *testing.T) {
	tempCache := t.TempDir()
	origXDG := os.Getenv("XDG_CACHE_HOME")
	os.Setenv("XDG_CACHE_HOME", tempCache)
	defer os.Setenv("XDG_CACHE_HOME", origXDG)

	podDir := filepath.Join(tempCache, "dummy_podcast")
	savePodcastCache(podDir, &CachedPodcastIndex{PodcastName: "Dummy"})

	absDir := filepath.Join(tempCache, "abs")
	if _, err := os.Stat(absDir); err != nil {
		t.Fatalf("expected abs cache dir to exist before reset: %v", err)
	}

	if err := resetCache(); err != nil {
		t.Fatalf("resetCache failed: %v", err)
	}

	if _, err := os.Stat(absDir); !os.IsNotExist(err) {
		t.Errorf("expected abs cache dir to be removed after resetCache")
	}
}

func TestCoverImageDiskAndMemoryCache(t *testing.T) {
	tempCache := t.TempDir()
	origXDG := os.Getenv("XDG_CACHE_HOME")
	os.Setenv("XDG_CACHE_HOME", tempCache)
	defer os.Setenv("XDG_CACHE_HOME", origXDG)

	// Create a dummy 2x2 PNG image
	imgPath := filepath.Join(tempCache, "cover.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("failed to create dummy image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("failed to encode dummy image: %v", err)
	}
	f.Close()

	// 1. First call: processes and stores in L1 and L2
	res1, err := encodeKittyGraphicsFile(imgPath, 10, 10)
	if err != nil || res1 == "" {
		t.Fatalf("encodeKittyGraphicsFile failed: %v", err)
	}

	// Verify disk file was created in podcast's cache directory
	cDir := podcastCacheDirForImage(imgPath)
	files, err := os.ReadDir(cDir)
	if err != nil || len(files) == 0 {
		t.Fatalf("expected cached .esc files in podcast cache dir %s, got err: %v, count: %d", cDir, err, len(files))
	}
	foundEsc := false
	for _, fi := range files {
		if strings.HasSuffix(fi.Name(), ".esc") {
			foundEsc = true
			break
		}
	}
	if !foundEsc {
		t.Fatalf("expected .esc file in %s, got files: %+v", cDir, files)
	}

	// 2. Clear L1 memory cache and verify it loads from L2 disk cache
	coverGraphicsCacheMu.Lock()
	coverGraphicsCache = make(map[string]string)
	coverGraphicsCacheMu.Unlock()

	res2, err := encodeKittyGraphicsFile(imgPath, 10, 10)
	if err != nil || res2 != res1 {
		t.Fatalf("expected res2 to match res1 from disk cache, got %v (err: %v)", res2 == res1, err)
	}
}
