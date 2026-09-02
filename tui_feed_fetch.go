package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

	cfg := loadConfig()
	client, _ := getABSClient(cfg, true)

	feedEpisodes, err := fetchFeedEpisodesForPodcast(pod, cfg, client, feedURL)
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

func fetchFeedEpisodesForPodcast(pod *tuiPodcast, cfg Config, client *ABSClient, feedURL string) ([]FeedEpisode, error) {
	itemID := ""
	if pod.absData != nil {
		itemID = pod.absData.ID
	}
	_ = resetPodcastDateCheck(client, cfg.AudiobookshelfDBPath, itemID, pod.name)

	var feedEpisodes []FeedEpisode
	var err error
	if client != nil {
		feedEpisodes, err = client.PodcastFeedEpisodes(feedURL)
	}
	if len(feedEpisodes) == 0 {
		feedEpisodes, _, _, _, err = fetchFeedDirect(feedURL, "", "")
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

func convertFeedEpisodeToTUI(fe FeedEpisode, podDir string) tuiEpisode {
	safeFilename := sanitizePodcastTitle(fe.Title) + ".mp3"
	desc := fe.Description
	if desc == "" {
		desc = fe.DescriptionPlain
	}
	if desc == "" {
		desc = fe.Subtitle
	}

	pubTime := time.UnixMilli(fe.PublishedAt)
	if fe.PublishedAt == 0 && fe.PubDate != "" {
		pubTime = time.UnixMilli(parsePubDate(fe.PubDate))
	}

	encURL := fe.EnclosureURL
	if fe.Enclosure != nil && fe.Enclosure.URL != "" {
		encURL = fe.Enclosure.URL
	}

	_ = saveEpisodeDetails(podDir, safeFilename, &CachedEpisodeDetails{
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

	var toDownload []FeedEpisode
	var count int

	for _, ep := range pod.episodes {
		if ep.isFeedOnly {
			encURL := ep.enclosureURL
			if encURL == "" {
				continue
			}
			fe := FeedEpisode{
				Title:           ep.title,
				GUID:            ep.guid,
				PublishedAt:     ep.publishedAt,
				DurationSeconds: ep.duration,
				EnclosureURL:    encURL,
				Enclosure:       &FeedEnclosure{URL: encURL},
				Description:     ep.description,
			}
			toDownload = append(toDownload, fe)
			podID := ""
			if pod.absData != nil {
				podID = pod.absData.ID
			}
			item := DownloadQueueItem{
				PodcastTitle: pod.name,
				PodcastDir:   pod.dir,
				PodcastID:    podID,
				EpisodeTitle: ep.title,
				GUID:         ep.guid,
				PublishedAt:  ep.publishedAt,
				DurationSec:  ep.duration,
				EnclosureURL: encURL,
			}
			_, _ = EnqueueDownload(item, m.podcasts)
			count++
		}
	}

	cfg := loadConfig()
	client, _ := getABSClient(cfg, true)
	if client != nil && len(toDownload) > 0 && pod.absData != nil {
		_ = client.DownloadEpisodes(pod.absData.ID, toDownload)
	}

	if count > 0 {
		m.showPopup(fmt.Sprintf("✓ Queued %d episode(s) for download", count))
	} else {
		m.showPopup("All episodes are already downloaded")
	}
}
