package backend

import (
	"context"
	"fmt"
	"time"
)

type PodcastReader interface {
	Libraries() ([]Library, error)
	PodcastLibraries() ([]Library, error)
	Podcasts() ([]Podcast, error)
	GetPodcast(id string) (*Podcast, error)
	PodcastFeedEpisodes(feedURL string) ([]FeedEpisode, error)
	ActiveDownloads(podcastID string) ([]ActiveDownload, error)
	OpenRSSFeed(podcastID, baseURL string) (string, error)
	DownloadCover(podcastID, destPath string) error
}

type PodcastWriter interface {
	CreatePodcast(libraryID, folderID, path, title, feedURL string) (*Podcast, error)
	DeletePodcast(id string) error
	DeleteItem(id string) error
	DownloadEpisodes(podcastID string, episodes []FeedEpisode) error
	DeletePodcastEpisode(podcastID, episodeID string) error
	ResetPodcastDateCheck(itemID, title string) error
	ResetPodcastDateCheckAPI(itemID string) error
	SyncDuration(filePath string, duration float64) error
	ApplyKeepPolicy(podcastID, podcastTitle string, keep int, dryRun, verbose, quiet bool) (int, error)
	UpdatePodcastSettings(podcastID string, autoDownload, autoCleanup bool, autoCleanupDays int) error
}

type PodcastAdmin interface {
	TestConnection(quiet bool) (bool, error)
	Login() (string, error)
	Scan(opts ScanOptions) (ScanResult, error)
	Rescan(opts RescanOptions) (RescanResult, error)
	ExportOPML(opts OPMLExportOptions) ([]byte, error)
	ImportOPML(data []byte, opts OPMLImportOptions) (OPMLImportResult, error)
	FetchPodcastFeeds(silent, verbose bool) ([]OPMLFeed, error)
	WaitForActiveDownloads(podcasts []Podcast, quiet bool, timeout time.Duration) error
}

type Backend interface {
	Name() string
	PodcastReader
	PodcastWriter
	PodcastAdmin
}

type Config struct {
	Host              string
	User              string
	Pass              string
	Token             string
	APIKey            string
	DBPath            string
	PodcastsDir       string
	SubscriptionsFile string
	Timeout           time.Duration
	MaxAttempts       int
	RetryDelay        time.Duration
	Quiet             bool
	Verbose           bool
}

type FactoryFunc func(cfg Config) (Backend, error)

var backends = make(map[string]FactoryFunc)

func Register(name string, factory FactoryFunc) {
	backends[name] = factory
}

func New(name string, cfg Config) (Backend, error) {
	if name == "" {
		name = "standalone"
	}
	factory, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %s", name)
	}
	return factory(cfg)
}

func NewContext(ctx context.Context, name string, cfg Config) (Backend, error) {
	return New(name, cfg)
}
