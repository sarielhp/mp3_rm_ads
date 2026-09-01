package main

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	debugLog("KEY: %q (screen: %s, podIdx: %d, epIdx: %d)", s, m.screenName(), m.podIdx, m.epIdx)

	if s == "f12" || s == "F12" {
		snapPath := m.takeSnapshot()
		m.showPopup("📸 Snapshot: " + filepath.Base(snapPath))
		return m, nil
	}

	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	if m.showHelpModal {
		switch s {
		case "esc", "?", "h", "q", "Q", "enter":
			m.showHelpModal = false
		}
		return m, nil
	}

	if m.showPolicyModal {
		switch s {
		case "up", "k":
			if m.policyModalIdx > 0 {
				m.policyModalIdx--
			}
		case "down", "j":
			if m.policyModalIdx < 2 {
				m.policyModalIdx++
			}
		case "1":
			m.policyModalIdx = 0
			m.applyPolicyModal()
		case "2":
			m.policyModalIdx = 1
			m.applyPolicyModal()
		case "3":
			m.policyModalIdx = 2
			m.applyPolicyModal()
		case "enter":
			m.applyPolicyModal()
		case "esc", "q", "Q", "c", "C":
			m.showPolicyModal = false
		}
		return m, nil
	}

	if m.showDownloadPolicyModal {
		switch s {
		case "up", "k":
			if m.downloadPolicyModalIdx > 0 {
				m.downloadPolicyModalIdx--
			}
		case "down", "j":
			if m.downloadPolicyModalIdx < 3 {
				m.downloadPolicyModalIdx++
			}
		case "1":
			m.downloadPolicyModalIdx = 0
		case "2":
			m.downloadPolicyModalIdx = 1
		case "3":
			m.downloadPolicyModalIdx = 2
		case "4":
			m.downloadPolicyModalIdx = 3
		case "+", "=", "right", "l":
			if m.downloadPolicyModalK < 99 {
				m.downloadPolicyModalK++
			}
		case "-", "_", "left", "h":
			if m.downloadPolicyModalK > 1 {
				m.downloadPolicyModalK--
			}
		case "enter":
			m.applyDownloadPolicyModal()
		case "esc", "q", "d", "D":
			m.showDownloadPolicyModal = false
		}
		return m, nil
	}

	if m.loadErr != "" {
		switch s {
		case "r", "R":
			m.loadErr = ""
			m.loading = true
			return m, m.Init()
		case "esc", "q", "Q", "ctrl+c", "ctrl+d":
			m.done = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch s {
	case "ctrl+c", "ctrl+d":
		globalPlayer.Stop()
		m.done = true
		return m, tea.Quit

	case "esc", "q", "Q":
		if m.screen == screenPodcasts {
			globalPlayer.Stop()
			m.done = true
			return m, tea.Quit
		}
		m.handleEscape()
		return m, nil

	case "1":
		m.screen = screenPodcasts
		return m, nil

	case "2", "f1", "F1":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenDownloadQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenPlayer
		return m, nil

	case "3", "f2", "F2":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenDownloadQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenPlayQueue
		m.pqGrabbed = false
		return m, nil

	case "4", "f3", "F3":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenDownloadQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenAdQueue
		m.adqGrabbed = false
		return m, nil

	case "5", "f5", "F5":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenDownloadQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenDownloadQueue
		return m, nil

	case "f4", "F4":
		if m.screen == screenEpisodeDetail {
			m.showEpisodePlayerPane = !m.showEpisodePlayerPane
		}
		return m, nil

	case "6":
		m.openTimelineViewer()
		return m, nil

	case "l", "L":
		if m.screen != screenLatestEpisodes {
			m.prevScreen = m.screen
			m.screen = screenLatestEpisodes
			m.latestIdx = 0
			m.latestScroll = 0
		} else {
			m.handleEscape()
		}
		return m, nil

	case "?", "h", "H":
		if m.screen != screenTranscript {
			m.showHelpModal = !m.showHelpModal
			return m, nil
		}
	}

	if m.screen == screenPlayer {
		switch s {
		case "esc", "q", "Q":
			m.handleEscape()
			return m, nil
		}
	}

	if m.screen == screenLatestEpisodes {
		return m.handleLatestViewKey(s)
	}

	if m.screen == screenDownloadQueue {
		return m.handleDownloadQueueKey(s)
	}

	if m.screen == screenTranscript {
		return m.handleTranscriptKey(s)
	}

	if m.screen == screenTimeline {
		return m.handleTimelineKey(s)
	}

	if m.screen == screenPlayQueue {
		return m.handlePlayQueueKey(s)
	}

	if m.screen == screenAdQueue {
		return m.handleAdQueueKey(s)
	}

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
		if globalPlayer.IsPaused {
			m.showPopup("Paused")
		} else if globalPlayer.IsPlaying {
			m.showPopup("Resumed")
		}

	case "right", "l", ">":
		if globalPlayer.IsPlaying {
			globalPlayer.Seek(30)
			m.showPopup("+30s (" + formatPlayerTime(globalPlayer.Position) + ")")
		}

	case "left", "h", "<":
		if globalPlayer.IsPlaying {
			globalPlayer.Seek(-30)
			m.showPopup("-30s (" + formatPlayerTime(globalPlayer.Position) + ")")
		}

	case "+", "=", "]":
		globalPlayer.VolumeUp()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.Volume))

	case "-", "_", "[":
		globalPlayer.VolumeDown()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.Volume))

	case "m", "M":
		globalPlayer.ToggleMute()
		if globalPlayer.Muted {
			m.showPopup("Muted")
		} else {
			m.showPopup("Unmuted")
		}

	case "s", "S":
		globalPlayer.CycleSpeaker()
		m.showPopup("Speaker: " + globalPlayer.CurrentSpeaker)

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
