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

type transcriptItem struct {
	startSec  float64
	endSec    float64
	timeFull  string
	timeShort string
	text      string
	isAd      bool
}

func parseTimestampSec(s string) float64 {
	s = strings.Trim(s, "[] ")
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		var m, sec float64
		_, _ = fmt.Sscanf(parts[0], "%f", &m)
		_, _ = fmt.Sscanf(parts[1], "%f", &sec)
		return m*60 + sec
	} else if len(parts) == 3 {
		var h, m, sec float64
		_, _ = fmt.Sscanf(parts[0], "%f", &h)
		_, _ = fmt.Sscanf(parts[1], "%f", &m)
		_, _ = fmt.Sscanf(parts[2], "%f", &sec)
		return h*3600 + m*60 + sec
	}
	return 0
}

func loadEpisodeAdIntervals(epPath string) [][2]float64 {
	base := strings.TrimSuffix(epPath, ".mp3")
	cutsFile := base + ".cuts.json"
	data, err := os.ReadFile(cutsFile)
	if err != nil {
		return nil
	}
	var cuts CutsData
	if err := json.Unmarshal(data, &cuts); err != nil {
		return nil
	}
	var intervals [][2]float64
	for _, mc := range cuts.MergedCutIntervals {
		if mc.End > mc.Start {
			intervals = append(intervals, [2]float64{mc.Start, mc.End})
		}
	}
	if len(intervals) == 0 {
		for _, ci := range cuts.CutIntervals {
			if ci.EndSec > ci.StartSec {
				intervals = append(intervals, [2]float64{ci.StartSec, ci.EndSec})
			}
		}
	}
	return intervals
}

func isSegmentInAd(st, en float64, adIntervals [][2]float64) bool {
	if len(adIntervals) == 0 {
		return false
	}
	for _, r := range adIntervals {
		if st < r[1] && en > r[0] {
			return true
		}
	}
	return false
}

func loadEpisodeTranscriptData(epPath string) ([]transcriptItem, []string, error) {
	base := strings.TrimSuffix(epPath, ".mp3")
	adIntervals := loadEpisodeAdIntervals(epPath)

	jsonFile := base + ".transcript.json"
	if data, err := os.ReadFile(jsonFile); err == nil {
		var td TranscriptionData
		if err := json.Unmarshal(data, &td); err == nil && len(td.Segments) > 0 {
			var items []transcriptItem
			var lines []string
			for _, seg := range td.Segments {
				st := formatTime(seg.Start)
				en := formatTime(seg.End)
				stShort := formatClock(seg.Start)
				tFull := fmt.Sprintf("[%s -> %s]", st, en)
				tShort := fmt.Sprintf("[%s]", stShort)
				txt := strings.TrimSpace(seg.Text)
				isAd := isSegmentInAd(seg.Start, seg.End, adIntervals)
				items = append(items, transcriptItem{
					startSec:  seg.Start,
					endSec:    seg.End,
					timeFull:  tFull,
					timeShort: tShort,
					text:      txt,
					isAd:      isAd,
				})
				lines = append(lines, fmt.Sprintf("%s %s", tFull, txt))
			}
			return items, lines, nil
		}
	}

	txtFile := base + ".transcript.txt"
	if data, err := os.ReadFile(txtFile); err == nil {
		text := strings.TrimSpace(string(data))
		rawLines := strings.Split(text, "\n")
		var items []transcriptItem
		var lines []string
		for _, l := range rawLines {
			trimmed := strings.TrimSpace(l)
			if trimmed == "" {
				continue
			}
			lines = append(lines, trimmed)
			if matches := timestampLineRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
				tFull := matches[1]
				txt := matches[2]
				inner := strings.Trim(tFull, "[] ")
				parts := strings.Split(inner, "->")
				stSec := 0.0
				enSec := 0.0
				stShort := ""
				if len(parts) == 2 {
					stSec = parseTimestampSec(strings.TrimSpace(parts[0]))
					enSec = parseTimestampSec(strings.TrimSpace(parts[1]))
					stShort = fmt.Sprintf("[%s]", strings.TrimSpace(parts[0]))
				} else {
					stShort = tFull
				}
				isAd := isSegmentInAd(stSec, enSec, adIntervals)
				items = append(items, transcriptItem{
					startSec:  stSec,
					endSec:    enSec,
					timeFull:  tFull,
					timeShort: stShort,
					text:      txt,
					isAd:      isAd,
				})
			} else {
				items = append(items, transcriptItem{
					text: trimmed,
				})
			}
		}
		return items, lines, nil
	}

	return nil, nil, fmt.Errorf("no transcript found")
}

