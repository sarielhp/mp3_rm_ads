package main

import (
	"fmt"
	"testing"
	"time"
)

func TestAnalyzePodcastFrequency_IntermittentFewEpisodes(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Now().Add(-100 * 24 * time.Hour)
	for i := 0; i < 5; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("Ep %d", i),
			PublishedAt: base.Add(time.Duration(i*14) * 24 * time.Hour).UnixMilli(),
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

func TestAnalyzePodcastFrequency_RollingHourly(t *testing.T) {
	var episodes []FeedEpisode
	base := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		episodes = append(episodes, FeedEpisode{
			Title:       fmt.Sprintf("NPR News Now Ep %d", i),
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
	if freq.EpisodesAnalyzed != 4 {
		t.Errorf("expected 4 episodes analyzed, got %d", freq.EpisodesAnalyzed)
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
