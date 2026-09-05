package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type tuiPodcast struct {
	name        string
	dir         string
	author      string
	description string
	feedURL     string
	coverPath   string
	episodes    []tuiEpisode
	absData     *absItem
	config      PodcastConfig
}

func (p tuiPodcast) transcribedCount() int {
	c := 0
	for _, e := range p.episodes {
		if e.hasTranscript {
			c++
		}
	}
	return c
}

func (p tuiPodcast) displayAuthor() string {
	if p.absData != nil && p.absData.Media.Metadata.Author != "" {
		return p.absData.Media.Metadata.Author
	}
	return p.author
}

func (p tuiPodcast) displayDescription() string {
	if p.absData != nil && p.absData.Media.Metadata.Description != "" {
		return p.absData.Media.Metadata.Description
	}
	return p.description
}

type tuiEpisode struct {
	filename      string
	path          string
	title         string
	hasAdsRemoved bool
	hasTranscript bool
	fileSize      int64
	modTime       time.Time
	publishedAt   int64
	duration      float64
	durationDone  bool
	season        string
	episode       string
	absData       *absEpisode
	isFeedOnly    bool
	enclosureURL  string
	guid          string
	description   string
}

func (e tuiEpisode) displayDate() time.Time {
	if e.publishedAt > 0 {
		return time.UnixMilli(e.publishedAt)
	}
	if e.absData != nil {
		if pub := parseABSEpisodePublishedAt(e.absData); pub > 0 {
			return time.UnixMilli(pub)
		}
	}
	return e.modTime
}

func (e tuiEpisode) displayTitle() string {
	if e.title != "" {
		return e.title
	}
	if e.absData != nil && e.absData.Title != "" {
		return e.absData.Title
	}
	if strings.EqualFold(e.filename, "podcast.mp3") && e.path != "" {
		return filepath.Base(filepath.Dir(e.path))
	}
	return e.filename
}

func (e tuiEpisode) displayEpisodeNum(index int) string {
	if e.episode != "" {
		return "#" + e.episode
	}
	if e.absData != nil && e.absData.Episode != "" {
		return "#" + e.absData.Episode
	}
	if index > 0 {
		return fmt.Sprintf("#%d", index)
	}
	return ""
}

type tuiScreen int

const (
	screenPodcasts tuiScreen = iota
	screenPodcastDetail
	screenEpisodeDetail
	screenPlayer
	screenPlayQueue
	screenAdQueue
	screenDownloadQueue
	screenLatestEpisodes
	screenTranscript
	screenTimeline
)

type TuiBackend struct {
	LoadPodcasts func(dir string) ([]tuiPodcast, error)
	LoadQueues   func(pods []tuiPodcast) map[string][]string
	SaveQueue    func(dir string, entries []string)
	GetDuration  func(path string) float64
}
