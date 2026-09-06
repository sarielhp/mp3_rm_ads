package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ResolvedType int

const (
	ResolvedTypeNone ResolvedType = iota
	ResolvedTypePodcast
	ResolvedTypeEpisode
)

type ResolvedPodcast struct {
	Dir        string        `json:"dir"`
	Title      string        `json:"title"`
	ShortID    string        `json:"short_id"`
	FolderName string        `json:"folder_name"`
	UUID       string        `json:"uuid,omitempty"`
	Config     PodcastConfig `json:"config"`
}

type ResolvedEpisode struct {
	Path           string             `json:"path"`
	Filename       string             `json:"filename"`
	Title          string             `json:"title"`
	ShortID        string             `json:"short_id"`
	PodcastDir     string             `json:"podcast_dir"`
	PodcastTitle   string             `json:"podcast_title"`
	PodcastShortID string             `json:"podcast_short_id"`
	Status         *EpisodeStatusFile `json:"status,omitempty"`
}

type ResolvedID struct {
	Type    ResolvedType     `json:"type"`
	Podcast *ResolvedPodcast `json:"podcast,omitempty"`
	Episode *ResolvedEpisode `json:"episode,omitempty"`
}

func (r *ResolvedID) IsPodcast() bool {
	return r != nil && r.Type == ResolvedTypePodcast && r.Podcast != nil
}

func (r *ResolvedID) IsEpisode() bool {
	return r != nil && r.Type == ResolvedTypeEpisode && r.Episode != nil
}

func episodeTitleFromPath(audioPath string) string {
	base := filepath.Base(audioPath)
	stem := stripExt(base)
	if strings.EqualFold(stem, "podcast") {
		parent := filepath.Base(filepath.Dir(audioPath))
		if parent != "." && parent != "/" && parent != "" {
			return parent
		}
	}
	return stem
}

func detectPodcastDirForAudio(audioPath string) string {
	dir := filepath.Dir(audioPath)
	base := filepath.Base(audioPath)
	stem := stripExt(base)
	if strings.EqualFold(stem, "podcast") {
		parent := filepath.Dir(dir)
		if fi, err := os.Stat(parent); err == nil && fi.IsDir() {
			return parent
		}
	}
	return dir
}

func episodeUniqueKey(podDir, audioPath string) string {
	if podDir != "" {
		if rel, err := filepath.Rel(podDir, audioPath); err == nil && rel != "." && rel != "" {
			return filepath.ToSlash(rel)
		}
	}
	title := episodeTitleFromPath(audioPath)
	if title != "" && title != "podcast" {
		return title
	}
	return filepath.Base(audioPath)
}

func generateEpisodeShortID(podShortID, epKey string) string {
	cleanKey := strings.TrimSpace(epKey)
	cleanPod := strings.ToLower(strings.TrimSpace(podShortID))
	h := sha256.Sum256([]byte(cleanPod + ":" + cleanKey))
	return "e" + hex.EncodeToString(h[:])[:5]
}

func getOrSetEpisodeShortID(podDir, podShortID, audioPath string) string {
	if podDir == "" {
		podDir = detectPodcastDirForAudio(audioPath)
	}
	if podShortID == "" {
		podShortID = getOrSetPodcastShortID(podDir, filepath.Base(podDir))
	}

	bogusID := generateEpisodeShortID(podShortID, "podcast")
	key := episodeUniqueKey(podDir, audioPath)

	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		id := strings.TrimSpace(st.ID)
		if id != "" && (id != bogusID || key == "podcast") {
			return id
		}
	}

	altStatPath := stripExt(audioPath) + ".json"
	if altStatPath != statPath {
		if st, err := loadEpisodeStatus(altStatPath); err == nil && st != nil {
			id := strings.TrimSpace(st.ID)
			if id != "" && (id != bogusID || key == "podcast") {
				return id
			}
		}
	}

	if cached, _ := loadPodcastCache(podDir); cached != nil {
		baseFn := filepath.Base(audioPath)
		for _, ep := range cached.Episodes {
			if (ep.Path == audioPath || ep.Filename == baseFn) && strings.TrimSpace(ep.ID) != "" {
				id := strings.TrimSpace(ep.ID)
				if id != bogusID || key == "podcast" {
					return id
				}
			}
		}
	}

	id := generateEpisodeShortID(podShortID, key)

	st := getOrCreateEpisodeStatus(audioPath)
	st.ID = id
	_ = saveEpisodeStatus(statPath, st)

	return id
}

