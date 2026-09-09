package podcast

import (
	"fmt"
	"time"

	"github.com/sariel/abs/pkg/backend"
	"github.com/sariel/abs/pkg/types"
)

type PodcastFreqResult struct {
	Title       string
	Item        backend.Podcast
	Freq        types.PodcastFrequencyInfo
	PodDir      string
	Disabled    bool
	PolicySaved bool
	Err         error
}

func GetEpisodesForFrequency(client backend.Backend, item backend.Podcast, podcastsDir string, refresh bool, feedCache *FeedCacheManager) ([]backend.FeedEpisode, error) {
	if feedCache == nil {
		feedCache = DefaultFeedCache()
	}
	feedURL := item.Media.Metadata.FeedURL
	podDir := FindPodcastDirForItem(item, podcastsDir)

	if !refresh {
		if feedURL != "" {
			if entry := feedCache.Get(feedURL); entry != nil && len(entry.Episodes) > 0 && !entry.IsExpired(FeedCacheDefaultTTL) {
				return entry.Episodes, nil
			}
		}
		if podDir != "" {
			if eps := LoadCachedFeedEpisodes(podDir); len(eps) > 0 {
				return eps, nil
			}
		}
	}

	if client != nil && feedURL != "" {
		if feedEpisodes, err := client.PodcastFeedEpisodes(feedURL); err == nil && len(feedEpisodes) > 0 {
			takeCount := min(100, len(feedEpisodes))
			cachedEps := feedEpisodes[:takeCount]
			feedCache.Put(feedURL, &FeedCacheEntry{
				FeedURL:     feedURL,
				LastChecked: time.Now(),
				Episodes:    cachedEps,
			})
			return cachedEps, nil
		}
	}

	if len(item.Media.Episodes) > 0 {
		var eps []backend.FeedEpisode
		for _, ep := range item.Media.Episodes {
			eps = append(eps, backend.FeedEpisode{
				Title:       ep.Title,
				PubDate:     ep.PubDate,
				PublishedAt: ep.PublishedAt,
			})
		}
		return eps, nil
	}

	if podDir != "" {
		if eps := LoadCachedFeedEpisodes(podDir); len(eps) > 0 {
			return eps, nil
		}
	}

	return nil, fmt.Errorf("no episodes available")
}

func LoadCachedFeedEpisodes(podDir string) []backend.FeedEpisode {
	if cache, _ := LoadPodcastCache(podDir); cache != nil && len(cache.Episodes) > 0 {
		eps := make([]backend.FeedEpisode, len(cache.Episodes))
		for i, ep := range cache.Episodes {
			eps[i] = backend.FeedEpisode{Title: ep.Title, PublishedAt: ep.PublishedAt}
		}
		return eps
	}
	return nil
}
