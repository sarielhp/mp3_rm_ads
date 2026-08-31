package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) handlePlayQueueKey(s string) (tea.Model, tea.Cmd) {
	unified := globalPlayer.GetUnifiedQueue()
	switch s {
	case "up", "k":
		if m.pqGrabbed && m.pqIdx > 0 {
			globalPlayer.MoveTrack(m.pqIdx, m.pqIdx-1)
			m.pqIdx--
		} else if m.pqIdx > 0 {
			m.pqIdx--
		}
		if m.pqIdx < m.pqScroll {
			m.pqScroll = m.pqIdx
		}
	case "down", "j":
		if m.pqGrabbed && m.pqIdx < len(unified)-1 {
			globalPlayer.MoveTrack(m.pqIdx, m.pqIdx+1)
			m.pqIdx++
		} else if m.pqIdx < len(unified)-1 {
			m.pqIdx++
		}
		maxVis := m.visibleLines(3)
		if m.pqIdx >= m.pqScroll+maxVis {
			m.pqScroll = m.pqIdx - maxVis + 1
		}
	case " ", "g", "G":
		m.pqGrabbed = !m.pqGrabbed
		if m.pqGrabbed {
			m.showPopup("Track grabbed (use ↑/↓ to move)")
		} else {
			m.showPopup("Track placed")
		}
	case "d", "D", "x", "X":
		if len(unified) > 0 && m.pqIdx < len(unified) {
			globalPlayer.RemoveTrack(m.pqIdx)
			m.showPopup("Removed from queue")
			if m.pqIdx >= len(unified)-1 && m.pqIdx > 0 {
				m.pqIdx--
			}
		}
	case "c", "C":
		globalPlayer.ClearQueue()
		m.showPopup("Queue cleared")
	case "enter":
		if len(unified) > 0 && m.pqIdx < len(unified) {
			globalPlayer.PlayQueueIndex(m.pqIdx)
			m.screen = screenPlayer
		}
	case "esc":
		m.handleEscape()
	}
	return m, nil
}

func (m *tuiModel) handleAdQueueKey(s string) (tea.Model, tea.Cmd) {
	items := getAllAdQueueItems(m.podcasts, m.queue)
	switch s {
	case "up", "k":
		if m.adqIdx > 0 {
			m.adqIdx--
		}
		if m.adqIdx < m.adqScroll {
			m.adqScroll = m.adqIdx
		}
	case "down", "j":
		if m.adqIdx < len(items)-1 {
			m.adqIdx++
		}
		maxVis := m.visibleLines(3)
		if m.adqIdx >= m.adqScroll+maxVis {
			m.adqScroll = m.adqIdx - maxVis + 1
		}
	case " ", "g", "G":
		m.adqGrabbed = !m.adqGrabbed
		if m.adqGrabbed {
			m.showPopup("Item grabbed (use ↑/↓ to move)")
		} else {
			m.showPopup("Item placed")
		}
	case "d", "D", "x", "X":
		if len(items) > 0 && m.adqIdx < len(items) {
			item := items[m.adqIdx]
			entries := m.queue[item.PodcastDir]
			for i, e := range entries {
				if e == item.Filename {
					m.queue[item.PodcastDir] = append(entries[:i], entries[i+1:]...)
					break
				}
			}
			if m.bk != nil && m.bk.SaveQueue != nil {
				m.bk.SaveQueue(item.PodcastDir, m.queue[item.PodcastDir])
			}
			m.showPopup("Removed from ad queue")
			if m.adqIdx >= len(items)-1 && m.adqIdx > 0 {
				m.adqIdx--
			}
		}
	case "esc":
		m.handleEscape()
	}
	return m, nil
}
