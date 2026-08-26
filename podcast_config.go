package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	podcastConfigFileName = "podcast.json"

	AdRemovalNone   = "none"
	AdRemovalLatest = "latest"
	AdRemovalAll    = "all"
)

type PodcastConfig struct {
	AdRemoval string    `json:"ad_removal"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func defaultPodcastConfig() PodcastConfig {
	return PodcastConfig{
		AdRemoval: AdRemovalNone,
	}
}

func normalizeAdRemovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "latest", "last", "recent", "newest":
		return AdRemovalLatest
	case "all", "every", "full":
		return AdRemovalAll
	default:
		return AdRemovalNone
	}
}

func cycleAdRemovalMode(current string) string {
	switch normalizeAdRemovalMode(current) {
	case AdRemovalNone:
		return AdRemovalLatest
	case AdRemovalLatest:
		return AdRemovalAll
	case AdRemovalAll:
		return AdRemovalNone
	default:
		return AdRemovalNone
	}
}

func adRemovalModeLabel(mode string) string {
	switch normalizeAdRemovalMode(mode) {
	case AdRemovalLatest:
		return "Remove from latest episode"
	case AdRemovalAll:
		return "Remove from all episodes"
	default:
		return "No ad removal"
	}
}

func adRemovalModeBadge(mode string) string {
	switch normalizeAdRemovalMode(mode) {
	case AdRemovalLatest:
		return "[Ads: Latest]"
	case AdRemovalAll:
		return "[Ads: All]"
	default:
		return "[Ads: None]"
	}
}

func loadPodcastConfig(dir string) PodcastConfig {
	cfgPath := filepath.Join(dir, podcastConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return defaultPodcastConfig()
	}
	var cfg PodcastConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultPodcastConfig()
	}
	if cfg.AdRemoval == "" {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			if v, ok := raw["ad_removal_mode"].(string); ok {
				cfg.AdRemoval = v
			} else if v, ok := raw["status"].(string); ok {
				cfg.AdRemoval = v
			}
		}
	}
	cfg.AdRemoval = normalizeAdRemovalMode(cfg.AdRemoval)
	return cfg
}

func savePodcastConfig(dir string, cfg PodcastConfig) error {
	cfg.AdRemoval = normalizeAdRemovalMode(cfg.AdRemoval)
	cfg.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, podcastConfigFileName)
	return os.WriteFile(cfgPath, append(data, '\n'), 0644)
}

func filterMP3FilesByPodcastConfig(files []string, dir string, cfg PodcastConfig) []string {
	if len(files) == 0 {
		return files
	}

	mode := normalizeAdRemovalMode(cfg.AdRemoval)
	if mode == AdRemovalNone {
		return nil
	}

	if mode == AdRemovalAll {
		return files
	}

	type fileWithTime struct {
		path    string
		modTime time.Time
	}
	var list []fileWithTime
	for _, f := range files {
		var mt time.Time
		if fi, err := os.Stat(f); err == nil {
			mt = fi.ModTime()
		}
		list = append(list, fileWithTime{path: f, modTime: mt})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].modTime.After(list[j].modTime)
	})

	for _, item := range list {
		base := strings.TrimSuffix(item.path, ".mp3")
		if _, err := os.Stat(base + ".cuts.json"); err != nil {
			return []string{item.path}
		}
	}

	return []string{list[0].path}
}
