package main

import (
	"encoding/json"
	"fmt"
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

	DownloadPolicyNone    = "none"
	DownloadPolicyLatest  = "latest"
	DownloadPolicyLatestK = "latest_k"
	DownloadPolicyAll     = "all"
)

type PodcastConfig struct {
	ID             string                `json:"id,omitempty"`
	AdRemoval      string                `json:"ad_removal"`
	DownloadPolicy string                `json:"download_policy,omitempty"`
	DownloadK      int                   `json:"download_k,omitempty"`
	Frequency      *PodcastFrequencyInfo `json:"frequency,omitempty"`
	UpdatedAt      time.Time             `json:"updated_at,omitempty"`
}

func defaultPodcastConfig() PodcastConfig {
	cfg := loadConfig()
	dlPolicy := cfg.DefaultDownloadPolicy
	if dlPolicy == "" {
		dlPolicy = DownloadPolicyLatest
	}
	dlK := cfg.DefaultDownloadK
	if dlK <= 0 {
		dlK = 3
	}
	adPolicy := cfg.DefaultAdRemoval
	if adPolicy == "" {
		adPolicy = AdRemovalAll
	}
	return PodcastConfig{
		AdRemoval:      normalizeAdRemovalMode(adPolicy),
		DownloadPolicy: normalizeDownloadPolicy(dlPolicy),
		DownloadK:      dlK,
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

func normalizeDownloadPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "latest", "last", "recent", "newest", "1", "single":
		return DownloadPolicyLatest
	case "latest_k", "latest-k", "latestk", "last_k", "last-k", "recent_k", "more_k", "more-k", "morek", "next_k", "next-k", "more":
		return DownloadPolicyLatestK
	case "all", "every", "full":
		return DownloadPolicyAll
	case "none", "off", "disabled", "no", "manual":
		return DownloadPolicyNone
	default:
		return DownloadPolicyNone
	}
}

func cycleDownloadPolicy(current string) string {
	switch normalizeDownloadPolicy(current) {
	case DownloadPolicyNone:
		return DownloadPolicyLatest
	case DownloadPolicyLatest:
		return DownloadPolicyLatestK
	case DownloadPolicyLatestK:
		return DownloadPolicyAll
	case DownloadPolicyAll:
		return DownloadPolicyNone
	default:
		return DownloadPolicyNone
	}
}

func downloadPolicyLabel(policy string, k int) string {
	if k <= 0 {
		k = 3
	}
	switch normalizeDownloadPolicy(policy) {
	case DownloadPolicyLatest:
		return "Latest episode only (latest)"
	case DownloadPolicyLatestK:
		return fmt.Sprintf("Latest %d episodes (latest_k)", k)
	case DownloadPolicyAll:
		return "All episodes (all)"
	default:
		return "No automatic downloads (none)"
	}
}

func downloadPolicyBadge(policy string, k int) string {
	if k <= 0 {
		k = 3
	}
	switch normalizeDownloadPolicy(policy) {
	case DownloadPolicyLatest:
		return "[DL: Latest]"
	case DownloadPolicyLatestK:
		return fmt.Sprintf("[DL: Latest %d]", k)
	case DownloadPolicyAll:
		return "[DL: All]"
	default:
		return "[DL: None]"
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
	def := defaultPodcastConfig()
	if cfg.AdRemoval == "" || cfg.DownloadPolicy == "" || cfg.DownloadK <= 0 || cfg.ID == "" {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			if cfg.ID == "" {
				if v, ok := raw["id"].(string); ok {
					cfg.ID = v
				} else if v, ok := raw["podcast_id"].(string); ok {
					cfg.ID = v
				}
			}
			if cfg.AdRemoval == "" {
				if v, ok := raw["ad_removal_mode"].(string); ok {
					cfg.AdRemoval = v
				} else if v, ok := raw["status"].(string); ok {
					cfg.AdRemoval = v
				}
			}
			if cfg.DownloadPolicy == "" {
				if v, ok := raw["download_policy"].(string); ok {
					cfg.DownloadPolicy = v
				} else if v, ok := raw["download_mode"].(string); ok {
					cfg.DownloadPolicy = v
				} else if v, ok := raw["policy"].(string); ok {
					cfg.DownloadPolicy = v
				}
			}
			if cfg.DownloadK <= 0 {
				if v, ok := raw["download_k"].(float64); ok && v > 0 {
					cfg.DownloadK = int(v)
				}
			}
		}
	}
	if cfg.AdRemoval == "" {
		cfg.AdRemoval = def.AdRemoval
	} else {
		cfg.AdRemoval = normalizeAdRemovalMode(cfg.AdRemoval)
	}
	if cfg.DownloadPolicy == "" {
		cfg.DownloadPolicy = def.DownloadPolicy
	} else {
		cfg.DownloadPolicy = normalizeDownloadPolicy(cfg.DownloadPolicy)
	}
	if cfg.DownloadK <= 0 {
		cfg.DownloadK = def.DownloadK
	}
	return cfg
}

func savePodcastConfig(dir string, cfg PodcastConfig) error {
	cfg.AdRemoval = normalizeAdRemovalMode(cfg.AdRemoval)
	cfg.DownloadPolicy = normalizeDownloadPolicy(cfg.DownloadPolicy)
	if cfg.DownloadK <= 0 {
		cfg.DownloadK = 3
	}
	cfg.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, podcastConfigFileName)
	return os.WriteFile(cfgPath, append(data, '\n'), 0644)
}

func getEpisodePublicationTime(filePath string) time.Time {
	dir := filepath.Dir(filePath)
	fn := filepath.Base(filePath)
	if cached, _ := loadPodcastCache(dir); cached != nil {
		for _, ep := range cached.Episodes {
			if (ep.Filename == fn || ep.Path == filePath) && ep.PublishedAt > 0 {
				return time.UnixMilli(ep.PublishedAt)
			}
		}
	}
	statPath := statusPathFor(filePath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil && st.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, st.PublishedAt); err == nil && !t.IsZero() {
			return t
		}
	}
	if fi, err := os.Stat(filePath); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
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
		pubTime time.Time
	}
	var list []fileWithTime
	for _, f := range files {
		pt := getEpisodePublicationTime(f)
		list = append(list, fileWithTime{path: f, pubTime: pt})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].pubTime.Equal(list[j].pubTime) {
			return list[i].path < list[j].path
		}
		return list[i].pubTime.After(list[j].pubTime)
	})

	for _, item := range list {
		base := strings.TrimSuffix(item.path, ".mp3")
		if _, err := os.Stat(base + ".cuts.json"); err != nil {
			return []string{item.path}
		}
	}

	return []string{list[0].path}
}