func loadEpisodeTranscriptText(epPath string) (string, []string, error) {
	_, lines, err := loadEpisodeTranscriptData(epPath)
	if err != nil {
		return "", nil, err
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

	items, lines, err := loadEpisodeTranscriptData(ep.path)
	if err != nil || len(items) == 0 {
		m.showPopup("No transcript found for: " + truncate(ep.displayTitle(), 25))
		return
	}

	if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue && m.screen != screenTranscript && m.screen != screenTimeline {
		m.prevScreen = m.screen
	}
	m.transcriptItems = items
	m.transcriptLines = lines
	m.transcriptScroll = 0
	m.transcriptMatchIdx = 0
	m.transcriptLoadedFor = ep.path
	m.screen = screenTranscript
}

func (m *tuiModel) handleTranscriptKey(s string) (tea.Model, tea.Cmd) {
	maxVis := max(5, m.height-7)
	total := len(m.transcriptItems)

	switch s {
	case "tab":
		m.transcriptViewMode = (m.transcriptViewMode + 1) % 3
	case "/":
		m.searchMode = true
		m.searchQuery = ""
		m.transcriptMatchIdx = 0
	case "n", "enter":
		if m.searchQuery != "" {
			m.nextTranscriptMatch()
		}
	case "N":
		if m.searchQuery != "" {
			m.prevTranscriptMatch()
		}
	case "s", "S", "e", "E":
		m.exportTranscript()
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
	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.transcriptMatchIdx = 0
		} else {
			m.screen = m.prevScreen
		}
	case "q", "t", "T":
		m.screen = m.prevScreen
	}
	return m, nil
}

func parseTimestampSeconds(ts string) float64 {
	ts = strings.TrimSpace(ts)
	parts := strings.Split(ts, ":")
	if len(parts) == 2 {
		var mins, secs float64
		fmt.Sscanf(parts[0], "%f", &mins)
		fmt.Sscanf(parts[1], "%f", &secs)
		return mins*60 + secs
	} else if len(parts) == 3 {
		var hrs, mins, secs float64
		fmt.Sscanf(parts[0], "%f", &hrs)
		fmt.Sscanf(parts[1], "%f", &mins)
		fmt.Sscanf(parts[2], "%f", &secs)
		return hrs*3600 + mins*60 + secs
	}
	return 0
}

func (m *tuiModel) exportTranscript() {
	epPath := m.transcriptLoadedFor
	if epPath == "" && m.podIdx < len(m.podcasts) {
		eps := m.filteredEpisodes()
		if m.epIdx < len(eps) {
			epPath = eps[m.epIdx].path
		}
	}
	if epPath == "" {
		m.showToast("No episode transcript available for export", ToastError)
		return
	}

	base := strings.TrimSuffix(epPath, ".mp3")
	srtFile := base + ".srt"
	txtFile := base + ".transcript.txt"
	jsonFile := base + ".transcript.json"

	if raw, err := os.ReadFile(jsonFile); err == nil {
		var td TranscriptionData
		if err := json.Unmarshal(raw, &td); err == nil {
			dur := 0.0
			if m.podIdx < len(m.podcasts) && m.epIdx < len(m.podcasts[m.podIdx].episodes) {
				dur = m.podcasts[m.podIdx].episodes[m.epIdx].duration
			}
			convertJSONToSRT(jsonFile, &td, srtFile, true)
			convertJSONToTXT(jsonFile, &td, dur, txtFile, true)
			m.showToast("Exported transcript to .srt and .txt", ToastSuccess)
			return
		}
	}

	if len(m.transcriptLines) == 0 {
		m.showToast("No transcript content to export", ToastError)
		return
	}

	txtContent := strings.Join(m.transcriptLines, "\n") + "\n"
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		m.showToast("Failed to save .txt: "+err.Error(), ToastError)
		return
	}

	var srtEntries []string
	idx := 1
	for _, line := range m.transcriptLines {
		if sub := timestampLineRegex.FindStringSubmatch(line); len(sub) == 3 {
			ts := strings.Trim(sub[1], "[] ")
			parts := strings.Split(ts, "->")
			if len(parts) == 2 {
				stSec := parseTimestampSeconds(parts[0])
				enSec := parseTimestampSeconds(parts[1])
				st := formatSRTTime(stSec)
				en := formatSRTTime(enSec)
				text := strings.TrimSpace(sub[2])
				srtEntries = append(srtEntries, fmt.Sprintf("%d\n%s --> %s\n%s\n", idx, st, en, text))
				idx++
			}
		}
	}

	if len(srtEntries) > 0 {
		_ = os.WriteFile(srtFile, []byte(strings.Join(srtEntries, "\n")), 0644)
	}

	m.showToast("Exported transcript to .srt and .txt", ToastSuccess)
}

