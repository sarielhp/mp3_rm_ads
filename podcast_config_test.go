package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPodcastConfigDefaults(t *testing.T) {
	d := t.TempDir()
	cfg := loadPodcastConfig(d)
	if cfg.AdRemoval != AdRemovalNone {
		t.Errorf("expected default AdRemoval to be %q, got %q", AdRemovalNone, cfg.AdRemoval)
	}
	if adRemovalModeLabel(cfg.AdRemoval) != "No ad removal" {
		t.Errorf("unexpected label: %s", adRemovalModeLabel(cfg.AdRemoval))
	}
}

func TestPodcastConfigCycle(t *testing.T) {
	m0 := AdRemovalNone
	m1 := cycleAdRemovalMode(m0)
	if m1 != AdRemovalLatest {
		t.Errorf("expected cycle from none to latest, got %s", m1)
	}
	m2 := cycleAdRemovalMode(m1)
	if m2 != AdRemovalAll {
		t.Errorf("expected cycle from latest to all, got %s", m2)
	}
	m3 := cycleAdRemovalMode(m2)
	if m3 != AdRemovalNone {
		t.Errorf("expected cycle from all to none, got %s", m3)
	}
}

func TestPodcastConfigSaveLoad(t *testing.T) {
	d := t.TempDir()

	cfg := PodcastConfig{
		AdRemoval: AdRemovalLatest,
	}
	if err := savePodcastConfig(d, cfg); err != nil {
		t.Fatalf("savePodcastConfig failed: %v", err)
	}

	loaded := loadPodcastConfig(d)
	if loaded.AdRemoval != AdRemovalLatest {
		t.Errorf("expected loaded AdRemoval %q, got %q", AdRemovalLatest, loaded.AdRemoval)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}

	cfg.AdRemoval = AdRemovalAll
	if err := savePodcastConfig(d, cfg); err != nil {
		t.Fatalf("savePodcastConfig failed on update: %v", err)
	}

	loaded2 := loadPodcastConfig(d)
	if loaded2.AdRemoval != AdRemovalAll {
		t.Errorf("expected loaded AdRemoval %q, got %q", AdRemovalAll, loaded2.AdRemoval)
	}
}

func TestFilterMP3FilesByPodcastConfig(t *testing.T) {
	d := t.TempDir()

	f1 := filepath.Join(d, "ep1.mp3")
	f2 := filepath.Join(d, "ep2.mp3")
	f3 := filepath.Join(d, "ep3.mp3")

	os.WriteFile(f1, []byte("audio1"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f2, []byte("audio2"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f3, []byte("audio3"), 0644)

	files := []string{f1, f2, f3}

	noneFiltered := filterMP3FilesByPodcastConfig(files, d, PodcastConfig{AdRemoval: AdRemovalNone})
	if len(noneFiltered) != 0 {
		t.Errorf("expected 0 files for none, got %d", len(noneFiltered))
	}

	allFiltered := filterMP3FilesByPodcastConfig(files, d, PodcastConfig{AdRemoval: AdRemovalAll})
	if len(allFiltered) != 3 {
		t.Errorf("expected 3 files for all, got %d", len(allFiltered))
	}

	latestFiltered := filterMP3FilesByPodcastConfig(files, d, PodcastConfig{AdRemoval: AdRemovalLatest})
	if len(latestFiltered) != 1 || latestFiltered[0] != f3 {
		t.Errorf("expected latest file %q, got %+v", f3, latestFiltered)
	}

	os.WriteFile(filepath.Join(d, "ep3.cuts.json"), []byte("{}"), 0644)
	latestFiltered2 := filterMP3FilesByPodcastConfig(files, d, PodcastConfig{AdRemoval: AdRemovalLatest})
	if len(latestFiltered2) != 1 || latestFiltered2[0] != f2 {
		t.Errorf("expected next uncut file %q, got %+v", f2, latestFiltered2)
	}
}
