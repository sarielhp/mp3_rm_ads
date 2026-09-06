package main

import (
	"fmt"
)

func (m *tuiModel) toggleEpisodeSelection(path string) {
	if m.selectedEpisodes == nil {
		m.selectedEpisodes = make(map[string]bool)
	}
	if m.selectedEpisodes[path] {
		delete(m.selectedEpisodes, path)
	} else {
		m.selectedEpisodes[path] = true
	}
}

func (m *tuiModel) isEpisodeSelected(path string) bool {
	if m.selectedEpisodes == nil {
		return false
	}
	return m.selectedEpisodes[path]
}

func (m *tuiModel) clearSelectedEpisodes() {
	m.selectedEpisodes = make(map[string]bool)
}

func (m *tuiModel) batchQueueAdRemoval() {
	if len(m.selectedEpisodes) == 0 || m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	entries := m.queue[pod.dir]
	if entries == nil {
		entries = []string{}
	}

	addedCount := 0
	for _, ep := range pod.episodes {
		if m.selectedEpisodes[ep.path] && !ep.hasAdsRemoved {
			found := false
			for _, q := range entries {
				if q == ep.filename {
					found = true
					break
				}
			}
			if !found {
				entries = append(entries, ep.filename)
				addedCount++
			}
		}
	}

	m.queue[pod.dir] = entries
	if m.bk != nil && m.bk.SaveQueue != nil {
		m.bk.SaveQueue(pod.dir, entries)
	}
	m.clearSelectedEpisodes()
	m.showToast(fmt.Sprintf("Batch added %d episode(s) to ad removal queue", addedCount), ToastSuccess)
}

func (m *tuiModel) batchQueuePlayback() {
	if len(m.selectedEpisodes) == 0 || m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	addedCount := 0

	for _, ep := range pod.episodes {
		if m.selectedEpisodes[ep.path] {
			track := PlayerTrack{
				Title:    ep.displayTitle(),
				Podcast:  pod.name,
				Path:     ep.path,
				Duration: ep.duration,
			}
			globalPlayer.EnqueueAndPlay(track)
			addedCount++
		}
	}

	m.clearSelectedEpisodes()
	m.showToast(fmt.Sprintf("Batch enqueued %d episode(s) for playback", addedCount), ToastSuccess)
}

func (m *tuiModel) enqueueCurrentEpisodeDownload() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]
	epGUID := ""
	pubDate := ""
	var pubAt int64
	if ep.absData != nil {
		epGUID = ep.absData.ID
		pubDate = ep.absData.PubDate
		pubAt = parseABSEpisodePublishedAt(ep.absData)
	}
	if epGUID == "" && ep.guid != "" {
		epGUID = ep.guid
	}
	if pubAt == 0 && ep.publishedAt > 0 {
		pubAt = ep.publishedAt
	}
	podID := ""
	if pod.absData != nil {
		podID = pod.absData.ID
	}
	item := DownloadQueueItem{
		PodcastTitle: pod.name,
		PodcastDir:   pod.dir,
		PodcastID:    podID,
		EpisodeTitle: ep.displayTitle(),
		GUID:         epGUID,
		PubDate:      pubDate,
		PublishedAt:  pubAt,
		DurationSec:  ep.duration,
		EnclosureURL: ep.enclosureURL,
	}
	ok, reason := EnqueueDownload(item, m.podcasts)
	if ok {
		m.showToast("Enqueued for download: "+ep.displayTitle(), ToastSuccess)
		cfg := loadConfig()
		var absCli *ABSClient
		if isAudiobookshelfActive(cfg) && cfg.AudiobookshelfURL != "" {
			absCli, _ = getABSClient(cfg, true)
		}
		TriggerDownloadQueueWorker(absCli)
	} else if reason == "already_queued" {
		m.showToast("Already in download queue", ToastWarning)
	} else if reason == "already_downloaded" {
		m.showToast("Episode already downloaded", ToastInfo)
	} else {
		m.showToast("Failed to enqueue download", ToastError)
	}
}

func (m *tuiModel) batchQueueDownload() {
	if len(m.selectedEpisodes) == 0 || m.podIdx >= len(m.podcasts) {
		return
	}
	pod := m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	queuedCount := 0

	for _, ep := range eps {
		if !m.isEpisodeSelected(ep.path) {
			continue
		}
		epGUID := ""
		pubDate := ""
		var pubAt int64
		if ep.absData != nil {
			epGUID = ep.absData.ID
			pubDate = ep.absData.PubDate
			pubAt = parseABSEpisodePublishedAt(ep.absData)
		}
		if epGUID == "" && ep.guid != "" {
			epGUID = ep.guid
		}
		if pubAt == 0 && ep.publishedAt > 0 {
			pubAt = ep.publishedAt
		}
		podID := ""
		if pod.absData != nil {
			podID = pod.absData.ID
		}
		item := DownloadQueueItem{
			PodcastTitle: pod.name,
			PodcastDir:   pod.dir,
			PodcastID:    podID,
			EpisodeTitle: ep.displayTitle(),
			GUID:         epGUID,
			PubDate:      pubDate,
			PublishedAt:  pubAt,
			DurationSec:  ep.duration,
			EnclosureURL: ep.enclosureURL,
		}
		ok, _ := EnqueueDownload(item, m.podcasts)
		if ok {
			queuedCount++
		}
	}
	m.clearSelectedEpisodes()
	if queuedCount > 0 {
		m.showToast(fmt.Sprintf("Batch enqueued %d episode(s) for download", queuedCount), ToastSuccess)
		cfg := loadConfig()
		var absCli *ABSClient
		if isAudiobookshelfActive(cfg) && cfg.AudiobookshelfURL != "" {
			absCli, _ = getABSClient(cfg, true)
		}
		TriggerDownloadQueueWorker(absCli)
	} else {
		m.showToast("No new episodes enqueued", ToastWarning)
	}
}
