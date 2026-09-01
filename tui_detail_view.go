package main

import (
	"fmt"
	"strings"
)

func (m *tuiModel) drawPodcastDetail() string {
	out := &strings.Builder{}

	if isKittyTerminal() {
		out.WriteString(kittyClearGraphics())
	}

	if m.podIdx >= len(m.podcasts) {
		m.screen = screenPodcasts
		return m.drawPodcastsList()
	}

	pod := m.podcasts[m.podIdx]

	title := tuiTitleStyle.Render("  " + displayName(pod.name))
	out.WriteString(title + "\n")

	eps := m.filteredEpisodes()

	total := len(eps)
	done := 0
	for _, e := range eps {
		if e.hasAdsRemoved {
			done++
		}
	}
	queued := len(m.queue[pod.dir])

	txCount := pod.transcribedCount()
	policyLabel := adRemovalModeLabel(pod.config.AdRemoval)
	dlPolicyLabel := downloadPolicyLabel(pod.config.DownloadPolicy, pod.config.DownloadK)
	selCount := len(m.selectedEpisodes)
	selInfo := ""
	if selCount > 0 {
		selInfo = fmt.Sprintf(" • %d selected ('a' queue ads, 'p' play)", selCount)
	}
	statPill := tuiStatStyle.Render(fmt.Sprintf("  %d episodes, %d ad-free, %d transcribed, %d queued • Ads: %s • DL: %s%s", total, done, txCount, queued, policyLabel, dlPolicyLabel, selInfo))
	out.WriteString(statPill + "\n")

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

	overhead := 8
	if globalPlayer.View().Has {
		overhead = 10
	}
	maxVis := max(3, m.height-overhead)

	if len(eps) > 0 {
		if m.epIdx >= len(eps) {
			m.epIdx = len(eps) - 1
		}
		if m.epIdx < 0 {
			m.epIdx = 0
		}
		if m.epIdx < m.epScroll {
			m.epScroll = m.epIdx
		}
		if m.epIdx >= m.epScroll+maxVis {
			m.epScroll = m.epIdx - maxVis + 1
		}
		if m.epScroll > max(0, len(eps)-maxVis) {
			m.epScroll = max(0, len(eps)-maxVis)
		}
		if m.epScroll < 0 {
			m.epScroll = 0
		}
	}

	start := m.epScroll
	end := min(len(eps), start+maxVis)

	queueEntries := m.queue[pod.dir]
	isQueued := func(filename string) bool {
		for _, q := range queueEntries {
			if q == filename {
				return true
			}
		}
		return false
	}

	renderEpRow := func(i int) {
		ep := eps[i]
		displayNameStr := ep.displayTitle()
		d := ep.displayDate()
		dateStr := strings.Repeat(" ", 10)
		if !d.IsZero() {
			dateStr = d.Format("2006-01-02")
		}

		availWidth := m.width - 2
		isSelected := (i == m.epIdx)
		isBatchSelected := m.isEpisodeSelected(ep.path)

		selPrefix := ""
		if len(m.selectedEpisodes) > 0 {
			if isBatchSelected {
				selPrefix = "[x] "
			} else {
				selPrefix = "[ ] "
			}
		}

		epGUID := ""
		if ep.absData != nil {
			epGUID = ep.absData.ID
		}
		inDLQueue := IsEpisodeInDownloadQueue(epGUID, "", ep.displayTitle()) || IsEpisodeInDownloadQueue(epGUID, "", ep.filename)

		if availWidth >= 40 {
			qBadgeWidth := 0
			if isQueued(ep.filename) {
				qBadgeWidth = 4
			}
			dlBadgeWidth := 0
			if inDLQueue {
				dlBadgeWidth = 11
			}
			txBadgeWidth := 0
			if ep.hasTranscript {
				txBadgeWidth = 5
			}
			selWidth := len([]rune(selPrefix))
			titleWidth := availWidth - 16 - qBadgeWidth - dlBadgeWidth - txBadgeWidth - selWidth
			if titleWidth < 10 {
				titleWidth = 10
			}
			truncTitle := truncate(displayName(displayNameStr), titleWidth)
			padLen := max(0, titleWidth-len([]rune(truncTitle)))
			padding := strings.Repeat(" ", padLen)

			if isSelected {
				chk := "  "
				if ep.isFeedOnly {
					chk = "☁ "
				} else if ep.hasAdsRemoved {
					chk = "✓ "
				}
				txBadge := ""
				if ep.hasTranscript {
					txBadge = "[TX] "
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " [Q]"
				}
				dlBadge := ""
				if inDLQueue {
					dlBadge = " [⏳ Queued]"
				}
				plainLine := "  " + selPrefix + dateStr + "  " + chk + txBadge + truncTitle + padding + qBadge + dlBadge
				fullPad := max(0, availWidth-len([]rune(plainLine)))
				fullRow := plainLine + strings.Repeat(" ", fullPad)
				out.WriteString(tuiSelectedStyle.Render(fullRow))
			} else {
				chk := "  "
				if ep.isFeedOnly {
					chk = tuiYellowStyle.Render("☁") + " "
				} else if ep.hasAdsRemoved {
					chk = tuiGreenStyle.Render("✓") + " "
				}
				txBadge := ""
				if ep.hasTranscript {
					txBadge = tuiCyanStyle.Render("[TX]") + " "
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " " + tuiBadgeQueued.Render("[Q]")
				}
				dlBadge := ""
				if inDLQueue {
					dlBadge = " " + tuiBadgeQueued.Render("[⏳ Queued]")
				}
				selRender := selPrefix
				if isBatchSelected {
					selRender = tuiYellowStyle.Render("[x] ")
				}
				line := "  " + selRender + tuiSubtextStyle.Render(dateStr) + "  " + chk + txBadge + truncTitle + padding + qBadge + dlBadge
				out.WriteString(line)
			}
		} else {
			if isSelected {
				chk := " "
				if ep.isFeedOnly {
					chk = "☁"
				} else if ep.hasAdsRemoved {
					chk = "✓"
				}
				txBadge := ""
				if ep.hasTranscript {
					txBadge = "[TX]"
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " [Q]"
				}
				line := selPrefix + dateStr + " " + chk + " " + txBadge + " " + displayName(displayNameStr) + qBadge
				truncLine := truncate(line, max(1, m.width-1))
				fullPad := max(0, (m.width-1)-len([]rune(truncLine)))
				out.WriteString(tuiSelectedStyle.Render(truncLine + strings.Repeat(" ", fullPad)))
			} else {
				chk := " "
				if ep.isFeedOnly {
					chk = tuiYellowStyle.Render("☁")
				} else if ep.hasAdsRemoved {
					chk = tuiGreenStyle.Render("✓")
				}
				txBadge := ""
				if ep.hasTranscript {
					txBadge = tuiCyanStyle.Render("[TX]")
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " " + tuiBadgeQueued.Render("[Q]")
				}
				line := selPrefix + tuiSubtextStyle.Render(dateStr) + " " + chk + " " + txBadge + " " + displayName(displayNameStr) + qBadge
				out.WriteString(truncate(line, max(1, m.width-1)))
			}
		}
		out.WriteByte('\n')
	}

	for i := start; i < end; i++ {
		renderEpRow(i)
	}

	helpText := "↑↓ navigate │ Enter details │ p play │ F fetch-feed │ D download │ v select │ a batch-queue │ t transcript │ ? help"
	if m.searchMode {
		helpText = fmt.Sprintf("Search: %s█  (Enter: Apply, Esc: Cancel)", m.searchQuery)
	} else if len(eps) > maxVis {
		helpText += fmt.Sprintf(" │ [%d/%d]", m.epIdx+1, len(eps))
	}
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")
	if m.searchMode {
		out.WriteString(tuiSearchStyle.Render("  "+helpText) + "\n")
	} else {
		out.WriteString(tuiDimStyle.Render("  "+helpText) + "\n")
	}

	return out.String()
}
