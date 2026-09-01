package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) handleKeyPart2(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "p", "P":
		if m.screen == screenPodcastDetail && len(m.selectedEpisodes) > 0 {
			m.batchQueuePlayback()
		} else if m.screen == screenPodcastDetail || m.screen == screenEpisodeDetail {
			m.playSelectedEpisode()
		}

	case "v", "V":
		if m.screen == screenPodcastDetail {
			eps := m.filteredEpisodes()
			if m.epIdx >= 0 && m.epIdx < len(eps) {
				m.toggleEpisodeSelection(eps[m.epIdx].path)
			}
		}

	case "a", "A":
		if m.screen == screenPodcastDetail && len(m.selectedEpisodes) > 0 {
			m.batchQueueAdRemoval()
		}

	case " ":
		globalPlayer.TogglePause()
		kv := globalPlayer.View()
		if kv.IsPaused {
			m.showPopup("Paused")
		} else if kv.IsPlaying {
			m.showPopup("Resumed")
		}

	case "right", "l", ">":
		if globalPlayer.View().IsPlaying {
			globalPlayer.Seek(30)
			m.showPopup("+30s (" + formatPlayerTime(globalPlayer.View().Position) + ")")
		}

	case "left", "h", "<":
		if globalPlayer.View().IsPlaying {
			globalPlayer.Seek(-30)
			m.showPopup("-30s (" + formatPlayerTime(globalPlayer.View().Position) + ")")
		}

	case "+", "=", "]":
		globalPlayer.VolumeUp()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.View().Volume))

	case "-", "_", "[":
		globalPlayer.VolumeDown()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.View().Volume))

	case "m", "M":
		globalPlayer.ToggleMute()
		if globalPlayer.View().Muted {
			m.showPopup("Muted")
		} else {
			m.showPopup("Unmuted")
		}

	case "s", "S":
		globalPlayer.CycleSpeaker()
		m.showPopup("Speaker: " + globalPlayer.View().CurrentSpeaker)

	case "n", "N":
		globalPlayer.Next()
		m.showPopup("Next track")

	case "c", "C":
		if m.screen == screenPodcasts || m.screen == screenPodcastDetail {
			m.openPolicyModal()
		} else {
			globalPlayer.ClearQueue()
			m.showPopup("Queue cleared")
		}

	case "f", "F":
		if m.screen == screenPodcasts || m.screen == screenPodcastDetail || m.screen == screenEpisodeDetail {
			m.fetchPodcastFullFeed()
		}

	case "d", "D":
		if m.screen == screenPodcasts {
			m.downloadAllForSelectedPodcast()
		} else if m.screen == screenPodcastDetail {
			if len(m.selectedEpisodes) > 0 {
				m.batchQueueDownload()
			} else {
				m.enqueueCurrentEpisodeDownload()
			}
		} else if m.screen == screenEpisodeDetail {
			m.enqueueCurrentEpisodeDownload()
		}

	case "t", "T":
		if m.screen == screenPodcastDetail || m.screen == screenEpisodeDetail {
			m.openTranscriptViewer()
		}

	case "e", "E", "o", "O":
		if m.screen == screenPodcasts || m.screen == screenPodcastDetail {
			m.openTimelineViewer()
		}

	case "up", "k":
		if m.screen == screenEpisodeDetail {
			if m.descScroll > 0 {
				m.descScroll--
			}
		} else {
			m.handleUp()
		}

	case "down", "j":
		if m.screen == screenEpisodeDetail {
			m.descScroll++
		} else {
			m.handleDown()
		}

	case "enter":
		return m.handleEnter()

	case "esc":
		m.handleEscape()

	case "r", "R":
		m.handleQueueToggle()

	case "i", "I":
		m.showCover = !m.showCover

	case "b", "B":
		m.showHelp = !m.showHelp

	case "/":
		m.searchMode = true
		m.searchQuery = ""
		return m, nil

	case "ctrl+s":
		m.handleSortToggle()
	}

	return m, nil
}

func (m *tuiModel) playSelectedEpisode() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]
	track := PlayerTrack{
		Title:    ep.displayTitle(),
		Podcast:  pod.name,
		Path:     ep.path,
		Duration: ep.duration,
	}

	if !ep.hasAdsRemoved {
		entries := m.queue[pod.dir]
		found := false
		for _, q := range entries {
			if q == ep.filename {
				found = true
				break
			}
		}
		if !found {
			m.queue[pod.dir] = append(entries, ep.filename)
			if m.bk != nil && m.bk.SaveQueue != nil {
				m.bk.SaveQueue(pod.dir, m.queue[pod.dir])
			}
			m.showPopup(fmt.Sprintf("Playing %s (Added to ad queue)", truncate(track.Title, 20)))
		} else {
			m.showPopup("Playing " + truncate(track.Title, 25))
		}
	} else {
		m.showPopup("Playing " + truncate(track.Title, 25))
	}

	globalPlayer.PlayTrack(track)
	if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenTranscript && m.screen != screenTimeline {
		m.prevScreen = m.screen
	}
	m.screen = screenPlayer
}
