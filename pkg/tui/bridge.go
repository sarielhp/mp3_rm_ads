package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"abs/pkg/audio"
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/format"
	"abs/pkg/kitty"
	"abs/pkg/pipeline"
	"abs/pkg/player"
	"abs/pkg/podcast"
	"abs/pkg/transcribe"
	"abs/pkg/types"
	"abs/pkg/util"
	tea "github.com/charmbracelet/bubbletea"
)

type (
	Config               = types.Config
	PodcastConfig        = config.PodcastConfig
	DownloadQueueItem    = podcast.DownloadQueueItem
	UnifiedQueueItem     = types.UnifiedQueueItem
	TranscriptionData    = types.TranscriptionData
	TranscriptionSegment = types.TranscriptionSegment
	CutEntry             = types.CutEntry
	CutsData             = types.CutsData
	PlayerTrack          = types.PlayerTrack
	TUIColorConfig       = types.TUIColorConfig

	PodcastItem   = backend.Podcast
	FeedEpisode   = backend.FeedEpisode
	FeedEnclosure = backend.FeedEnclosure
	ABSClient     = backend.AudiobookshelfBackend
	absItem       = backend.Podcast
	absEpisode    = backend.Episode

	CachedPodcastIndex   = podcast.CachedPodcastIndex
	CachedEpisodeSummary = podcast.CachedEpisodeSummary
	CachedEpisodeDetails = podcast.CachedEpisodeDetails
	syncMutex            = util.SyncMutex
	fileLockWrapper      = util.FileLockWrapper
)

const (
	AdRemovalNone   = config.AdRemovalNone
	AdRemovalAll    = config.AdRemovalAll
	AdRemovalLatest = config.AdRemovalLatest

	DownloadPolicyNone    = config.DownloadPolicyNone
	DownloadPolicyLatest  = config.DownloadPolicyLatest
	DownloadPolicyLatestK = config.DownloadPolicyLatestK
	DownloadPolicyAll     = config.DownloadPolicyAll
)

var globalPlayer = player.GetGlobalPlayer()

func isKittyTerminal() bool  { return kitty.IsKittyTerminal() }
func isKittySupported() bool { return kitty.IsKittySupported() }
func encodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	return kitty.EncodeKittyGraphicsFile(filePath, cols, rows)
}
func kittyClearGraphics() string        { return kitty.KittyClearGraphics() }
func findCoverImage(dir string) string  { return kitty.FindCoverImage(dir) }
func detectImageFormat(path string) int { return kitty.DetectImageFormat(path) }
func mergeSegments(segs []TranscriptionSegment) []TranscriptionSegment {
	return transcribe.MergeSegments(segs)
}
func prewarmPodcastCovers(podcasts []tuiPodcast, cols, rows int) {
	go func() {
		for _, pod := range podcasts {
			cp := pod.coverPath
			if cp == "" {
				cp = findCoverImage(pod.dir)
			}
			if cp != "" {
				_, _ = encodeKittyGraphicsFile(cp, cols, rows)
			}
		}
	}()
}

func formatClock(sec float64) string      { return format.FormatClock(sec) }
func formatTime(sec float64) string       { return format.FormatTime(sec) }
func formatSRTTime(sec float64) string    { return format.FormatSRTTime(sec) }
func formatPlayerTime(sec float64) string { return player.FormatPlayerTime(sec) }
func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToSRT(inputFile, data, customPath, quiet)
	return res
}
func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {
	res, _ := format.ConvertJSONToTXT(inputFile, data, totalDuration, customPath, quiet)
	return res
}

func fileExists(path string) bool { return util.FileExists(path) }
func writeFileAtomic(path string, data []byte, perm uint32) error {
	return util.WriteFileAtomic(path, data, os.FileMode(perm))
}
func displayName(s string) string { return util.DisplayName(s) }
func stripHTML(s string) string   { return backend.StripHTML(s) }
func acquireFileLockWithTimeout(path string, timeout time.Duration) (*fileLockWrapper, error) {
	return util.AcquireFileLockWithTimeout(path, timeout)
}
func findMP3Files(dir string) []string { return util.FindMP3Files(dir) }

