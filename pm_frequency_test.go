package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPodcastConfigFrequencySaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	cfg := PodcastConfig{
		AdRemoval:      AdRemovalAll,
		DownloadPolicy: DownloadPolicyLatest,
		DownloadK:      3,
		Frequency: &PodcastFrequencyInfo{
			Type:                string(CadenceDaily),
			EpisodesAnalyzed:    42,
			AvgDaysInterval:     1.2,
			MedianHoursInterval: 24.0,
			EpisodesPerWeek:     5.8,
			AnalyzedAt:          now,
		},
	}

	if err := savePodcastConfig(tmpDir, cfg); err != nil {
		t.Fatalf("savePodcastConfig failed: %v", err)
	}

	loaded := loadPodcastConfig(tmpDir)
	if loaded.Frequency == nil {
		t.Fatal("expected Frequency to be loaded, got nil")
	}
	if loaded.Frequency.Type != string(CadenceDaily) {
		t.Errorf("expected Frequency.Type %q, got %q", CadenceDaily, loaded.Frequency.Type)
	}
	if loaded.Frequency.EpisodesAnalyzed != 42 {
		t.Errorf("expected EpisodesAnalyzed 42, got %d", loaded.Frequency.EpisodesAnalyzed)
	}
	if loaded.Frequency.MedianHoursInterval != 24.0 {
		t.Errorf("expected MedianHoursInterval 24.0, got %f", loaded.Frequency.MedianHoursInterval)
	}
	if loaded.Frequency.EpisodesPerWeek != 5.8 {
		t.Errorf("expected EpisodesPerWeek 5.8, got %f", loaded.Frequency.EpisodesPerWeek)
	}
}

func TestDisableHourlyPolicyOnConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := PodcastConfig{
		AdRemoval:      AdRemovalAll,
		DownloadPolicy: DownloadPolicyLatestK,
		DownloadK:      5,
		Frequency: &PodcastFrequencyInfo{
			Type: string(CadenceHourly),
		},
	}
	_ = savePodcastConfig(tmpDir, cfg)

	loaded := loadPodcastConfig(tmpDir)
	if loaded.Frequency.Type == string(CadenceHourly) {
		loaded.DownloadPolicy = DownloadPolicyNone
		loaded.AdRemoval = AdRemovalNone
		_ = savePodcastConfig(tmpDir, loaded)
	}

	updated := loadPodcastConfig(tmpDir)
	if updated.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected download_policy %q, got %q", DownloadPolicyNone, updated.DownloadPolicy)
	}
	if updated.AdRemoval != AdRemovalNone {
		t.Errorf("expected ad_removal %q, got %q", AdRemovalNone, updated.AdRemoval)
	}
}

func TestLoadCachedFeedEpisodes(t *testing.T) {
	tmpDir := t.TempDir()
	idx := &CachedPodcastIndex{
		PodcastName: "Test Show",
		PodcastDir:  tmpDir,
		Episodes: []CachedEpisodeSummary{
			{Title: "Ep 1", PublishedAt: 1700000000000},
			{Title: "Ep 2", PublishedAt: 1700086400000},
		},
	}
	if err := savePodcastCache(tmpDir, idx); err != nil {
		t.Fatalf("savePodcastCache failed: %v", err)
	}

	eps := loadCachedFeedEpisodes(tmpDir)
	if len(eps) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(eps))
	}
	if eps[0].Title != "Ep 1" || eps[0].PublishedAt != 1700000000000 {
		t.Errorf("unexpected episode 0: %+v", eps[0])
	}
}

func TestHandleServerFrequency_LocalDirectory(t *testing.T) {
	rootTmp := t.TempDir()

	hourlyDir := filepath.Join(rootTmp, "Hourly Podcast")
	_ = os.MkdirAll(hourlyDir, 0755)
	_ = savePodcastConfig(hourlyDir, PodcastConfig{
		AdRemoval:      AdRemovalAll,
		DownloadPolicy: DownloadPolicyLatest,
		DownloadK:      3,
	})

	var hourlyEpisodes []CachedEpisodeSummary
	baseH := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		hourlyEpisodes = append(hourlyEpisodes, CachedEpisodeSummary{
			Title:       fmt.Sprintf("H %d", i),
			PublishedAt: baseH.Add(time.Duration(i) * time.Hour).UnixMilli(),
		})
	}
	_ = savePodcastCache(hourlyDir, &CachedPodcastIndex{
		PodcastName: "Hourly Podcast",
		PodcastDir:  hourlyDir,
		Episodes:    hourlyEpisodes,
	})

	dailyDir := filepath.Join(rootTmp, "Daily Podcast")
	_ = os.MkdirAll(dailyDir, 0755)
	_ = savePodcastConfig(dailyDir, PodcastConfig{
		AdRemoval:      AdRemovalLatest,
		DownloadPolicy: DownloadPolicyLatestK,
		DownloadK:      5,
	})

	var dailyEpisodes []CachedEpisodeSummary
	baseD := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		dailyEpisodes = append(dailyEpisodes, CachedEpisodeSummary{
			Title:       fmt.Sprintf("D %d", i),
			PublishedAt: baseD.Add(time.Duration(i) * 24 * time.Hour).UnixMilli(),
		})
	}
	_ = savePodcastCache(dailyDir, &CachedPodcastIndex{
		PodcastName: "Daily Podcast",
		PodcastDir:  dailyDir,
		Episodes:    dailyEpisodes,
	})

	config := Config{PodcastsDir: rootTmp}
	cli := CLIOptions{
		ServerSubcmd:  "disable_hourly",
		DisableHourly: true,
		Quiet:         true,
	}

	handleServerFrequency(config, cli)

	hourlyCfg := loadPodcastConfig(hourlyDir)
	if hourlyCfg.Frequency == nil {
		t.Fatal("expected hourly frequency to be set")
	}
	if hourlyCfg.Frequency.Type != string(CadenceHourly) {
		t.Errorf("expected cadence hourly, got %s", hourlyCfg.Frequency.Type)
	}
	if hourlyCfg.DownloadPolicy != DownloadPolicyNone {
		t.Errorf("expected download_policy none, got %s", hourlyCfg.DownloadPolicy)
	}
	if hourlyCfg.AdRemoval != AdRemovalNone {
		t.Errorf("expected ad_removal none, got %s", hourlyCfg.AdRemoval)
	}

	dailyCfg := loadPodcastConfig(dailyDir)
	if dailyCfg.Frequency == nil {
		t.Fatal("expected daily frequency to be set")
	}
	if dailyCfg.Frequency.Type != string(CadenceDaily) {
		t.Errorf("expected cadence daily, got %s", dailyCfg.Frequency.Type)
	}
	if dailyCfg.DownloadPolicy != DownloadPolicyLatestK {
		t.Errorf("expected download_policy latest_k unchanged, got %s", dailyCfg.DownloadPolicy)
	}
	if dailyCfg.AdRemoval != AdRemovalLatest {
		t.Errorf("expected ad_removal latest unchanged, got %s", dailyCfg.AdRemoval)
	}
}
