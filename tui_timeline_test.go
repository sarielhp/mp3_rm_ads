package main

import (
	"strings"
	"testing"
	"time"
)

func TestGetPodcastLastEpisodesOnlineTimeline(t *testing.T) {
	t1 := time.Date(2026, 8, 25, 14, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 18, 14, 30, 0, 0, time.UTC)

	pod := tuiPodcast{
		name: "Test Show",
		episodes: []tuiEpisode{
			{
				filename:      "ep2.mp3",
				title:         "Episode Two",
				publishedAt:   t2.UnixMilli(),
				duration:      1800,
				hasAdsRemoved: false,
				hasTranscript: true,
			},
			{
				filename:      "ep1.mp3",
				title:         "Episode One",
				publishedAt:   t1.UnixMilli(),
				duration:      2400,
				hasAdsRemoved: true,
				hasTranscript: true,
			},
		},
	}

	releases := getPodcastLastEpisodesOnlineTimeline(pod, 20)
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}

	// Should be sorted newest first
	if releases[0].Title != "Episode One" {
		t.Errorf("expected newest episode first, got %q", releases[0].Title)
	}
	if !strings.Contains(releases[0].ExactTimeLocal, "2026-08-25") {
		t.Errorf("unexpected local time: %s", releases[0].ExactTimeLocal)
	}
	if !releases[0].HasAdsRemoved {
		t.Error("expected HasAdsRemoved to be true for episode one")
	}
	if !releases[0].HasTranscript {
		t.Error("expected HasTranscript to be true for episode one")
	}

	table := formatEpisodesTimelineTable(releases, pod.name, 120)
	if !strings.Contains(table, "Episode One") || !strings.Contains(table, "Episode Two") {
		t.Error("table should contain both episode titles")
	}
	if !strings.Contains(table, "2026-08-25") {
		t.Error("table should contain exact timestamp")
	}
}

func TestGetPodcastLastEpisodesOnlineTimelineLimit(t *testing.T) {
	var eps []tuiEpisode
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		eps = append(eps, tuiEpisode{
			filename:    "ep.mp3",
			title:       "Ep",
			publishedAt: base.Add(time.Duration(i) * 24 * time.Hour).UnixMilli(),
		})
	}

	pod := tuiPodcast{
		name:     "Long Show",
		episodes: eps,
	}

	releases := getPodcastLastEpisodesOnlineTimeline(pod, 20)
	if len(releases) != 20 {
		t.Fatalf("expected 20 releases, got %d", len(releases))
	}
}