func loadPodcastConfig(dir string) PodcastConfig {
	return config.LoadPodcastConfig(dir, config.PodcastConfig{})
}
func savePodcastConfig(dir string, cfg PodcastConfig) error {
	return config.SavePodcastConfig(dir, cfg)
}
func adRemovalModeLabel(mode string) string                    { return config.AdRemovalModeLabel(mode) }
func cycleAdRemovalMode(mode string) string                    { return config.CycleAdRemovalMode(mode) }
func downloadPolicyLabel(policy string, k int) string          { return config.DownloadPolicyLabel(policy, k) }
func normalizeAdRemovalMode(mode string) string                { return config.NormalizeAdRemovalMode(mode) }
func cacheDirForPodcast(dir string) string                     { return podcast.CacheDirForPodcast(dir) }
func loadPodcastCache(dir string) (*CachedPodcastIndex, error) { return podcast.LoadPodcastCache(dir) }
func savePodcastCache(dir string, cache *CachedPodcastIndex) error {
	return podcast.SavePodcastCache(dir, cache)
}
func loadEpisodeDetails(podDir, epFilename string) (*CachedEpisodeDetails, error) {
	return podcast.LoadEpisodeDetails(podDir, epFilename)
}
func saveEpisodeDetails(podDir, epFilename string, details *CachedEpisodeDetails) error {
	return podcast.SaveEpisodeDetails(podDir, epFilename, details)
}

var testDownloadQueuePath string

func getDownloadQueue() *podcast.DownloadQueue {
	q := podcast.DefaultDownloadQueue()
	if testDownloadQueuePath != "" {
		q.SetFilePath(testDownloadQueuePath)
	}
	return q
}

func EnqueueDownload(item DownloadQueueItem, pods ...[]tuiPodcast) (bool, string) {
	return getDownloadQueue().Enqueue(item)
}
func GetDownloadQueueItems() []DownloadQueueItem { return getDownloadQueue().Items() }
func IsEpisodeInDownloadQueue(guid, encURL, title string) bool {
	return getDownloadQueue().IsEpisodeInQueue(guid, encURL, title)
}
func RemoveDownloadQueueItemAt(index int) bool {
	items := getDownloadQueue().Items()
	if index < 0 || index >= len(items) {
		return false
	}
	return getDownloadQueue().Remove(items[index].ID)
}
func ClearDownloadQueue() { getDownloadQueue().Clear() }
func TriggerDownloadQueueWorker(client *ABSClient) {
	getDownloadQueue().TriggerWorker(client)
}
func WaitDownloadWorkerForTest() {
	getDownloadQueue().WaitWorkerForTest()
}

func ensureABSIgnore(dir string) error { return pipeline.EnsureABSIgnore(dir) }
func quarantineAbandonedDuplicates(dir string, tracked []backend.Episode) []string {
	return pipeline.QuarantineAbandonedDuplicates(dir, tracked)
}
func printQuarantinedSummary(quarantined []string, podName string) {
	pipeline.PrintQuarantinedSummary(quarantined, podName)
}

func getBackend(cfg Config, quiet bool) (backend.Backend, error) {
	bCfg := backend.Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       cfg.AudiobookshelfToken,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	}
	return backend.New("audiobookshelf", bCfg)
}

func getABSClient(cfg Config, quiet bool) (*backend.AudiobookshelfBackend, error) {
	client := backend.NewAudiobookshelf(backend.Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       cfg.AudiobookshelfToken,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	})
	return client, nil
}

func NewABSClient(host, token string) *backend.AudiobookshelfBackend {
	return backend.NewAudiobookshelf(backend.Config{
		Host:  host,
		Token: token,
	})
}

