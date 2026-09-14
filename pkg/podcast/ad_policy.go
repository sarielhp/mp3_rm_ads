package podcast

import (
	"os"
	"pod/pkg/config"
	"sort"
	"strings"
	"time"
)

// FilterByAdRemovalPolicy narrows a podcast's episodes to those its ad-removal
// policy says should be processed now.
func FilterByAdRemovalPolicy(files []string, dir string, cfg config.PodcastConfig) []string {
	if len(files) == 0 {
		return files
	}
	mode := config.NormalizeAdRemovalMode(cfg.AdRemoval)
	if mode == config.AdRemovalNone {
		return nil
	}
	if mode == config.AdRemovalAll {
		return files
	}
	type fileWithTime struct {
		path    string
		pubTime time.Time
	}
	var list []fileWithTime
	for _, f := range files {
		pt := GetEpisodePublicationTime(f)
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

// SanitizeTitle turns an episode or podcast title into a safe filename stem.
func SanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled Podcast"
	}
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, c := range badChars {
		title = strings.ReplaceAll(title, c, "_")
	}
	title = strings.TrimSpace(title)
	if title == "" || title == ".." || title == "." || strings.Trim(title, ".") == "" || strings.HasPrefix(title, "../") || strings.HasPrefix(title, ".._") {
		return "Untitled Podcast"
	}
	return title
}
