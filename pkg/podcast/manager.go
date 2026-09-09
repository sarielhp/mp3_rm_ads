package podcast

import (
	"github.com/sariel/abs/pkg/backend"
	"github.com/sariel/abs/pkg/config"
	"github.com/sariel/abs/pkg/types"
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

func (m *PodcastManager) AuditTranscripts(targets []string, minRatio float64, minChars int) []*TranscriptAuditItem {
	if len(targets) == 0 && m.Config.PodcastsDir != "" {
		targets = []string{m.Config.PodcastsDir}
	}
	audioFiles := CollectAudioFilesForAudit(targets)
	var results []*TranscriptAuditItem
	for _, f := range audioFiles {
		if item := InspectEpisodeTranscript(f, minRatio, minChars); item != nil {
			results = append(results, item)
		}
	}
	return results
}