func isAudiobookshelfActive(cfg Config) bool {
	return backend.IsAudiobookshelfActive(&cfg)
}

func resetPodcastDateCheck(client *backend.AudiobookshelfBackend, dbPath, itemID, title string) error {
	if client != nil {
		return client.ResetPodcastDateCheck(itemID, title)
	}
	return backend.ResetPodcastDateCheckInDB(dbPath, itemID, title)
}

func sanitizePodcastTitle(title string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := title
	for _, ch := range invalidChars {
		result = strings.ReplaceAll(result, ch, "_")
	}
	result = strings.TrimSpace(result)
	if result == "." || result == ".." || result == "" {
		return "podcast"
	}
	return result
}

func fetchFeedDirect(feedURL, absBaseURL, itemID string) ([]backend.FeedEpisode, string, string, bool, error) {
	return podcast.FetchFeedDirect(feedURL, absBaseURL, itemID)
}

func parsePubDate(pubStr string) int64 {
	if pubStr == "" {
		return 0
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	trimmed := strings.TrimSpace(pubStr)
	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

func loadConfig() Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		return config.DefaultConfig()
	}
	return *cfg
}

func parseABSEpisodePublishedAt(ep *absEpisode) int64 {
	if ep == nil {
		return 0
	}
	if ep.PublishedAt > 0 {
		return ep.PublishedAt
	}
	if ep.PubDate != "" {
		formats := []string{
			time.RFC1123,
			time.RFC1123Z,
			time.RFC3339,
			time.RFC822,
			time.RFC822Z,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05.000Z",
			"2006-01-02",
		}
		for _, f := range formats {
			if t, err := time.Parse(f, strings.TrimSpace(ep.PubDate)); err == nil {
				return t.UnixMilli()
			}
		}
	}
	return 0
}

func normalizeEpisodeTitle(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".mp3")
	name = strings.TrimSuffix(name, ".tmp")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type AdQueueItem struct {
	PodcastDir    string
	PodcastName   string
	Filename      string
	Title         string
	HasAdsRemoved bool
	PublishedAt   int64
	Duration      float64
}

func getAllAdQueueItems(pods []tuiPodcast, q map[string][]string) []AdQueueItem {
	var list []AdQueueItem
	for _, pod := range pods {
		filenames := q[pod.dir]
		for _, fn := range filenames {
			item := AdQueueItem{
				PodcastDir:  pod.dir,
				PodcastName: pod.name,
				Filename:    fn,
				Title:       fn,
			}
			hasAdsRemoved := false
			for _, ep := range pod.episodes {
				if ep.filename == fn {
					item.Title = ep.displayTitle()
					item.HasAdsRemoved = ep.hasAdsRemoved
					item.PublishedAt = ep.publishedAt
					item.Duration = ep.duration
					hasAdsRemoved = ep.hasAdsRemoved
					break
				}
			}
			if !hasAdsRemoved {
				list = append(list, item)
			}
		}
	}
	return list
}

func resolveLocalPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func RunTUI(cfg *types.Config, podcastsDir string) error {
	if podcastsDir == "" && cfg != nil && cfg.PodcastsDir != "" {
		podcastsDir = cfg.PodcastsDir
	}
	if podcastsDir == "" {
		podcastsDir = "~/podcasts"
	}
	podcastsDir = resolveLocalPath(podcastsDir)

	bk := &TuiBackend{
		LoadPodcasts: func(dir string) ([]tuiPodcast, error) {
			return loadTUIPodcastsABS(dir, *cfg)
		},
		LoadQueues: loadAllQueues,
		SaveQueue: func(dir string, entries []string) {
			_ = saveQueue(dir, entries)
		},
		GetDuration: audio.GetAudioDuration,
	}

	p := tea.NewProgram(newTuiModel(bk, podcastsDir, cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
