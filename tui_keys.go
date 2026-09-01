package main

import (
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

	default:
		return m.handleKeyPart2(s)
	}

}
