package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestGenerateEpisodeShortID(t *testing.T) {
	podID := "lxfrd"
	ep1 := "episode_101"
	ep2 := "episode_102"

	id1 := generateEpisodeShortID(podID, ep1)
	id2 := generateEpisodeShortID(podID, ep1)
	id3 := generateEpisodeShortID(podID, ep2)

	if id1 != id2 {
		t.Errorf("expected deterministic IDs for same input, got %q != %q", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("expected different IDs for different episodes, got %q == %q", id1, id3)
	}

	if len(id1) != 6 {
		t.Errorf("expected 6 characters for episode ID, got %d (%q)", len(id1), id1)
	}

	matched, _ := regexp.MatchString(`^e[0-9a-f]{5}$`, id1)
	if !matched {
		t.Errorf("expected 'e' followed by 5 lowercase hex chars, got %q", id1)
	}
}

func TestGetOrSetEpisodeShortIDPersistence(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Lex_Fridman")
	_ = os.MkdirAll(podDir, 0755)

	podID := getOrSetPodcastShortID(podDir, "Lex Fridman Podcast")
	audioFile := filepath.Join(podDir, "ep101.mp3")
	_ = os.WriteFile(audioFile, []byte("audio"), 0644)

	id1 := getOrSetEpisodeShortID(podDir, podID, audioFile)
	if len(id1) != 6 || id1[0] != 'e' {
		t.Errorf("unexpected generated episode ID: %q", id1)
	}

	st, err := loadEpisodeStatus(statusPathFor(audioFile))
	if err != nil {
		t.Fatalf("expected episode status file to exist: %v", err)
	}
	if st.ID != id1 {
		t.Errorf("expected persisted ID %q in status file, got %q", id1, st.ID)
	}

	id2 := getOrSetEpisodeShortID(podDir, podID, audioFile)
	if id2 != id1 {
		t.Errorf("expected reused ID %q on reload, got %q", id1, id2)
	}
}

func TestResolveAnyIDPodcasts(t *testing.T) {
	tempDir := t.TempDir()
	pod1 := filepath.Join(tempDir, "Daily_Show")
	pod2 := filepath.Join(tempDir, "Planet_Money")
	_ = os.MkdirAll(pod1, 0755)
	_ = os.MkdirAll(pod2, 0755)

	_ = os.WriteFile(filepath.Join(pod1, "ep1.mp3"), []byte("audio1"), 0644)
	_ = os.WriteFile(filepath.Join(pod2, "ep2.mp3"), []byte("audio2"), 0644)

	id1 := getOrSetPodcastShortID(pod1, "Daily Show")
	id2 := getOrSetPodcastShortID(pod2, "Planet Money")

	res1, err := resolveAnyID(tempDir, id1)
	if err != nil || !res1.IsPodcast() || res1.Podcast.ShortID != id1 {
		t.Fatalf("failed to resolve podcast by short ID %q: %v", id1, err)
	}

	res2, err := resolveAnyID(tempDir, "Planet_Money")
	if err != nil || !res2.IsPodcast() || res2.Podcast.ShortID != id2 {
		t.Fatalf("failed to resolve podcast by folder name: %v", err)
	}

	resIdx, err := resolveAnyID(tempDir, "1")
	if err != nil || !resIdx.IsPodcast() {
		t.Fatalf("failed to resolve podcast by index 1: %v", err)
	}

	resDir, err := resolveAnyID(tempDir, pod1)
	if err != nil || !resDir.IsPodcast() || resDir.Podcast.Dir != pod1 {
		t.Fatalf("failed to resolve podcast by absolute path: %v", err)
	}
}

func TestResolveAnyIDEpisodes(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Science_Friday")
	_ = os.MkdirAll(podDir, 0755)
	audioPath := filepath.Join(podDir, "ep100.mp3")
	_ = os.WriteFile(audioPath, []byte("audio"), 0644)

	podID := getOrSetPodcastShortID(podDir, "Science Friday")
	epID := getOrSetEpisodeShortID(podDir, podID, audioPath)

	resEp, err := resolveAnyID(tempDir, epID)
	if err != nil || !resEp.IsEpisode() {
		t.Fatalf("failed to resolve episode by 6-char ID %q: %v", epID, err)
	}
	if resEp.Episode.ShortID != epID || resEp.Episode.PodcastShortID != podID {
		t.Errorf("mismatched episode resolution: got %+v", resEp.Episode)
	}

	resPath, err := resolveAnyID(tempDir, audioPath)
	if err != nil || !resPath.IsEpisode() || resPath.Episode.Path != audioPath {
		t.Fatalf("failed to resolve episode by direct file path: %v", err)
	}

	resFn, err := resolveAnyID(tempDir, "ep100.mp3")
	if err != nil || !resFn.IsEpisode() {
		t.Fatalf("failed to resolve episode by filename: %v", err)
	}
}

func TestResolveAnyIDErrors(t *testing.T) {
	tempDir := t.TempDir()

	_, err := resolveAnyID(tempDir, "")
	if err == nil {
		t.Errorf("expected error for empty query")
	}

	_, err = resolveAnyID(tempDir, "non_existent_target_12345")
	if err == nil {
		t.Errorf("expected error for non-existent query")
	}
}

func TestResolveFolderBasedEpisodes(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "FiveFour")
	_ = os.MkdirAll(filepath.Join(podDir, "Episode One"), 0755)
	_ = os.MkdirAll(filepath.Join(podDir, "Episode Two"), 0755)

	ep1Audio := filepath.Join(podDir, "Episode One", "podcast.mp3")
	ep2Audio := filepath.Join(podDir, "Episode Two", "podcast.mp3")
	_ = os.WriteFile(ep1Audio, []byte("audio1"), 0644)
	_ = os.WriteFile(ep2Audio, []byte("audio2"), 0644)

	podID := getOrSetPodcastShortID(podDir, "FiveFour")
	ep1ID := getOrSetEpisodeShortID(podDir, podID, ep1Audio)
	ep2ID := getOrSetEpisodeShortID(podDir, podID, ep2Audio)

	if ep1ID == ep2ID {
		t.Fatalf("expected distinct episode IDs for different folder episodes, got %q for both", ep1ID)
	}

	res1, err := resolveAnyID(tempDir, ep1ID)
	if err != nil || !res1.IsEpisode() {
		t.Fatalf("failed to resolve ep1 by ID: %v", err)
	}
	if res1.Episode.Title != "Episode One" {
		t.Errorf("expected title 'Episode One', got %q", res1.Episode.Title)
	}

	resByTitle, err := resolveAnyID(tempDir, "Episode Two")
	if err != nil || !resByTitle.IsEpisode() {
		t.Fatalf("failed to resolve ep2 by folder title: %v", err)
	}
	if resByTitle.Episode.ShortID != ep2ID {
		t.Errorf("expected ID %q, got %q", ep2ID, resByTitle.Episode.ShortID)
	}
}