func resolveAnyID(podcastsDir, query string) (*ResolvedID, error) {
	search := strings.TrimSpace(query)
	if search == "" {
		return nil, fmt.Errorf("empty query")
	}

	if podcastsDir == "" {
		podcastsDir = "."
	}

	if res, ok := resolveDirectPath(podcastsDir, search); ok {
		return res, nil
	}

	podEntries := scanPodcastDirs(podcastsDir)
	if len(podEntries) == 0 {
		return nil, fmt.Errorf("no podcasts found in %s", podcastsDir)
	}

	if res, ok := resolveByEpisodeShortID(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveByPodcastShortID(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveByIndex(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveByPodcastUUID(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveByPodcastName(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveByEpisodeFileOrTitle(podEntries, search); ok {
		return res, nil
	}

	if res, ok := resolveBySubstring(podEntries, search); ok {
		return res, nil
	}

	return nil, fmt.Errorf("identifier %q not found in %s", query, podcastsDir)
}

func resolveDirectPath(podcastsDir, search string) (*ResolvedID, bool) {
	candidates := []string{search}
	if !filepath.IsAbs(search) {
		candidates = append(candidates, filepath.Join(podcastsDir, search))
	}

	for _, cand := range candidates {
		fi, err := os.Stat(cand)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			title := filepath.Base(cand)
			if cached, _ := loadPodcastCache(cand); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
				title = strings.TrimSpace(cached.PodcastName)
			}
			shortID := getOrSetPodcastShortID(cand, title)
			cfg := loadPodcastConfig(cand)
			return &ResolvedID{
				Type: ResolvedTypePodcast,
				Podcast: &ResolvedPodcast{
					Dir:        cand,
					Title:      title,
					ShortID:    shortID,
					FolderName: filepath.Base(cand),
					UUID:       cfg.ID,
					Config:     cfg,
				},
			}, true
		}

		if strings.HasSuffix(strings.ToLower(cand), ".mp3") {
			return buildResolvedEpisodeFromPath(cand), true
		}
	}

	return nil, false
}

func buildResolvedEpisodeFromPath(audioPath string) *ResolvedID {
	podDir := detectPodcastDirForAudio(audioPath)
	podTitle := filepath.Base(podDir)
	if cached, _ := loadPodcastCache(podDir); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
		podTitle = strings.TrimSpace(cached.PodcastName)
	}
	podShortID := getOrSetPodcastShortID(podDir, podTitle)
	epShortID := getOrSetEpisodeShortID(podDir, podShortID, audioPath)
	epTitle := episodeTitleFromPath(audioPath)
	st := getOrCreateEpisodeStatus(audioPath)

	return &ResolvedID{
		Type: ResolvedTypeEpisode,
		Episode: &ResolvedEpisode{
			Path:           audioPath,
			Filename:       filepath.Base(audioPath),
			Title:          epTitle,
			ShortID:        epShortID,
			PodcastDir:     podDir,
			PodcastTitle:   podTitle,
			PodcastShortID: podShortID,
			Status:         st,
		},
	}
}

func resolveByEpisodeShortID(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	if len(search) != 6 || !strings.HasPrefix(strings.ToLower(search), "e") {
		return nil, false
	}
	for _, p := range podEntries {
		mp3s := findMP3Files(p.dir)
		for _, mp3 := range mp3s {
			epID := getOrSetEpisodeShortID(p.dir, p.shortID, mp3)
			if strings.EqualFold(epID, search) {
				return buildResolvedEpisodeFromParams(p, mp3, epID), true
			}
		}
	}
	return nil, false
}

func buildResolvedEpisodeFromParams(p podcastDirEntry, mp3Path, epID string) *ResolvedID {
	epTitle := episodeTitleFromPath(mp3Path)
	st := getOrCreateEpisodeStatus(mp3Path)
	return &ResolvedID{
		Type: ResolvedTypeEpisode,
		Episode: &ResolvedEpisode{
			Path:           mp3Path,
			Filename:       filepath.Base(mp3Path),
			Title:          epTitle,
			ShortID:        epID,
			PodcastDir:     p.dir,
			PodcastTitle:   p.title,
			PodcastShortID: p.shortID,
			Status:         st,
		},
	}
}

func resolveByPodcastShortID(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	for _, p := range podEntries {
		if strings.EqualFold(p.shortID, search) {
			cfg := loadPodcastConfig(p.dir)
			return &ResolvedID{
				Type: ResolvedTypePodcast,
				Podcast: &ResolvedPodcast{
					Dir:        p.dir,
					Title:      p.title,
					ShortID:    p.shortID,
					FolderName: p.folderName,
					UUID:       cfg.ID,
					Config:     cfg,
				},
			}, true
		}
	}
	return nil, false
}

func resolveByIndex(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	idx, err := strconv.Atoi(search)
	if err != nil || idx < 1 || idx > len(podEntries) {
		return nil, false
	}
	p := podEntries[idx-1]
	cfg := loadPodcastConfig(p.dir)
	return &ResolvedID{
		Type: ResolvedTypePodcast,
		Podcast: &ResolvedPodcast{
			Dir:        p.dir,
			Title:      p.title,
			ShortID:    p.shortID,
			FolderName: p.folderName,
			UUID:       cfg.ID,
			Config:     cfg,
		},
	}, true
}

func resolveByPodcastUUID(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	for _, p := range podEntries {
		cfg := loadPodcastConfig(p.dir)
		cached, _ := loadPodcastCache(p.dir)
		matched := strings.EqualFold(cfg.ID, search) || (cached != nil && strings.EqualFold(cached.ABSItemID, search))
		if matched {
			return &ResolvedID{
				Type: ResolvedTypePodcast,
				Podcast: &ResolvedPodcast{
					Dir:        p.dir,
					Title:      p.title,
					ShortID:    p.shortID,
					FolderName: p.folderName,
					UUID:       cfg.ID,
					Config:     cfg,
				},
			}, true
		}
	}
	return nil, false
}

func resolveByPodcastName(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	for _, p := range podEntries {
		if strings.EqualFold(p.folderName, search) || strings.EqualFold(p.title, search) {
			cfg := loadPodcastConfig(p.dir)
			return &ResolvedID{
				Type: ResolvedTypePodcast,
				Podcast: &ResolvedPodcast{
					Dir:        p.dir,
					Title:      p.title,
					ShortID:    p.shortID,
					FolderName: p.folderName,
					UUID:       cfg.ID,
					Config:     cfg,
				},
			}, true
		}
	}
	return nil, false
}

func resolveByEpisodeFileOrTitle(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	cleanSearch := strings.ToLower(strings.TrimSuffix(search, filepath.Ext(search)))
	for _, p := range podEntries {
		mp3s := findMP3Files(p.dir)
		for _, mp3 := range mp3s {
			fn := filepath.Base(mp3)
			base := stripExt(fn)
			title := episodeTitleFromPath(mp3)
			if strings.EqualFold(fn, search) || strings.EqualFold(base, cleanSearch) || strings.EqualFold(title, search) {
				epID := getOrSetEpisodeShortID(p.dir, p.shortID, mp3)
				return buildResolvedEpisodeFromParams(p, mp3, epID), true
			}
		}
	}
	return nil, false
}

func resolveBySubstring(podEntries []podcastDirEntry, search string) (*ResolvedID, bool) {
	lower := strings.ToLower(search)
	for _, p := range podEntries {
		if strings.Contains(strings.ToLower(p.folderName), lower) || strings.Contains(strings.ToLower(p.title), lower) {
			cfg := loadPodcastConfig(p.dir)
			return &ResolvedID{
				Type: ResolvedTypePodcast,
				Podcast: &ResolvedPodcast{
					Dir:        p.dir,
					Title:      p.title,
					ShortID:    p.shortID,
					FolderName: p.folderName,
					UUID:       cfg.ID,
					Config:     cfg,
				},
			}, true
		}
	}

	for _, p := range podEntries {
		mp3s := findMP3Files(p.dir)
		for _, mp3 := range mp3s {
			fn := filepath.Base(mp3)
			title := episodeTitleFromPath(mp3)
			if strings.Contains(strings.ToLower(fn), lower) || strings.Contains(strings.ToLower(title), lower) {
				epID := getOrSetEpisodeShortID(p.dir, p.shortID, mp3)
				return buildResolvedEpisodeFromParams(p, mp3, epID), true
			}
		}
	}

	return nil, false
}
