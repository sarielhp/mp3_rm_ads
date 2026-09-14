package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"pod/pkg/config"
	"pod/pkg/pipeline"
	"pod/pkg/types"
	"testing"
	"time"
)

func TestRemotePriorityInheritsWithoutStickingLocally(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "show")
	path := filepath.Join(dir, "episode", "podcast.mp3")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultPodcastConfig(nil)
	cfg.Priority = 8
	if err := config.SavePodcastConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	pipeline.SetPublicationSource(root, map[string]time.Time{path: date})
	defer pipeline.SetPublicationSource("", nil)
	transport := newMockTransport()
	if err := PushSingleAudioFile(path, root, "remote", "host", 0, transport); err != nil {
		t.Fatal(err)
	}
	var sent types.EpisodeStatusFile
	if err := json.Unmarshal(transport.uploadContent["remote/show/episode/podcast.mp3.json"], &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Priority != 8 || sent.PublishedAt != date.Format(time.RFC3339) {
		t.Fatalf("remote metadata=%+v", sent)
	}
	local, err := pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(path))
	if err != nil || local.Priority != 0 {
		t.Fatalf("local priority=%+v %v", local, err)
	}
	cfg.Priority = 2
	if err := config.SavePodcastConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if GetEpisodePriorityForQueue(path) != 2 {
		t.Fatal("lowering podcast priority did not take effect")
	}
	pulled := filepath.Join(root, "pulled.json")
	data, _ := json.Marshal(sent)
	if err := os.WriteFile(pulled, data, 0644); err != nil {
		t.Fatal(err)
	}
	savePulledEpisodeStatus(path, pulled)
	local, _ = pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(path))
	if local.Priority != 0 {
		t.Fatal("remote collection persisted inherited priority")
	}
}
