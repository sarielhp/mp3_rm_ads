package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) drawEpisodeDetail() string {
	out := &strings.Builder{}

	if m.podIdx >= len(m.podcasts) {
		m.screen = screenPodcasts
		return m.drawPodcastsList()
	}

	pod := m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		m.screen = screenPodcastDetail
		return m.drawPodcastDetail()
	}

	ep := eps[m.epIdx]
	absEp := ep.absData
	if absEp == nil {
		if cachedDet, err := loadEpisodeDetails(pod.dir, ep.filename); err == nil && cachedDet != nil {
			absEp = cachedDet.RawABS
			if absEp == nil && (cachedDet.Description != "" || cachedDet.Subtitle != "") {
				absEp = &absEpisode{
					Title:       cachedDet.Title,
					Subtitle:    cachedDet.Subtitle,
					Description: cachedDet.Description,
					EpisodeType: cachedDet.EpisodeType,
				}
			}
		}
	}

	displayHeader := ep.displayTitle()
	author := pod.displayAuthor()

	d := ep.displayDate()
	dateStr := ""
	if !d.IsZero() {
		dateStr = d.Format("2006-01-02")
	}

	totalDurStr := "--:--"
	if ep.duration > 0 {
		totalDurStr = formatPlayerTime(ep.duration)
	}

	// Split pane if width >= 70
	if m.width >= 70 {
		leftW := (m.width - 5) / 2
		rightW := m.width - 5 - leftW
		maxLines := max(10, m.height-4)

		var leftLines []string
		var rightLines []string

		// Left Pane: Episode Header & Badges
		leftLines = append(leftLines, tuiTitleStyle.Render(truncate(displayName(displayHeader), leftW-2)))
		subStr := displayName(pod.name)
		if author != "" {
			subStr = displayName(author) + " • " + subStr
		}
		leftLines = append(leftLines, tuiSubtitleStyle.Render(truncate(subStr, leftW-2)))

		// Badges row
		badgeRow := ""
		if ep.hasAdsRemoved {
			badgeRow += tuiBadgeAdFree.Render("✓ Ad-Free") + " "
		} else {
			badgeRow += tuiBadgeHasAds.Render("Has Ads") + " "
		}
		badgeRow += tuiBadgeDuration.Render(totalDurStr) + " "
		if dateStr != "" {
			badgeRow += tuiStatStyle.Render(dateStr)
		}
		leftLines = append(leftLines, badgeRow)
		leftLines = append(leftLines, tuiDividerStyle.Render(strings.Repeat("─", leftW-2)))

		// Description Header with scroll info
		descRaw := ""
		if absEp != nil && absEp.Description != "" {
			descRaw = absEp.Description
		} else if absEp != nil && absEp.Subtitle != "" {
			descRaw = absEp.Subtitle
		}

		descClean := strings.TrimSpace(renderHTML(descRaw))
		var descWrapped []string
		if len(descClean) > 0 {
			for _, line := range strings.Split(descClean, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					descWrapped = append(descWrapped, "")
				} else {
					descWrapped = append(descWrapped, wrapText(line, leftW-4)...)
				}
			}
		} else {
			descWrapped = append(descWrapped, tuiDimStyle.Render("No description available."))
		}

		availDescLines := max(3, maxLines-len(leftLines)-2)
		if m.descScroll > len(descWrapped)-availDescLines {
			m.descScroll = max(0, len(descWrapped)-availDescLines)
		}
		if m.descScroll < 0 {
			m.descScroll = 0
		}

		descHeader := "Show Notes"
		if len(descWrapped) > availDescLines {
			descHeader += fmt.Sprintf(" [%d-%d of %d (Scroll: ↑/↓)]", m.descScroll+1, min(len(descWrapped), m.descScroll+availDescLines), len(descWrapped))
		}
		leftLines = append(leftLines, tuiLabelStyle.Render(descHeader))

		endDesc := min(len(descWrapped), m.descScroll+availDescLines)
		for idx := m.descScroll; idx < endDesc; idx++ {
			leftLines = append(leftLines, descWrapped[idx])
		}

		// Right Pane: Audio Player
		rightLines = append(rightLines, tuiSectionTitle.Render(" AUDIO PLAYER "))

		// 1. Now Playing Status
		if globalPlayer.Current != nil {
			statusBadge := tuiPlayerPlaying.Render("▶ PLAYING")
			if globalPlayer.IsPaused {
				statusBadge = tuiPlayerPaused.Render("⏸ PAUSED")
			}
			rightLines = append(rightLines, statusBadge+" "+tuiSelectedStyle.Render(" "+truncate(displayName(globalPlayer.Current.Title), rightW-16)+" "))
			rightLines = append(rightLines, tuiSubtextStyle.Render("  in "+displayName(globalPlayer.Current.Podcast)))
			rightLines = append(rightLines, "  "+tuiCyanStyle.Render(globalPlayer.RenderProgressBar(rightW-4)))
		} else {
			rightLines = append(rightLines, tuiPlayerStopped.Render("⏹ STOPPED")+" "+tuiDimStyle.Render("Press 'p' or Enter to play"))
			rightLines = append(rightLines, tuiDimStyle.Render("  [────────────────────] 00:00 / "+totalDurStr))
		}

		rightLines = append(rightLines, "")

		// 2. Output Device & Volume
		rightLines = append(rightLines, tuiLabelStyle.Render("Audio Output & Volume:"))
		speaker := globalPlayer.CurrentSpeaker
		if speaker == "" {
			speaker = "Default Audio Sink"
		}
		rightLines = append(rightLines, fmt.Sprintf("  Speaker: %s %s", tuiYellowStyle.Render(speaker), tuiDimStyle.Render("('s' switch)")))
		rightLines = append(rightLines, fmt.Sprintf("  Volume:  %s %s", tuiGreenStyle.Render(globalPlayer.RenderVolumeBar(rightW-20)), tuiDimStyle.Render("(+/- vol, 'm' mute)")))

		rightLines = append(rightLines, "")

		// 3. Playback Queue
		queueTitle := fmt.Sprintf("Playback Queue (%d queued)", len(globalPlayer.Queue))
		rightLines = append(rightLines, tuiLabelStyle.Render(queueTitle))
		if len(globalPlayer.Queue) > 0 {
			for qIdx, qTrack := range globalPlayer.Queue {
				if qIdx >= 3 {
					rightLines = append(rightLines, tuiDimStyle.Render(fmt.Sprintf("  ... and %d more", len(globalPlayer.Queue)-3)))
					break
				}
				qStr := fmt.Sprintf("  %d. %s", qIdx+1, truncate(displayName(qTrack.Title), rightW-8))
				rightLines = append(rightLines, tuiSubtextStyle.Render(qStr))
			}
			rightLines = append(rightLines, tuiDimStyle.Render("  ('n' next track, 'c' clear queue)"))
		} else {
			rightLines = append(rightLines, tuiDimStyle.Render("  Queue is empty."))
		}

		rightLines = append(rightLines, "")
		rightLines = append(rightLines, tuiDividerStyle.Render(strings.Repeat("─", rightW-2)))

		// 4. Key Controls Legend
		rightLines = append(rightLines, tuiDimStyle.Render("Space Play/Pause │ p Play Episode │ Esc Back │ q Quit"))
		rightLines = append(rightLines, tuiDimStyle.Render("←/→   -30s / +30s │ +/- Volume     │ s Speaker"))
		rightLines = append(rightLines, tuiDimStyle.Render("↑/↓   Scroll Notes│ n Next in Queue│ m Mute"))

		// Join Panes
		totalRows := max(len(leftLines), len(rightLines))
		for k := 0; k < totalRows; k++ {
			lPart := ""
			if k < len(leftLines) {
				lPart = leftLines[k]
			}
			lPad := max(0, leftW-visibleRuneCount(lPart))
			fullL := lPart + strings.Repeat(" ", lPad)

			rPart := ""
			if k < len(rightLines) {
				rPart = rightLines[k]
			}

			out.WriteString(fullL + tuiDividerStyle.Render(" │ ") + rPart + "\n")
		}

		return out.String()
	}

	// Narrow terminal fallback
	out.WriteString(tuiTitleStyle.Render("  " + displayName(displayHeader)))
	out.WriteByte('\n')
	out.WriteString(tuiSubtitleStyle.Render("  " + displayName(pod.name)))
	out.WriteByte('\n')
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", m.width-4)))
	out.WriteByte('\n')

	if globalPlayer.Current != nil {
		status := "▶ PLAYING"
		if globalPlayer.IsPaused {
			status = "⏸ PAUSED"
		}
		out.WriteString(fmt.Sprintf("  %s %s\n", tuiStatStyle.Render(status), globalPlayer.RenderProgressBar(m.width-15)))
	}

	out.WriteString(tuiHelpStyle.Render("  Space: Play/Pause │ p: Play │ ←/→: 30s │ +/-: Vol │ Esc: Back\n"))
	return out.String()
}

func wrapText(text string, maxW int) []string {
	if maxW <= 0 {
		maxW = 40
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	curLine := words[0]
	for _, w := range words[1:] {
		if len([]rune(curLine))+1+len([]rune(w)) <= maxW {
			curLine += " " + w
		} else {
			lines = append(lines, curLine)
			curLine = w
		}
	}
	if len(curLine) > 0 {
		lines = append(lines, curLine)
	}
	return lines
}

func visibleRuneCount(s string) int {
	return lipgloss.Width(s)
}
