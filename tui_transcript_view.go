package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var timestampLineRegex = regexp.MustCompile(`^(\[\d{1,2}:\d{2}(?:\.\d+)?\s*->\s*\d{1,2}:\d{2}(?:\.\d+)?\])\s*(.*)$`)

func loadEpisodeTranscriptText(epPath string) (string, []string, error) {
	base := strings.TrimSuffix(epPath, ".mp3")
	txtFile := base + ".transcript.txt"
	if data, err := os.ReadFile(txtFile); err == nil {
		text := strings.TrimSpace(string(data))
		lines := strings.Split(text, "\n")
		return text, lines, nil
	}

	jsonFile := base + ".transcript.json"
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return "", nil, fmt.Errorf("no transcript found")
	}

	var td TranscriptionData
	if err := json.Unmarshal(data, &td); err != nil {
		return "", nil, err
	}

	var lines []string
	if len(td.Segments) == 0 && td.Text != "" {
		lines = append(lines, td.Text)
	} else {
		for _, seg := range td.Segments {
			st := formatTime(seg.Start)
			en := formatTime(seg.End)
			text := strings.TrimSpace(seg.Text)
			lines = append(lines, fmt.Sprintf("[%s -> %s] %s", st, en, text))
		}
	}

	return strings.Join(lines, "\n"), lines, nil
}

func (m *tuiModel) openTranscriptViewer() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]

	_, lines, err := loadEpisodeTranscriptText(ep.path)
	if err != nil || len(lines) == 0 {
		m.showPopup("No transcript found for: " + truncate(ep.displayTitle(), 25))
		return
	}

	if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenTranscript && m.screen != screenTimeline {
		m.prevScreen = m.screen
	}
	m.transcriptLines = lines
	m.transcriptScroll = 0
	m.transcriptLoadedFor = ep.path
	m.screen = screenTranscript
}

func (m *tuiModel) handleTranscriptKey(s string) (tea.Model, tea.Cmd) {
	maxVis := max(5, m.height-7)
	total := len(m.transcriptLines)

	switch s {
	case "up", "k":
		if m.transcriptScroll > 0 {
			m.transcriptScroll--
		}
	case "down", "j":
		if m.transcriptScroll < max(0, total-maxVis) {
			m.transcriptScroll++
		}
	case "pgup", "b", "ctrl+u":
		m.transcriptScroll = max(0, m.transcriptScroll-maxVis)
	case "pgdown", "f", "space", "ctrl+d":
		m.transcriptScroll = min(max(0, total-maxVis), m.transcriptScroll+maxVis)
	case "g", "home":
		m.transcriptScroll = 0
	case "G", "end":
		m.transcriptScroll = max(0, total-maxVis)
	case "esc", "q", "t", "T":
		m.screen = m.prevScreen
	}
	return m, nil
}

func (m *tuiModel) drawTranscriptScreen() string {
	out := &strings.Builder{}

	titleStr := "Episode Transcript"
	podName := ""
	durStr := ""
	if m.podIdx < len(m.podcasts) {
		pod := m.podcasts[m.podIdx]
		podName = pod.name
		eps := m.filteredEpisodes()
		if m.epIdx < len(eps) {
			ep := eps[m.epIdx]
			titleStr = ep.displayTitle()
			if ep.duration > 0 {
				durStr = formatPlayerTime(ep.duration)
			}
		}
	}

	banner := tuiHeaderBanner.Render(" TRANSCRIPT ")
	out.WriteString("  " + banner + "  " + tuiTitleStyle.Render(truncate(displayName(titleStr), max(10, m.width-20))) + "\n")

	subInfo := displayName(podName)
	if durStr != "" {
		subInfo += " • " + durStr
	}
	subInfo += fmt.Sprintf(" • %d lines", len(m.transcriptLines))
	out.WriteString("    " + tuiSubtitleStyle.Render(subInfo) + "\n")

	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth) + "\n"))

	maxVis := max(5, m.height-7)
	total := len(m.transcriptLines)
	if total == 0 {
		out.WriteString("\n  " + tuiDimStyle.Render("No transcript lines available.") + "\n\n")
		out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth) + "\n"))
		out.WriteString(tuiDimStyle.Render("  Esc/q/t Back\n"))
		return out.String()
	}

	if m.transcriptScroll > max(0, total-maxVis) {
		m.transcriptScroll = max(0, total-maxVis)
	}
	if m.transcriptScroll < 0 {
		m.transcriptScroll = 0
	}

	availW := max(20, m.width-6)
	end := min(total, m.transcriptScroll+maxVis)

	for i := m.transcriptScroll; i < end; i++ {
		line := m.transcriptLines[i]
		if matches := timestampLineRegex.FindStringSubmatch(line); len(matches) == 3 {
			ts := tuiYellowStyle.Render(matches[1])
			text := tuiSubtextStyle.Render(" " + truncate(matches[2], max(10, availW-len(matches[1])-2)))
			out.WriteString("  " + ts + text + "\n")
		} else {
			out.WriteString("  " + tuiSubtextStyle.Render(truncate(line, availW)) + "\n")
		}
	}

	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth) + "\n"))

	scrollPct := 0
	if total > 0 {
		scrollPct = int(float64(min(end, total)) / float64(total) * 100)
	}
	helpText := fmt.Sprintf("↑/↓ Scroll │ PgUp/PgDn Page │ g/G Top/Bottom │ Esc/q/t Back │ [%d%%]", scrollPct)
	out.WriteString(tuiDimStyle.Render("  " + helpText + "\n"))

	return out.String()
}
