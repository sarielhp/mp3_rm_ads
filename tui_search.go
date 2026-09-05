package main

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func fuzzyMatch(pattern, text string) (bool, int, []int) {
	if pattern == "" {
		return true, 0, nil
	}

	pRunes := []rune(strings.ToLower(pattern))
	tRunes := []rune(strings.ToLower(text))

	if len(pRunes) > len(tRunes) {
		return false, 0, nil
	}

	pIdx := 0
	score := 0
	var matchedIndices []int

	for tIdx, tRune := range tRunes {
		if pIdx < len(pRunes) && tRune == pRunes[pIdx] {
			matchedIndices = append(matchedIndices, tIdx)
			pIdx++
			score += 10
			if tIdx == 0 || unicode.IsSpace(rune(text[tIdx-1])) || text[tIdx-1] == '-' || text[tIdx-1] == '_' {
				score += 15
			}
		}
	}

	if pIdx == len(pRunes) {
		return true, score, matchedIndices
	}
	return false, 0, nil
}

func renderHighlightedText(text string, matchedIndices []int, baseStyle, highlightStyle lipgloss.Style) string {
	if len(matchedIndices) == 0 {
		return baseStyle.Render(text)
	}

	matchMap := make(map[int]bool)
	for _, idx := range matchedIndices {
		matchMap[idx] = true
	}

	var out strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		if matchMap[i] {
			out.WriteString(highlightStyle.Render(string(r)))
		} else {
			out.WriteString(baseStyle.Render(string(r)))
		}
	}
	return out.String()
}

func (m *tuiModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenTranscript:
		return m.handleTranscriptSearchKey(msg)
	case screenPodcastDetail:
		return m.handlePodcastDetailSearchKey(msg)
	default:
		return m.handlePodcastsSearchKey(msg)
	}
}

func (m *tuiModel) handlePodcastsSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		if m.podIdx >= len(m.podcasts) {
			m.podIdx = max(0, len(m.podcasts)-1)
		}
	case "enter":
		m.searchMode = false
		pods := m.filteredPodcasts()
		if len(pods) > 0 && m.podIdx < len(pods) {
			selectedPod := pods[m.podIdx]
			for i, p := range m.podcasts {
				if p.dir == selectedPod.dir {
					m.podIdx = i
					break
				}
			}
			m.searchQuery = ""
			m.screen = screenPodcastDetail
			m.epIdx = 0
			m.epScroll = 0
		}
	case "up", "ctrl+p", "ctrl+k":
		m.handleUp()
	case "down", "ctrl+n", "ctrl+j":
		m.handleDown()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.podIdx = 0
			m.podScroll = 0
		}
	default:
		if isPrintableKey(msg) {
			m.searchQuery += keyString(msg)
			m.podIdx = 0
			m.podScroll = 0
		}
	}
	return m, nil
}

func (m *tuiModel) handlePodcastDetailSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		if m.podIdx < len(m.podcasts) && m.epIdx >= len(m.podcasts[m.podIdx].episodes) {
			m.epIdx = max(0, len(m.podcasts[m.podIdx].episodes)-1)
		}
	case "enter":
		m.searchMode = false
		eps := m.filteredEpisodes()
		if m.podIdx < len(m.podcasts) && len(eps) > 0 && m.epIdx < len(eps) {
			selectedEp := eps[m.epIdx]
			pod := &m.podcasts[m.podIdx]
			for i, e := range pod.episodes {
				if e.path == selectedEp.path {
					m.epIdx = i
					break
				}
			}
			m.searchQuery = ""
			m.screen = screenEpisodeDetail
			m.descScroll = 0
			ep := &pod.episodes[m.epIdx]
			if !ep.durationDone {
				idx := m.epIdx
				return m, func() tea.Msg {
					dur := m.bk.GetDuration(ep.path)
					return episodeDurationMsg{idx: idx, duration: dur}
				}
			}
		}
	case "up", "ctrl+p", "ctrl+k":
		m.handleUp()
	case "down", "ctrl+n", "ctrl+j":
		m.handleDown()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.epIdx = 0
			m.epScroll = 0
		}
	default:
		if isPrintableKey(msg) {
			m.searchQuery += keyString(msg)
			m.epIdx = 0
			m.epScroll = 0
		}
	}
	return m, nil
}

