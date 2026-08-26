package main

import (
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
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
	case "enter":
		m.searchMode = false
		if m.screen == screenPodcasts {
			m.podIdx = 0
			m.podScroll = 0
		} else if m.screen == screenPodcastDetail {
			m.epIdx = 0
			m.epScroll = 0
		}
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
		}
	}
	return m, nil
}
