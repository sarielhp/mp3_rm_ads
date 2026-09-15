package podcast

import (
	"pod/pkg/backend"
	"pod/pkg/config"
	"pod/pkg/types"
)

type PodcastManager struct {
	Config    *types.Config
	Backend   backend.Backend
	FeedCache *FeedCacheManager
	Queue     *DownloadQueue
}

func NewPodcastManager(cfg *types.Config, b backend.Backend) *PodcastManager {
	if cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
	}
	return &PodcastManager{
		Config:    cfg,
		Backend:   b,
		FeedCache: DefaultFeedCache(),
		Queue:     DefaultDownloadQueue(),
	}
}

func (m *PodcastManager) ResolveID(query string) (*ResolvedID, error) {
	return ResolveAnyID(m.Config.PodcastsDir, query)
}

func (m *PodcastManager) ScanPodcasts() []PodcastDirEntry {
	return ScanPodcastDirs(m.Config.PodcastsDir)
}

func (m *PodcastManager) CleanOrphans(opts CleanOrphansOptions) (CleanOrphansResult, error) {
	return RunCleanOrphans(m.Backend, opts)
}
