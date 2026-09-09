package podcast

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/backend"
	"github.com/sariel/abs/pkg/pipeline"
	"github.com/sariel/abs/pkg/util"
)

type CachedEpisodeSummary struct {
	ID            string  `json:"id,omitempty"`
	Path          string  `json:"path"`
	Filename      string  `json:"filename"`
	Title         string  `json:"title"`
	PublishedAt   int64   `json:"published_at"`
	Duration      float64 `json:"duration"`
	FileSize      int64   `json:"file_size"`
	Season        string  `json:"season,omitempty"`
	Episode       string  `json:"episode,omitempty"`
	HasAdsRemoved bool    `json:"has_ads_removed"`
	HasTranscript bool    `json:"has_transcript,omitempty"`
}

type CachedPodcastIndex struct {
	PodcastName string                 `json:"podcast_name"`
	PodcastDir  string                 `json:"podcast_dir"`
	ABSItemID   string                 `json:"abs_item_id,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Description string                 `json:"description,omitempty"`
	FeedURL     string                 `json:"feed_url,omitempty"`
	CoverPath   string                 `json:"cover_path,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Episodes    []CachedEpisodeSummary `json:"episodes"`
}

type CachedEpisodeDetails struct {
	Path        string           `json:"path"`
	Filename    string           `json:"filename"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Subtitle    string           `json:"subtitle,omitempty"`
	EpisodeType string           `json:"episode_type,omitempty"`
	Genres      []string         `json:"genres,omitempty"`
	Author      string           `json:"author,omitempty"`
	FeedURL     string           `json:"feed_url,omitempty"`
	RawABS      *backend.Episode `json:"raw_abs,omitempty"`
}

func CacheBaseDir() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "cache")
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(cacheHome, "abs", "podcasts")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func SanitizeDirName(dirPath string) string {
	clean := filepath.Clean(dirPath)
	base := filepath.Base(clean)
	h := sha256.Sum256([]byte(clean))
	hashPrefix := hex.EncodeToString(h[:4])
	safeBase := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, base)
	return safeBase + "_" + hashPrefix
}

func CacheDirForPodcast(podcastDir string) string {
	absDir, err := filepath.Abs(podcastDir)
	if err != nil {
		absDir = podcastDir
	}
	name := SanitizeDirName(absDir)
	dir := filepath.Join(CacheBaseDir(), name)
	_ = os.MkdirAll(dir, 0755)
	_ = os.MkdirAll(filepath.Join(dir, "details"), 0755)
	return dir
}

func LoadPodcastCache(podcastDir string) (*CachedPodcastIndex, error) {
	cDir := CacheDirForPodcast(podcastDir)
	indexPath := filepath.Join(cDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	var index CachedPodcastIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func SavePodcastCache(podcastDir string, index *CachedPodcastIndex) error {
	cDir := CacheDirForPodcast(podcastDir)
	indexPath := filepath.Join(cDir, "index.json")
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(indexPath, append(data, '\n'), 0644)
}

func detailFileName(filename string) string {
	h := sha256.Sum256([]byte(filename))
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, filename)
	if len(safe) > 32 {
		safe = safe[:32]
	}
	return safe + "_" + hex.EncodeToString(h[:4]) + ".json"
}

func LoadEpisodeDetails(podcastDir, filename string) (*CachedEpisodeDetails, error) {
	cDir := CacheDirForPodcast(podcastDir)
	detPath := filepath.Join(cDir, "details", detailFileName(filename))
	data, err := os.ReadFile(detPath)
	if err != nil {
		return nil, err
	}
	var det CachedEpisodeDetails
	if err := json.Unmarshal(data, &det); err != nil {
		return nil, err
	}
	return &det, nil
}

func SaveEpisodeDetails(podcastDir, filename string, details *CachedEpisodeDetails) error {
	cDir := CacheDirForPodcast(podcastDir)
	detPath := filepath.Join(cDir, "details", detailFileName(filename))
	data, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(detPath, append(data, '\n'), 0644)
}

func ResetCache() error {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			cacheHome = filepath.Join(os.TempDir(), "cache")
		} else {
			cacheHome = filepath.Join(home, ".cache")
		}
	}
	absCacheDir := filepath.Join(cacheHome, "abs")
	return os.RemoveAll(absCacheDir)
}

func CacheStats() (dir string, entries int, bytes int64) {
	dir = CacheBaseDir()
	items, err := os.ReadDir(dir)
	if err != nil {
		return dir, 0, 0
	}
	entries = len(items)
	for _, it := range items {
		p := filepath.Join(dir, it.Name())
		_ = filepath.Walk(p, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && fi != nil && !fi.IsDir() {
				bytes += fi.Size()
			}
			return nil
		})
	}
	return dir, entries, bytes
}

func GetEpisodePublicationTime(filePath string) time.Time {
	statPath := pipeline.StatusPathFor(filePath)
	if st, err := pipeline.LoadEpisodeStatus(statPath); err == nil && st != nil && st.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, st.PublishedAt); err == nil && !t.IsZero() {
			return t
		}
	}
	dir := filepath.Dir(filePath)
	fn := filepath.Base(filePath)
	if cached, _ := LoadPodcastCache(dir); cached != nil {
		for _, ep := range cached.Episodes {
			if (ep.Filename == fn || ep.Path == filePath) && ep.PublishedAt > 0 {
				return time.UnixMilli(ep.PublishedAt)
			}
		}
		basePrefix := util.StripExt(fn)
		if idx := strings.LastIndex(basePrefix, " ("); idx != -1 && strings.HasSuffix(basePrefix, ")") {
			basePrefix = strings.TrimSpace(basePrefix[:idx])
		}
		for _, ep := range cached.Episodes {
			epPrefix := util.StripExt(ep.Filename)
			if idx := strings.LastIndex(epPrefix, " ("); idx != -1 && strings.HasSuffix(epPrefix, ")") {
				epPrefix = strings.TrimSpace(epPrefix[:idx])
			}
			if (epPrefix == basePrefix || strings.HasPrefix(ep.Filename, basePrefix) || strings.HasPrefix(fn, epPrefix)) && ep.PublishedAt > 0 {
				return time.UnixMilli(ep.PublishedAt)
			}
		}
	}
	if !strings.Contains(filePath, "abs_remote") && !strings.Contains(filePath, "/.work/") {
		if fi, err := os.Stat(filePath); err == nil {
			return fi.ModTime()
		}
	}
	return time.Time{}
}
