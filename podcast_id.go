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

	fields := strings.FieldsFunc(t, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == ':' || r == ',' || r == '.' || r == '\'' || r == '"'
	})

	var cleanWords []string
	for _, f := range fields {
		var b strings.Builder
		for _, r := range f {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			} else if r >= 'A' && r <= 'Z' {
				b.WriteRune(r + ('a' - 'A'))
			}
		}
		if b.Len() > 0 {
			cleanWords = append(cleanWords, b.String())
		}
	}

	if len(cleanWords) >= 3 {
		var initials strings.Builder
		for _, w := range cleanWords {
			initials.WriteByte(w[0])
		}
		if initials.Len() >= 5 {
			return initials.String()[:5]
		}
		var b strings.Builder
		for _, w := range cleanWords {
			b.WriteByte(w[0])
			for i := 1; i < len(w); i++ {
				c := w[i]
				if c != 'a' && c != 'e' && c != 'i' && c != 'o' && c != 'u' {
					b.WriteByte(c)
					if b.Len() == 5 {
						return b.String()
					}
				}
			}
		}
		if b.Len() >= 5 {
			return b.String()[:5]
		}
	} else if len(cleanWords) == 2 {
		w1, w2 := cleanWords[0], cleanWords[1]
		var b strings.Builder
		b.WriteByte(w1[0])
		for i := 1; i < len(w1); i++ {
			if w1[i] != 'a' && w1[i] != 'e' && w1[i] != 'i' && w1[i] != 'o' && w1[i] != 'u' {
				b.WriteByte(w1[i])
			}
		}
		b.WriteByte(w2[0])
		for i := 1; i < len(w2); i++ {
			if w2[i] != 'a' && w2[i] != 'e' && w2[i] != 'i' && w2[i] != 'o' && w2[i] != 'u' {
				b.WriteByte(w2[i])
			}
		}
		if b.Len() >= 5 {
			return b.String()[:5]
		}
		comb := w1 + w2
		if len(comb) >= 5 {
			return comb[:5]
		}
	} else if len(cleanWords) == 1 {
		w := cleanWords[0]
		var b strings.Builder
		b.WriteByte(w[0])
		for i := 1; i < len(w); i++ {
			if w[i] != 'a' && w[i] != 'e' && w[i] != 'i' && w[i] != 'o' && w[i] != 'u' {
				b.WriteByte(w[i])
			}
		}
		if b.Len() >= 5 {
			return b.String()[:5]
		}
		if len(w) >= 5 {
			return w[:5]
		}
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
			entries = append(entries, podcastDirEntry{
				dir:        podPath,
				folderName: de.Name(),
				title:      title,
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
			entries = append(entries, podcastDirEntry{
				dir:        podcastsDir,
				folderName: filepath.Base(podcastsDir),
				title:      title,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].folderName) < strings.ToLower(entries[j].folderName)
	})

	seenIDs := make(map[string]string)
	for i := range entries {
		cfg := loadPodcastConfig(entries[i].dir)
		candID := strings.TrimSpace(cfg.ID)
		if candID == "" || seenIDs[candID] != "" {
			candID = generatePodcastShortID(entries[i].title)
		}
		if seenIDs[candID] != "" {
			h := sha256.Sum256([]byte(entries[i].title))
			hexStr := hex.EncodeToString(h[:])
			for off := 0; off+5 <= len(hexStr); off++ {
				slice := hexStr[off : off+5]
				if seenIDs[slice] == "" {
					candID = slice
					break
				}
			}
		}
		seenIDs[candID] = entries[i].dir
		entries[i].shortID = candID
		if cfg.ID != candID {
			cfg.ID = candID
			_ = savePodcastConfig(entries[i].dir, cfg)
		}
	}

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
