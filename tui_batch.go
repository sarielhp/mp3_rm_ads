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
