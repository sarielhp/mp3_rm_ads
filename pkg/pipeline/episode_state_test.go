package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"abs/pkg/types"
)

func TestIsEpisodeCleanRequiresNonEmptyTranscript(t *testing.T) {
	audioPath := filepath.Join(t.TempDir(), "episode.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	st := GetOrCreateEpisodeStatus(audioPath)
	st.Status = types.StateDone
	if err := SaveEpisodeStatus(StatusPathFor(audioPath), st); err != nil {
		t.Fatal(err)
	}
	if IsEpisodeClean(audioPath) {
		t.Fatal("completed episode without a transcript is clean")
	}

	transcriptPath := filepath.Join(filepath.Dir(audioPath), "episode.transcript.json")
	if err := os.WriteFile(transcriptPath, []byte(`{"text":"This episode contains a complete discussion with enough meaningful transcript text."}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsEpisodeClean(audioPath) {
		t.Fatal("completed episode with a non-empty transcript is not clean")
	}
}

func TestGetOrCreateEpisodeStatusFavoriteInheritance(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "FavPodcast")
	_ = os.MkdirAll(podDir, 0755)

	podCfgJSON := `{"favorite":true}`
	_ = os.WriteFile(filepath.Join(podDir, "podcast.json"), []byte(podCfgJSON), 0644)

	audioPath := filepath.Join(podDir, "new_episode.mp3")
	_ = os.WriteFile(audioPath, []byte("audio"), 0644)

	st := GetOrCreateEpisodeStatus(audioPath)
	if !st.IsFavorite() {
		t.Errorf("expected new episode status to inherit favorite from podcast config")
	}

	loaded, err := LoadEpisodeStatus(StatusPathFor(audioPath))
	if err != nil {
		t.Fatalf("LoadEpisodeStatus failed: %v", err)
	}
	if !loaded.IsFavorite() || !loaded.Favorite || !loaded.Favourite {
		t.Errorf("expected loaded episode status to have favorite: true")
	}
}