func (m *tuiModel) handleTranscriptSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		m.transcriptMatchIdx = 0
	case "enter":
		m.searchMode = false
		m.nextTranscriptMatch()
	case "up", "ctrl+p", "ctrl+k":
		m.prevTranscriptMatch()
	case "down", "ctrl+n", "ctrl+j":
		m.nextTranscriptMatch()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.transcriptMatchIdx = 0
			m.updateTranscriptSearch()
		}
	default:
		if isPrintableKey(msg) {
			m.searchQuery += keyString(msg)
			m.transcriptMatchIdx = 0
			m.updateTranscriptSearch()
		}
	}
	return m, nil
}

func isPrintableKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeySpace {
		return true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		return true
	}
	if len(msg.String()) == 1 {
		return true
	}
	return false
}

func keyString(msg tea.KeyMsg) string {
	if msg.Type == tea.KeySpace {
		return " "
	}
	if len(msg.Runes) > 0 {
		return string(msg.Runes)
	}
	return msg.String()
}

func (m *tuiModel) matchingTranscriptIndices() []int {
	if m.searchQuery == "" || len(m.transcriptLines) == 0 {
		return nil
	}
	q := strings.ToLower(m.searchQuery)
	var indices []int
	for i, line := range m.transcriptLines {
		if strings.Contains(strings.ToLower(line), q) {
			indices = append(indices, i)
		}
	}
	if len(indices) == 0 {
		for i, line := range m.transcriptLines {
			if matched, _, _ := fuzzyMatch(m.searchQuery, line); matched {
				indices = append(indices, i)
			}
		}
	}
	return indices
}

func (m *tuiModel) updateTranscriptSearch() {
	matches := m.matchingTranscriptIndices()
	if len(matches) > 0 {
		m.transcriptMatchIdx = 0
		m.scrollToTranscriptLine(matches[0])
	}
}

func (m *tuiModel) nextTranscriptMatch() {
	matches := m.matchingTranscriptIndices()
	if len(matches) == 0 {
		if m.searchQuery != "" {
			m.showPopup(fmt.Sprintf("No matches for %q", m.searchQuery))
		}
		return
	}
	if m.transcriptMatchIdx < 0 || m.transcriptMatchIdx >= len(matches)-1 {
		m.transcriptMatchIdx = 0
	} else {
		m.transcriptMatchIdx++
	}
	m.scrollToTranscriptLine(matches[m.transcriptMatchIdx])
	m.showPopup(fmt.Sprintf("Match %d of %d", m.transcriptMatchIdx+1, len(matches)))
}

func (m *tuiModel) prevTranscriptMatch() {
	matches := m.matchingTranscriptIndices()
	if len(matches) == 0 {
		if m.searchQuery != "" {
			m.showPopup(fmt.Sprintf("No matches for %q", m.searchQuery))
		}
		return
	}
	if m.transcriptMatchIdx <= 0 || m.transcriptMatchIdx >= len(matches) {
		m.transcriptMatchIdx = len(matches) - 1
	} else {
		m.transcriptMatchIdx--
	}
	m.scrollToTranscriptLine(matches[m.transcriptMatchIdx])
	m.showPopup(fmt.Sprintf("Match %d of %d", m.transcriptMatchIdx+1, len(matches)))
}

func (m *tuiModel) scrollToTranscriptLine(lineIdx int) {
	maxVis := max(5, m.height-7)
	total := len(m.transcriptLines)
	if total == 0 {
		m.transcriptScroll = 0
		return
	}
	target := max(0, lineIdx-maxVis/3)
	maxScroll := max(0, total-maxVis)
	if target > maxScroll {
		target = maxScroll
	}
	m.transcriptScroll = target
}
