package tui

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
	m.showToast(fmt.Sprintf("Batch added %d episode(s) to AdR queue", addedCount), ToastSuccess)
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
	item := downloadQueueItemFor(pod.name, pod.dir, podcastBackendID(&pod), ep)
	ok, reason := m.lib.Queue().Enqueue(item)
	if ok {
		m.showToast("Enqueued for download: "+ep.displayTitle(), ToastSuccess)
		m.lib.Queue().TriggerWorker(m.lib.Backend())
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
		item := downloadQueueItemFor(pod.name, pod.dir, podcastBackendID(&pod), ep)
		ok, _ := m.lib.Queue().Enqueue(item)
		if ok {
			queuedCount++
		}
	}
	m.clearSelectedEpisodes()
	if queuedCount > 0 {
		m.showToast(fmt.Sprintf("Batch enqueued %d episode(s) for download", queuedCount), ToastSuccess)
		m.lib.Queue().TriggerWorker(m.lib.Backend())
	} else {
		m.showToast("No new episodes enqueued", ToastWarning)
	}
}
