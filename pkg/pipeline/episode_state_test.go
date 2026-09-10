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
