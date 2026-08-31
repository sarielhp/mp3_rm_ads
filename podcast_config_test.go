package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestPodcastConfigDefaults(t *testing.T) {
	d := t.TempDir()
	cfg := loadPodcastConfig(d)
	if cfg.AdRemoval != AdRemovalAll {
		t.Errorf("expected default AdRemoval to be %q, got %q", AdRemovalAll, cfg.AdRemoval)
	}
	if adRemovalModeLabel(cfg.AdRemoval) != "Remove from all episodes" {
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

func TestPodcastConfigDownloadPolicyDefaults(t *testing.T) {
	d := t.TempDir()
	cfg := loadPodcastConfig(d)
	if cfg.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected default DownloadPolicy to be %q, got %q", DownloadPolicyNone, cfg.DownloadPolicy)
	}
	if cfg.DownloadK != 3 {
		t.Errorf("expected default DownloadK to be 3, got %d", cfg.DownloadK)
	}
	if downloadPolicyLabel(cfg.DownloadPolicy, cfg.DownloadK) != "No automatic downloads (none)" {
		t.Errorf("unexpected label: %s", downloadPolicyLabel(cfg.DownloadPolicy, cfg.DownloadK))
	}
	if downloadPolicyBadge(cfg.DownloadPolicy, cfg.DownloadK) != "[DL: None]" {
		t.Errorf("unexpected badge: %s", downloadPolicyBadge(cfg.DownloadPolicy, cfg.DownloadK))
	}
}

func TestPodcastConfigDownloadPolicyNormalization(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"none", DownloadPolicyNone},
		{"off", DownloadPolicyNone},
		{"disabled", DownloadPolicyNone},
		{"latest", DownloadPolicyLatest},
		{"newest", DownloadPolicyLatest},
		{"last", DownloadPolicyLatest},
		{"1", DownloadPolicyLatest},
		{"latest_k", DownloadPolicyLatestK},
		{"latest-k", DownloadPolicyLatestK},
		{"latestk", DownloadPolicyLatestK},
		{"more_k", DownloadPolicyMoreK},
		{"more-k", DownloadPolicyMoreK},
		{"more", DownloadPolicyMoreK},
		{"all", DownloadPolicyAll},
		{"full", DownloadPolicyAll},
		{"every", DownloadPolicyAll},
		{"unknown", DownloadPolicyNone},
	}

	for _, c := range cases {
		norm := normalizeDownloadPolicy(c.input)
		if norm != c.expected {
			t.Errorf("normalizeDownloadPolicy(%q) = %q, expected %q", c.input, norm, c.expected)
		}
	}
}

func TestPodcastConfigDownloadPolicyCycle(t *testing.T) {
	m := DownloadPolicyNone
	m = cycleDownloadPolicy(m)
	if m != DownloadPolicyLatest {
		t.Errorf("expected cycle none -> latest, got %s", m)
	}
	m = cycleDownloadPolicy(m)
	if m != DownloadPolicyLatestK {
		t.Errorf("expected cycle latest -> latest_k, got %s", m)
	}
	m = cycleDownloadPolicy(m)
	if m != DownloadPolicyMoreK {
		t.Errorf("expected cycle latest_k -> more_k, got %s", m)
	}
	m = cycleDownloadPolicy(m)
	if m != DownloadPolicyAll {
		t.Errorf("expected cycle more_k -> all, got %s", m)
	}
	m = cycleDownloadPolicy(m)
	if m != DownloadPolicyNone {
		t.Errorf("expected cycle all -> none, got %s", m)
	}
}

func TestPodcastConfigDownloadPolicyLabelsAndBadges(t *testing.T) {
	if downloadPolicyLabel(DownloadPolicyLatest, 5) != "Latest episode only (latest)" {
		t.Errorf("unexpected label for latest: %s", downloadPolicyLabel(DownloadPolicyLatest, 5))
	}
	if downloadPolicyLabel(DownloadPolicyLatestK, 5) != "Latest 5 episodes (latest_k)" {
		t.Errorf("unexpected label for latest_k: %s", downloadPolicyLabel(DownloadPolicyLatestK, 5))
	}
	if downloadPolicyLabel(DownloadPolicyMoreK, 4) != "Next 4 undownloaded (more_k)" {
		t.Errorf("unexpected label for more_k: %s", downloadPolicyLabel(DownloadPolicyMoreK, 4))
	}
	if downloadPolicyLabel(DownloadPolicyAll, 0) != "All episodes (all)" {
		t.Errorf("unexpected label for all: %s", downloadPolicyLabel(DownloadPolicyAll, 0))
	}

	if downloadPolicyBadge(DownloadPolicyLatest, 5) != "[DL: Latest]" {
		t.Errorf("unexpected badge for latest: %s", downloadPolicyBadge(DownloadPolicyLatest, 5))
	}
	if downloadPolicyBadge(DownloadPolicyLatestK, 5) != "[DL: Latest 5]" {
		t.Errorf("unexpected badge for latest_k: %s", downloadPolicyBadge(DownloadPolicyLatestK, 5))
	}
	if downloadPolicyBadge(DownloadPolicyMoreK, 4) != "[DL: More 4]" {
		t.Errorf("unexpected badge for more_k: %s", downloadPolicyBadge(DownloadPolicyMoreK, 4))
	}
	if downloadPolicyBadge(DownloadPolicyAll, 0) != "[DL: All]" {
		t.Errorf("unexpected badge for all: %s", downloadPolicyBadge(DownloadPolicyAll, 0))
	}
}