func highlightTranscriptText(text, query string, isCurrent bool) string {
	if query == "" {
		return text
	}
	idx := strings.Index(strings.ToLower(text), strings.ToLower(query))
	if idx == -1 {
		return text
	}
	before := text[:idx]
	match := text[idx : idx+len(query)]
	after := text[idx+len(query):]

	style := tuiYellowStyle.Bold(true)
	if isCurrent {
		style = tuiSelectedStyle.Bold(true)
	}
	return before + style.Render(match) + highlightTranscriptText(after, query, isCurrent)
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
	subInfo += fmt.Sprintf(" • %d lines", len(m.transcriptItems))
	out.WriteString("    " + tuiSubtitleStyle.Render(subInfo) + "\n")

	if len(m.transcriptItems) == 0 && len(m.transcriptLines) > 0 {
		for _, l := range m.transcriptLines {
			if sub := timestampLineRegex.FindStringSubmatch(l); len(sub) == 3 {
				m.transcriptItems = append(m.transcriptItems, transcriptItem{
					timeFull:  sub[1],
					timeShort: sub[1],
					text:      sub[2],
				})
			} else {
				m.transcriptItems = append(m.transcriptItems, transcriptItem{
					text: l,
				})
			}
		}
	}

	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

	maxVis := max(5, m.height-7)
	total := len(m.transcriptItems)
	if total == 0 {
		out.WriteString("\n  " + tuiDimStyle.Render("No transcript lines available.") + "\n\n")
		out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")
		out.WriteString(tuiDimStyle.Render("  Esc/q/t Back") + "\n")
		return out.String()
	}

	if m.transcriptScroll > max(0, total-maxVis) {
		m.transcriptScroll = max(0, total-maxVis)
	}
	if m.transcriptScroll < 0 {
		m.transcriptScroll = 0
	}

	targetW := min(80, max(20, m.width-4))
	end := min(total, m.transcriptScroll+maxVis)
	matches := m.matchingTranscriptIndices()
	digits := max(3, len(fmt.Sprintf("%d", total)))

	for i := m.transcriptScroll; i < end; i++ {
		item := m.transcriptItems[i]
		isCurrent := len(matches) > 0 && m.transcriptMatchIdx < len(matches) && i == matches[m.transcriptMatchIdx]

		switch m.transcriptViewMode {
		case 0:
			ts := item.timeFull
			availTextW := max(10, targetW-len(ts)-3)
			truncText := truncate(item.text, availTextW)
			renderedText := ""
			if m.searchQuery != "" {
				renderedText = highlightTranscriptText(truncText, m.searchQuery, isCurrent)
			} else if item.isAd {
				renderedText = tuiAdStrikeStyle.Render(truncText)
			} else {
				renderedText = tuiTranscriptTextStyle.Render(truncText)
			}
			prefix := "  "
			if isCurrent {
				prefix = "▶ "
			}
			if item.isAd {
				out.WriteString(prefix + tuiAdStrikeStyle.Render(ts) + " " + renderedText + "\n")
			} else {
				out.WriteString(prefix + tuiYellowStyle.Render(ts) + " " + renderedText + "\n")
			}

		case 1:
			ts := item.timeShort
			if ts == "" {
				ts = item.timeFull
			}
			availTextW := max(10, targetW-len(ts)-3)
			truncText := truncate(item.text, availTextW)
			renderedText := ""
			if m.searchQuery != "" {
				renderedText = highlightTranscriptText(truncText, m.searchQuery, isCurrent)
			} else if item.isAd {
				renderedText = tuiAdStrikeStyle.Render(truncText)
			} else {
				renderedText = tuiTranscriptTextStyle.Render(truncText)
			}
			prefix := "  "
			if isCurrent {
				prefix = "▶ "
			}
			if item.isAd {
				out.WriteString(prefix + tuiAdStrikeStyle.Render(ts) + " " + renderedText + "\n")
			} else {
				out.WriteString(prefix + tuiYellowStyle.Render(ts) + " " + renderedText + "\n")
			}

		case 2:
			lineNumPrefix := fmt.Sprintf("%*d │ ", digits, i+1)
			availTextW := max(10, targetW-digits-6)
			truncText := truncate(item.text, availTextW)
			renderedText := ""
			if m.searchQuery != "" {
				renderedText = highlightTranscriptText(truncText, m.searchQuery, isCurrent)
			} else if item.isAd {
				renderedText = tuiAdStrikeStyle.Render(truncText)
			} else {
				renderedText = tuiTranscriptTextStyle.Render(truncText)
			}
			prefix := "  "
			if isCurrent {
				prefix = "▶ "
			}
			if item.isAd {
				out.WriteString(prefix + tuiDimStyle.Render(lineNumPrefix) + renderedText + "\n")
			} else {
				out.WriteString(prefix + tuiDimStyle.Render(lineNumPrefix) + renderedText + "\n")
			}
		}
	}

	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

	scrollPct := 0
	if total > 0 {
		scrollPct = int(float64(min(end, total)) / float64(total) * 100)
	}

	arrowHelp := "Tab Short Time"
	if m.transcriptViewMode == 1 {
		arrowHelp = "Tab Line Nums"
	} else if m.transcriptViewMode == 2 {
		arrowHelp = "Tab Time Arrows"
	}

	var helpText string
	if m.searchQuery != "" {
		matchInfo := fmt.Sprintf("[%d/%d matches]", m.transcriptMatchIdx+1, len(matches))
		if len(matches) == 0 {
			matchInfo = "[0 matches]"
		}
		helpText = fmt.Sprintf("Enter/n Next │ N Prev │ / Search │ Esc Clear │ s/e Export │ %s", matchInfo)
	} else {
		helpText = fmt.Sprintf("↑/↓ Scroll │ %s │ / Search │ s/e Export │ Esc/q/t Back │ [%d%%]", arrowHelp, scrollPct)
	}
	out.WriteString(tuiDimStyle.Render("  " + helpText + "\n"))

	return out.String()
}
