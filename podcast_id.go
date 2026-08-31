package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type podcastDirEntry struct {
	dir        string
	folderName string
	title      string
	shortID    string
}

func generatePodcastShortID(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		h := sha256.Sum256([]byte("podcast"))
		return hex.EncodeToString(h[:])[:5]
	}

	var sb strings.Builder
	for _, r := range t {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			sb.WriteRune(r + ('a' - 'A'))
		}
	}
	filtered := sb.String()
	if len(filtered) >= 5 {
		return filtered[:5]
	}

	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])[:5]
}

func getOrSetPodcastShortID(podDir, title string) string {
	cfg := loadPodcastConfig(podDir)
	if id := strings.TrimSpace(cfg.ID); id != "" {
		return id
	}

	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" || cleanTitle == "." {
		if c, _ := loadPodcastCache(podDir); c != nil && strings.TrimSpace(c.PodcastName) != "" {
			cleanTitle = strings.TrimSpace(c.PodcastName)
		}
	}
	if cleanTitle == "" || cleanTitle == "." {
		cleanTitle = filepath.Base(podDir)
	}

	id := generatePodcastShortID(cleanTitle)
	cfg.ID = id
	_ = savePodcastConfig(podDir, cfg)
	return id
}

func scanPodcastDirs(podcastsDir string) []podcastDirEntry {
	if podcastsDir == "" {
		podcastsDir = "."
	}

	var entries []podcastDirEntry
	dirEntries, err := os.ReadDir(podcastsDir)
	if err == nil {
		for _, de := range dirEntries {
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") || de.Name() == ".work" {
				continue
			}
			podPath := filepath.Join(podcastsDir, de.Name())
			mp3s := findMP3Files(podPath)
			cfgPath := filepath.Join(podPath, podcastConfigFileName)
			_, errCfg := os.Stat(cfgPath)
			if len(mp3s) == 0 && errCfg != nil {
				continue
			}

			title := de.Name()
			if cached, _ := loadPodcastCache(podPath); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
				title = strings.TrimSpace(cached.PodcastName)
			}
			shortID := getOrSetPodcastShortID(podPath, title)
			entries = append(entries, podcastDirEntry{
				dir:        podPath,
				folderName: de.Name(),
				title:      title,
				shortID:    shortID,
			})
		}
	}

	if len(entries) == 0 {
		mp3s := findMP3Files(podcastsDir)
		if len(mp3s) > 0 {
			title := filepath.Base(podcastsDir)
			if cached, _ := loadPodcastCache(podcastsDir); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
				title = strings.TrimSpace(cached.PodcastName)
			}
			shortID := getOrSetPodcastShortID(podcastsDir, title)
			entries = append(entries, podcastDirEntry{
				dir:        podcastsDir,
				folderName: filepath.Base(podcastsDir),
				title:      title,
				shortID:    shortID,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].folderName) < strings.ToLower(entries[j].folderName)
	})

	return entries
}

func resolvePodcastDirByIDOrName(podcastsDir, query string) (string, string, bool) {
	search := strings.TrimSpace(query)
	if search == "" {
		return "", "", false
	}

	podcasts := scanPodcastDirs(podcastsDir)
	if len(podcasts) == 0 {
		return "", "", false
	}

	for _, p := range podcasts {
		if strings.EqualFold(p.shortID, search) {
			return p.dir, p.title, true
		}
	}

	if idx, err := strconv.Atoi(search); err == nil {
		if idx >= 1 && idx <= len(podcasts) {
			p := podcasts[idx-1]
			return p.dir, p.title, true
		}
	}

	for _, p := range podcasts {
		if strings.EqualFold(p.folderName, search) || strings.EqualFold(p.title, search) {
			return p.dir, p.title, true
		}
	}

	lower := strings.ToLower(search)
	for _, p := range podcasts {
		if strings.Contains(strings.ToLower(p.folderName), lower) || strings.Contains(strings.ToLower(p.title), lower) {
			return p.dir, p.title, true
		}
	}

	return "", "", false
}