func TestPodcastConfigDownloadPolicySaveLoad(t *testing.T) {
	d := t.TempDir()

	cfg := PodcastConfig{
		AdRemoval:      AdRemovalLatest,
		DownloadPolicy: DownloadPolicyLatestK,
		DownloadK:      7,
	}
	if err := savePodcastConfig(d, cfg); err != nil {
		t.Fatalf("savePodcastConfig failed: %v", err)
	}

	loaded := loadPodcastConfig(d)
	if loaded.DownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected loaded DownloadPolicy %q, got %q", DownloadPolicyLatestK, loaded.DownloadPolicy)
	}
	if loaded.DownloadK != 7 {
		t.Errorf("expected loaded DownloadK 7, got %d", loaded.DownloadK)
	}

	cfg.DownloadPolicy = DownloadPolicyMoreK
	cfg.DownloadK = 2
	if err := savePodcastConfig(d, cfg); err != nil {
		t.Fatalf("savePodcastConfig update failed: %v", err)
	}

	loaded2 := loadPodcastConfig(d)
	if loaded2.DownloadPolicy != DownloadPolicyMoreK {
		t.Errorf("expected loaded DownloadPolicy %q, got %q", DownloadPolicyMoreK, loaded2.DownloadPolicy)
	}
	if loaded2.DownloadK != 2 {
		t.Errorf("expected loaded DownloadK 2, got %d", loaded2.DownloadK)
	}
}

func TestSelectEpisodesByDownloadPolicy(t *testing.T) {
	catalog := []FeedEpisode{
		{Title: "Ep 1", GUID: "g1", EnclosureURL: "http://example.com/1.mp3", PublishedAt: 1000},
		{Title: "Ep 2", GUID: "g2", EnclosureURL: "http://example.com/2.mp3", PublishedAt: 2000},
		{Title: "Ep 3", GUID: "g3", EnclosureURL: "http://example.com/3.mp3", PublishedAt: 3000},
		{Title: "Ep 4", GUID: "g4", EnclosureURL: "http://example.com/4.mp3", PublishedAt: 4000},
		{Title: "Ep 5", GUID: "g5", EnclosureURL: "http://example.com/5.mp3", PublishedAt: 5000},
	}

	downloaded := map[string]bool{
		"g1": true,
		"g3": true,
	}
	isDownloaded := func(ep FeedEpisode) bool {
		return downloaded[ep.GUID]
	}

	eps, _ := selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyNone, 0, false)
	if len(eps) != 0 {
		t.Errorf("expected 0 episodes for none, got %d", len(eps))
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyLatest, 0, false)
	if len(eps) != 1 || eps[0].GUID != "g5" {
		t.Errorf("expected latest ep g5, got %+v", eps)
	}

	downloaded["g5"] = true
	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyLatest, 0, false)
	if len(eps) != 1 || eps[0].GUID != "g4" {
		t.Errorf("expected next latest ep g4, got %+v", eps)
	}
	delete(downloaded, "g5")

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyLatestK, 2, false)
	if len(eps) != 2 || eps[0].GUID != "g5" || eps[1].GUID != "g4" {
		t.Errorf("expected latest_2 eps [g5, g4], got %+v", eps)
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyLatestK, 3, false)
	if len(eps) != 2 || eps[0].GUID != "g5" || eps[1].GUID != "g4" {
		t.Errorf("expected latest_3 eps [g5, g4], got %+v", eps)
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyMoreK, 2, false)
	if len(eps) != 2 || eps[0].GUID != "g5" || eps[1].GUID != "g4" {
		t.Errorf("expected more_2 eps [g5, g4], got %+v", eps)
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyMoreK, 3, false)
	if len(eps) != 3 || eps[0].GUID != "g5" || eps[1].GUID != "g4" || eps[2].GUID != "g2" {
		t.Errorf("expected more_3 eps [g5, g4, g2], got %+v", eps)
	}

	eps, _ = selectEpisodesByDownloadPolicy(catalog, isDownloaded, DownloadPolicyAll, 0, false)
	if len(eps) != 3 || eps[0].GUID != "g5" || eps[1].GUID != "g4" || eps[2].GUID != "g2" {
		t.Errorf("expected all eps [g5, g4, g2], got %+v", eps)
	}
}

func TestDeterministicBatchEpisodeSorting(t *testing.T) {
	d := t.TempDir()

	f1 := filepath.Join(d, "ep1.mp3")
	f2 := filepath.Join(d, "ep2.mp3")
	f3 := filepath.Join(d, "ep3.mp3")

	os.WriteFile(f1, []byte("audio1"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f2, []byte("audio2"), 0644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f3, []byte("audio3"), 0644)

	cachedIdx := &CachedPodcastIndex{
		PodcastName: "Test Podcast",
		PodcastDir:  d,
		Episodes: []CachedEpisodeSummary{
			{Filename: "ep1.mp3", Path: f1, PublishedAt: 100000},
			{Filename: "ep2.mp3", Path: f2, PublishedAt: 300000},
			{Filename: "ep3.mp3", Path: f3, PublishedAt: 200000},
		},
	}
	savePodcastCache(d, cachedIdx)

	files := []string{f1, f2, f3}
	sort.SliceStable(files, func(i, j int) bool {
		ti := getEpisodePublicationTime(files[i])
		tj := getEpisodePublicationTime(files[j])
		if ti.Equal(tj) {
			return files[i] < files[j]
		}
		return ti.After(tj)
	})

	if files[0] != f2 || files[1] != f3 || files[2] != f1 {
		t.Errorf("expected sorted order [ep2, ep3, ep1], got %+v", files)
	}
}
