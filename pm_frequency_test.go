package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyzePodcastFrequency_IntermittentFewEpisodes(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 5; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 2 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceIntermittent) {
		t.Errorf("expected %s, got %s", CadenceIntermittent, freq.Type)
	}
	if freq.EpisodesAnalyzed != 5 {
		t.Errorf("expected 5 episodes analyzed, got %d", freq.EpisodesAnalyzed)
	}
	if freq.AnalyzedAt.IsZero() {
		t.Error("expected non-zero AnalyzedAt")
	}
}

func TestAnalyzePodcastFrequency_Hourly(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Hourly Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceHourly) {
		t.Errorf("expected %s, got %s", CadenceHourly, freq.Type)
	}
	if freq.MedianHoursInterval != 1.0 {
		t.Errorf("expected median interval 1.0, got %f", freq.MedianHoursInterval)
	}
	if freq.EpisodesAnalyzed != 20 {
		t.Errorf("expected 20 episodes analyzed, got %d", freq.EpisodesAnalyzed)
	}
}

func TestAnalyzePodcastFrequency_HourlyThreeHours(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:   fmt.Sprintf("Ep %d", i),
			PubDate: base.Add(time.Duration(i*3) * time.Hour).Format(time.RFC1123),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceHourly) {
		t.Errorf("expected %s, got %s", CadenceHourly, freq.Type)
	}
	if freq.MedianHoursInterval != 3.0 {
		t.Errorf("expected median interval 3.0, got %f", freq.MedianHoursInterval)
	}
}

func TestAnalyzePodcastFrequency_Daily(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 14; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Daily Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 24 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceDaily) {
		t.Errorf("expected %s, got %s", CadenceDaily, freq.Type)
	}
	if freq.MedianHoursInterval != 24.0 {
		t.Errorf("expected median interval 24.0, got %f", freq.MedianHoursInterval)
	}
	if freq.EpisodesPerWeek < 6.9 || freq.EpisodesPerWeek > 7.1 {
		t.Errorf("expected ~7 eps/week, got %f", freq.EpisodesPerWeek)
	}
}

func TestAnalyzePodcastFrequency_DailyWeekdays(t *testing.T) {
	var episodes []FeedEpisode
	curr := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		for curr.Weekday() == time.Saturday || curr.Weekday() == time.Sunday {
			curr = curr.Add(24 * time.Hour)
		}
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Weekday Ep %d", i),
			PublishedAt: curr.UnixMilli(),
		})
		curr = curr.Add(24 * time.Hour)
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceDaily) {
		t.Errorf("expected %s for weekday show, got %s", CadenceDaily, freq.Type)
	}
}

func TestAnalyzePodcastFrequency_Weekly(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Weekly Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 7 * 24 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceWeekly) {
		t.Errorf("expected %s, got %s", CadenceWeekly, freq.Type)
	}
	if freq.MedianHoursInterval != 168.0 {
		t.Errorf("expected median interval 168.0, got %f", freq.MedianHoursInterval)
	}
	if freq.EpisodesPerWeek < 0.99 || freq.EpisodesPerWeek > 1.01 {
		t.Errorf("expected ~1 ep/week, got %f", freq.EpisodesPerWeek)
	}
}

func TestAnalyzePodcastFrequency_Monthly(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Monthly Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 30 * 24 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceMonthly) {
		t.Errorf("expected %s, got %s", CadenceMonthly, freq.Type)
	}
	if freq.MedianHoursInterval != 720.0 {
		t.Errorf("expected median interval 720.0, got %f", freq.MedianHoursInterval)
	}
}

func TestAnalyzePodcastFrequency_Intermittent(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Sporadic Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 60 * 24 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceIntermittent) {
		t.Errorf("expected %s, got %s", CadenceIntermittent, freq.Type)
	}
}

func TestAnalyzePodcastFrequency_ReverseChronologicalOrder(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 15; i >= 0; i-- {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Ep %d", i),
			PublishedAt: base.Add(time.Duration(i) * 24 * time.Hour).UnixMilli(),
		})
	}

	freq := analyzePodcastFrequency(episodes)
	if freq.Type != string(CadenceDaily) {
		t.Errorf("expected %s, got %s", CadenceDaily, freq.Type)
	}
	if freq.MedianHoursInterval != 24.0 {
		t.Errorf("expected median interval 24.0, got %f", freq.MedianHoursInterval)
	}
}

func TestAnalyzePodcastFrequency_EdgeCases(t *testing.T) {
	var zeroEpisodes []FeedEpisode
	for i := 0; i < 12; i++ {
		zeroEpisodes = append(zeroEpisodes, FeedEpisode{Title: fmt.Sprintf("Zero %d", i)})
	}
	zeroFreq := analyzePodcastFrequency(zeroEpisodes)
	if zeroFreq.Type != string(CadenceIntermittent) {
		t.Errorf("expected intermittent for 0 timestamps, got %s", zeroFreq.Type)
	}

	var sameEpisodes []FeedEpisode
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 12; i++ {
		sameEpisodes = append(sameEpisodes, FeedEpisode{Title: fmt.Sprintf("Same %d", i), PublishedAt: fixed})
	}
	sameFreq := analyzePodcastFrequency(sameEpisodes)
	if sameFreq.Type != string(CadenceHourly) {
		t.Errorf("expected hourly for same timestamps (median interval 0), got %s", sameFreq.Type)
	}
}

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
