package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/podcast"
	"abs/pkg/types"
)

func (m *tuiModel) fetchPodcastFullFeed() {
	if m.podIdx < 0 || m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	feedURL := pod.feedURL
	if feedURL == "" && pod.absData != nil {
		feedURL = pod.absData.Media.Metadata.FeedURL
	}
	if feedURL == "" {
		m.showPopup("No RSS feed URL configured for this podcast")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		def := config.DefaultConfig()
		cfg = &def
	}
	bCli, _ := backend.FromAppConfig(cfg, true)
	feedEpisodes, err := fetchFeedEpisodesForPodcast(pod, *cfg, bCli, feedURL)
	if err != nil && len(feedEpisodes) == 0 {
		m.showPopup(fmt.Sprintf("Failed to fetch feed: %v", err))
		return
	}

	existingTitles, existingGUIDs, existingURLs := collectExistingEpisodeIdentifiers(pod.episodes)
	var addedCount int

	for _, fe := range feedEpisodes {
		tNorm := strings.ToLower(strings.TrimSpace(fe.Title))
		encURL := fe.EnclosureURL
		if fe.Enclosure != nil && fe.Enclosure.URL != "" {
			encURL = fe.Enclosure.URL
		}

		if (tNorm != "" && existingTitles[tNorm]) || (fe.GUID != "" && existingGUIDs[fe.GUID]) || (encURL != "" && existingURLs[encURL]) {
			continue
		}

		newEp := convertFeedEpisodeToTUI(fe, pod.dir)
		pod.episodes = append(pod.episodes, newEp)
		addedCount++
	}

	sort.Slice(pod.episodes, func(i, j int) bool {
		return pod.episodes[i].displayDate().After(pod.episodes[j].displayDate())
	})

	if addedCount > 0 {
		m.showPopup(fmt.Sprintf("✓ Fetched %d online episode(s) (%d total in archive)", addedCount, len(pod.episodes)))
	} else {
		m.showPopup(fmt.Sprintf("✓ Feed is up-to-date (%d total episodes)", len(pod.episodes)))
	}
}

func fetchFeedEpisodesForPodcast(pod *tuiPodcast, cfg types.Config, client backend.Backend, feedURL string) ([]backend.FeedEpisode, error) {
	var feedEpisodes []backend.FeedEpisode
	var err error
	if client != nil {
		feedEpisodes, err = client.PodcastFeedEpisodes(feedURL)
	}
	if len(feedEpisodes) == 0 {
		feedEpisodes, _, _, _, err = podcast.FetchFeedDirect(feedURL, "", "")
	}
	return feedEpisodes, err
}

func collectExistingEpisodeIdentifiers(episodes []tuiEpisode) (map[string]bool, map[string]bool, map[string]bool) {
	existingTitles := make(map[string]bool, len(episodes)*2)
	existingGUIDs := make(map[string]bool, len(episodes))
	existingURLs := make(map[string]bool, len(episodes))

	for _, ep := range episodes {
		if ep.title != "" {
			existingTitles[strings.ToLower(strings.TrimSpace(ep.title))] = true
		}
		if ep.guid != "" {
			existingGUIDs[ep.guid] = true
		}
		if ep.absData != nil && ep.absData.Title != "" {
			existingTitles[strings.ToLower(strings.TrimSpace(ep.absData.Title))] = true
		}
		if ep.enclosureURL != "" {
			existingURLs[ep.enclosureURL] = true
		}
	}
	return existingTitles, existingGUIDs, existingURLs
}

func convertFeedEpisodeToTUI(fe backend.FeedEpisode, podDir string) tuiEpisode {
	safeFilename := backend.SanitizePodcastTitle(fe.Title) + ".mp3"
	desc := fe.Description
	if desc == "" {
		desc = fe.DescriptionPlain
	}
	if desc == "" {
		desc = fe.Subtitle
	}

	pubTime := time.UnixMilli(fe.PublishedAt)
	if fe.PublishedAt == 0 && fe.PubDate != "" {
		pubTime = time.UnixMilli(backend.ParsePubDate(fe.PubDate))
	}

	encURL := fe.EnclosureURL
	if fe.Enclosure != nil && fe.Enclosure.URL != "" {
		encURL = fe.Enclosure.URL
	}

	_ = podcast.SaveEpisodeDetails(podDir, safeFilename, &podcast.CachedEpisodeDetails{
		Filename:    safeFilename,
		Title:       fe.Title,
		Subtitle:    fe.Subtitle,
		Description: desc,
		EpisodeType: fe.EpisodeType,
	})

	return tuiEpisode{
		filename:      safeFilename,
		path:          filepath.Join(podDir, safeFilename),
		title:         fe.Title,
		hasAdsRemoved: false,
		hasTranscript: false,
		publishedAt:   pubTime.UnixMilli(),
		modTime:       pubTime,
		duration:      fe.DurationSeconds,
		durationDone:  true,
		season:        fe.Season,
		episode:       fe.Episode,
		isFeedOnly:    true,
		enclosureURL:  encURL,
		guid:          fe.GUID,
		description:   desc,
	}
}

func (m *tuiModel) downloadAllForSelectedPodcast() {
	if m.podIdx < 0 || m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]

	m.fetchPodcastFullFeed()

	var toDownload []backend.FeedEpisode
	var count int

	for _, ep := range pod.episodes {
		if ep.isFeedOnly {
			encURL := ep.enclosureURL
			if encURL == "" {
				continue
			}
			fe := backend.FeedEpisode{
				Title:           ep.title,
				GUID:            ep.guid,
				PublishedAt:     ep.publishedAt,
				DurationSeconds: ep.duration,
				EnclosureURL:    encURL,
				Enclosure:       &backend.FeedEnclosure{URL: encURL},
				Description:     ep.description,
			}
			toDownload = append(toDownload, fe)
			podID := ""
			if pod.absData != nil {
				podID = pod.absData.ID
			}
			item := podcast.DownloadQueueItem{
				PodcastTitle: pod.name,
				PodcastDir:   pod.dir,
				PodcastID:    podID,
				EpisodeTitle: ep.title,
				GUID:         ep.guid,
				PublishedAt:  ep.publishedAt,
				DurationSec:  ep.duration,
				EnclosureURL: encURL,
			}
			_, _ = podcast.DefaultDownloadQueue().Enqueue(item)
			count++
		}
	}

	cfg, _ := config.LoadConfig()
	if cfg != nil {
		client, _ := backend.FromAppConfig(cfg, true)
		if client != nil && len(toDownload) > 0 && pod.absData != nil {
			_ = client.DownloadEpisodes(pod.absData.ID, toDownload)
		}
	}

	if count > 0 {
		m.showPopup(fmt.Sprintf("✓ Queued %d episode(s) for download", count))
	} else {
		m.showPopup("All episodes are already downloaded")
	}
}
