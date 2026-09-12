package cli

import (
	"abs/pkg/config"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFavoriteSubcommandMarkAndUnmark(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Show_Fav")
	_ = os.MkdirAll(podDir, 0755)
	podID := podcast.GetOrSetPodcastShortID(podDir, "Show Fav")

	cfg := Config{PodcastsDir: tempDir}

	// 1. Mark as favorite
	cliMark := CLIOptions{Args: []string{podID}}
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := handleServerFavorite(cfg, cliMark)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("handleServerFavorite mark failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Marked as favorite") {
		t.Errorf("expected output to mention Marked as favorite, got: %s", string(outBytes))
	}

	podCfg := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if !podCfg.Favorite {
		t.Errorf("expected Favorite to be true")
	}
	if !podCfg.IsAutoDownloadEnabled() {
		t.Errorf("expected AutoDownload to be true")
	}
	if podCfg.DownloadPolicy != config.DownloadPolicyNew {
		t.Errorf("expected DownloadPolicy to be %q, got %q", config.DownloadPolicyNew, podCfg.DownloadPolicy)
	}
	if podCfg.AdRemoval != config.AdRemovalAll {
		t.Errorf("expected AdRemoval to be %q, got %q", config.AdRemovalAll, podCfg.AdRemoval)
	}
	if podCfg.FavoriteSince == nil {
		t.Errorf("expected FavoriteSince to be non-nil")
	}

	// 2. Unmark as favorite
	cliUnmark := CLIOptions{Args: []string{podID, "off"}}
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	err = handleServerFavorite(cfg, cliUnmark)
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ = io.ReadAll(r)

	if err != nil {
		t.Fatalf("handleServerFavorite unmark failed: %v", err)
	}
	if !strings.Contains(string(outBytes), "Removed from favorites") {
		t.Errorf("expected output to mention Removed from favorites, got: %s", string(outBytes))
	}

	podCfgUnmarked := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if podCfgUnmarked.Favorite {
		t.Errorf("expected Favorite to be false")
	}
	if podCfgUnmarked.DownloadPolicy != config.DownloadPolicyNone {
		t.Errorf("expected DownloadPolicy none, got %s", podCfgUnmarked.DownloadPolicy)
	}
}

func TestFavoriteSubcommandList(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Show_List")
	_ = os.MkdirAll(podDir, 0755)
	podID := podcast.GetOrSetPodcastShortID(podDir, "Show List")

	cfg := Config{PodcastsDir: tempDir}

	pCfg := config.PodcastConfig{}
	pCfg.SetFavorite(true)
	_ = config.SavePodcastConfig(podDir, pCfg)

	// List text
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	err := handleServerFavorite(cfg, CLIOptions{})
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("handleServerFavorite list failed: %v", err)
	}
	if !strings.Contains(string(outBytes), podID) || !strings.Contains(string(outBytes), "Show") {
		t.Errorf("expected list output to contain podcast, got: %s", string(outBytes))
	}

	// List JSON
	r, w, _ = os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	err = handleServerFavorite(cfg, CLIOptions{JSON: true})
	_ = w.Close()
	os.Stdout = oldStdout
	jsonBytes, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("handleServerFavorite list json failed: %v", err)
	}
	var results []FavoritePodcastResult
	if err := json.Unmarshal(jsonBytes, &results); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(results) != 1 || results[0].ID != podID {
		t.Errorf("expected 1 result with ID %s, got: %+v", podID, results)
	}
}

func TestFavoritePodcastDoesNotMarkExistingEpisodes(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Show_EpFav")
	_ = os.MkdirAll(podDir, 0755)
	podID := podcast.GetOrSetPodcastShortID(podDir, "Show EpFav")

	mp3Path := filepath.Join(podDir, "existing_ep.mp3")
	_ = os.WriteFile(mp3Path, []byte("fake audio content"), 0644)
	oldTime := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(mp3Path, oldTime, oldTime)

	st := pipeline.GetOrCreateEpisodeStatus(mp3Path)
	if st.IsFavorite() {
		t.Fatalf("expected initial episode status not favorite")
	}

	cfg := Config{PodcastsDir: tempDir}
	cliMark := CLIOptions{Args: []string{podID}}
	if err := handleServerFavorite(cfg, cliMark); err != nil {
		t.Fatalf("handleServerFavorite failed: %v", err)
	}

	existingSt, err := pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(mp3Path))
	if err != nil {
		t.Fatalf("LoadEpisodeStatus failed: %v", err)
	}
	if existingSt.IsFavorite() {
		t.Errorf("expected existing episode status file to remain not favorite")
	}

	newMp3Path := filepath.Join(podDir, "new_downloaded.mp3")
	_ = os.WriteFile(newMp3Path, []byte("new audio"), 0644)
	newTime := time.Now().Add(5 * time.Minute)
	_ = os.Chtimes(newMp3Path, newTime, newTime)

	newSt := pipeline.GetOrCreateEpisodeStatus(newMp3Path)
	if !newSt.IsFavorite() {
		t.Errorf("expected newly downloaded episode status file to have favorite: true")
	}
}

func TestFavoriteSingleEpisode(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Show_SingleEp")
	_ = os.MkdirAll(podDir, 0755)
	podID := podcast.GetOrSetPodcastShortID(podDir, "Show SingleEp")

	mp3Path := filepath.Join(podDir, "single_ep.mp3")
	_ = os.WriteFile(mp3Path, []byte("fake audio content"), 0644)
	_ = pipeline.GetOrCreateEpisodeStatus(mp3Path)

	epID := podcast.GetOrSetEpisodeShortID(podDir, podID, mp3Path)
	cfg := Config{PodcastsDir: tempDir}

	cliMark := CLIOptions{Args: []string{epID}}
	if err := handleServerFavorite(cfg, cliMark); err != nil {
		t.Fatalf("handleServerFavorite single episode failed: %v", err)
	}

	st, err := pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(mp3Path))
	if err != nil {
		t.Fatalf("LoadEpisodeStatus failed: %v", err)
	}
	if !st.IsFavorite() {
		t.Errorf("expected episode status file to have favorite: true")
	}

	podCfg := config.LoadPodcastConfig(podDir, config.PodcastConfig{})
	if podCfg.Favorite {
		t.Errorf("expected podcast to remain not favorite")
	}

	cliUnmark := CLIOptions{Args: []string{epID, "off"}}
	if err := handleServerFavorite(cfg, cliUnmark); err != nil {
		t.Fatalf("handleServerFavorite single episode unmark failed: %v", err)
	}

	unmarkedSt, _ := pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(mp3Path))
	if unmarkedSt.IsFavorite() {
		t.Errorf("expected episode status file to have favorite: false")
	}
}
