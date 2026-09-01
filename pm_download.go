package main

import (
	"os"
	"path/filepath"
	"strings"
)

func findPodcastDirForItem(item PodcastItem, podcastsDir string) string {
	if podcastsDir == "" {
		cfg := loadConfig()
		podcastsDir = cfg.PodcastsDir
	}
	if podcastsDir == "" {
		return ""
	}

	title := item.Media.Metadata.Title
	if title != "" {
		p := filepath.Join(podcastsDir, title)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
		safeName := strings.Map(func(r rune) rune {
			if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
				return '_'
			}
			return r
		}, title)
		safeName = strings.TrimSpace(safeName)
		if safeName != "" {
			p := filepath.Join(podcastsDir, safeName)
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return p
			}
		}
	}

	entries, err := os.ReadDir(podcastsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(podcastsDir, e.Name())
			if cached, _ := loadPodcastCache(dir); cached != nil {
				if cached.ABSItemID == item.ID || cached.ABSItemID == item.Media.ID || (cached.FeedURL != "" && cached.FeedURL == item.Media.Metadata.FeedURL) {
					return dir
				}
			}
			if title != "" && strings.EqualFold(e.Name(), title) {
				return dir
			}
		}
	}

	return ""
}
