package main

import (
	"fmt"
	"strings"
	"time"
)

func (m *tuiModel) drawPodcastsList() string {
	out := &strings.Builder{}

	titleBanner := tuiHeaderBanner.Render(" PODCASTS ")
	out.WriteString(fmt.Sprintf("  %s\n", titleBanner))

	pods := m.filteredPodcasts()

	totalEps := 0
	totalDone := 0
	for _, p := range pods {
		totalEps += len(p.episodes)
		for _, e := range p.episodes {
			if e.hasAdsRemoved {
				totalDone++
			}
		}
	}

	statPill := tuiStatStyle.Render(fmt.Sprintf("  %d podcasts, %d episodes, %d ad-free", len(pods), totalEps, totalDone))
	out.WriteString(statPill)
	out.WriteByte('\n')

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	out.WriteByte('\n')

	maxVis := m.visibleLines(4)
	start := m.podScroll
	end := start + maxVis
	if end > len(pods) {
		end = len(pods)
	}

	if m.width >= 65 && len(pods) > 0 {
		leftW := min(36, max(26, m.width*32/100))
		rightW := max(30, m.width-leftW-5)

		var leftLines []string
		for i := start; i < end; i++ {
			p := pods[i]
			epCount := len(p.episodes)
			doneCount := 0
			for _, e := range p.episodes {
				if e.hasAdsRemoved {
					doneCount++
				}
			}

			nameStr := displayName(p.name)
			statsStr := fmt.Sprintf("(%d/%d)", epCount, doneCount)

			line := fmt.Sprintf("  %s %s", nameStr, statsStr)
			truncLine := truncate(line, leftW-2)
			pad := strings.Repeat(" ", max(0, leftW-len([]rune(truncLine))))
			fullLine := truncLine + pad

			if i == m.podIdx {
				leftLines = append(leftLines, tuiSelectedStyle.Render(fullLine))
			} else {
				leftLines = append(leftLines, fullLine)
			}
		}

		// Build right pane for selected podcast with rich details
		var rightLines []string
		if m.podIdx < len(pods) {
			selPod := pods[m.podIdx]

			// 1. Cover Art (if available and enabled)
			if isKittySupported() && m.showCover {
				coverPath := selPod.coverPath
				if coverPath == "" {
					coverPath = findCoverImage(selPod.dir)
				}
				if coverPath != "" {
					imgW := 24
					imgH := 7
					if imgEsc, err := encodeKittyGraphicsFile(coverPath, imgW, imgH); err == nil && imgEsc != "" {
						if isKittyTerminal() {
							rightLines = append(rightLines, imgEsc)
							for h := 1; h < imgH; h++ {
								rightLines = append(rightLines, strings.Repeat(" ", imgW))
							}
						} else {
							lines := strings.Split(imgEsc, "\n")
							for h := 0; h < imgH; h++ {
								if h < len(lines) && lines[h] != "" {
									rightLines = append(rightLines, lines[h])
								} else {
									rightLines = append(rightLines, strings.Repeat(" ", imgW))
								}
							}
						}
					}
				}
			}

			// 2. Podcast Title
			title := displayName(selPod.name)
			rightLines = append(rightLines, tuiTitleStyle.Render(truncate(title, rightW-2)))

			// 3. Author
			if author := selPod.displayAuthor(); author != "" {
				rightLines = append(rightLines, tuiSubtitleStyle.Render(truncate("by "+displayName(author), rightW-2)))
			}

			// 4. Detailed Stats & Timeline
			selDone := 0
			var totalDurSec float64
			var newestDate, oldestDate time.Time
			for _, e := range selPod.episodes {
				if e.hasAdsRemoved {
					selDone++
				}
				totalDurSec += e.duration
				d := e.displayDate()
				if !d.IsZero() {
					if newestDate.IsZero() || d.After(newestDate) {
						newestDate = d
					}
					if oldestDate.IsZero() || d.Before(oldestDate) {
						oldestDate = d
					}
				}
			}

			statParts := []string{
				fmt.Sprintf("%d episodes", len(selPod.episodes)),
				fmt.Sprintf("%d ad-free", selDone),
				fmt.Sprintf("%d transcribed", selPod.transcribedCount()),
			}
			if queued := len(m.queue[selPod.dir]); queued > 0 {
				statParts = append(statParts, fmt.Sprintf("%d queued", queued))
			}
			if totalDurSec > 0 {
				hrs := int(totalDurSec) / 3600
				mins := (int(totalDurSec) % 3600) / 60
				if hrs > 0 {
					statParts = append(statParts, fmt.Sprintf("~%dh %dm audio", hrs, mins))
				} else {
					statParts = append(statParts, fmt.Sprintf("~%dm audio", mins))
				}
			}
			rightLines = append(rightLines, tuiStatStyle.Render(truncate(strings.Join(statParts, " • "), rightW-2)))

			policyLine := fmt.Sprintf("Ad Policy: %s ('c' change, 'e' timeline)", adRemovalModeLabel(selPod.config.AdRemoval))
			rightLines = append(rightLines, tuiBadgePolicy.Render(truncate(policyLine, rightW-2)))

			dlPolicyLine := fmt.Sprintf("Download: %s ('d' change)", downloadPolicyLabel(selPod.config.DownloadPolicy, selPod.config.DownloadK))
			rightLines = append(rightLines, tuiBadgePolicy.Render(truncate(dlPolicyLine, rightW-2)))

			if !newestDate.IsZero() {
				dateInfo := fmt.Sprintf("Timeline: %s", newestDate.Format("2006-01-02"))
				if !oldestDate.IsZero() && !oldestDate.Equal(newestDate) {
					dateInfo += fmt.Sprintf(" (oldest: %s)", oldestDate.Format("2006-01-02"))
				}
				rightLines = append(rightLines, tuiSubtextStyle.Render(truncate(dateInfo, rightW-2)))
			}

			// 5. Description (rich multi-line wrapped text)
			if desc := selPod.displayDescription(); desc != "" {
				clean := strings.TrimSpace(renderHTML(desc))
				if len(clean) > 0 {
					rightLines = append(rightLines, "")
					prefix := "About: "
					lines := wrapText(prefix+clean, rightW-2)
					if len(lines) > 0 {
						firstLine := lines[0]
						if strings.HasPrefix(firstLine, prefix) {
							rest := strings.TrimPrefix(firstLine, prefix)
							rightLines = append(rightLines, tuiSectionTitle.Render(prefix)+tuiDimStyle.Render(rest))
						} else {
							rightLines = append(rightLines, tuiDimStyle.Render(firstLine))
						}
						for _, l := range lines[1:] {
							if len(rightLines) >= maxVis-5 && len(selPod.episodes) > 0 {
								break
							}
							if len(rightLines) >= maxVis {
								break
							}
							rightLines = append(rightLines, tuiDimStyle.Render(l))
						}
					}
				}
			}

			// 6. Recent Episodes preview
			if len(selPod.episodes) > 0 && len(rightLines) < maxVis-2 {
				rightLines = append(rightLines, "")
				rightLines = append(rightLines, tuiSectionTitle.Render("Recent Episodes:"))
				for epIdx, ep := range selPod.episodes {
					if len(rightLines) >= maxVis {
						break
					}
					if epIdx >= 5 {
						break
					}
					d := ep.displayDate()
					dStr := ""
					if !d.IsZero() {
						dStr = d.Format("2006-01-02") + " "
					}
					chk := " "
					if ep.hasAdsRemoved {
						chk = "✓ "
					}
					txTag := ""
					if ep.hasTranscript {
						txTag = "[TX] "
					}
					epLine := fmt.Sprintf("  %s%s%s%s", chk, txTag, dStr, ep.displayTitle())
					rightLines = append(rightLines, tuiDimStyle.Render(truncate(epLine, rightW-2)))
				}
			}
		}

		// Join columns
		totalLines := max(len(leftLines), len(rightLines))
		for k := 0; k < totalLines; k++ {
			leftPart := ""
			if k < len(leftLines) {
				leftPart = leftLines[k]
			} else {
				leftPart = strings.Repeat(" ", leftW)
			}

			rightPart := ""
			if k < len(rightLines) {
				rightPart = rightLines[k]
			}

			out.WriteString(leftPart + tuiDividerStyle.Render(" │ ") + rightPart + "\n")
		}
	} else {
		for i := start; i < end; i++ {
			p := pods[i]
			epCount := len(p.episodes)
			doneCount := 0
			for _, e := range p.episodes {
				if e.hasAdsRemoved {
					doneCount++
				}
			}

			nameStr := displayName(p.name)
			statsStr := fmt.Sprintf("(%d/%d)", epCount, doneCount)

			authorStr := ""
			if author := p.displayAuthor(); author != "" {
				authorStr = fmt.Sprintf(" [%s]", displayName(author))
			}

			line := fmt.Sprintf("  %s  %s%s", nameStr, statsStr, authorStr)
			line = truncate(line, max(1, m.width-1))

			if i == m.podIdx {
				out.WriteString(tuiSelectedStyle.Render(line))
			} else {
				out.WriteString(line)
			}
			out.WriteByte('\n')
		}
	}

	helpText := "↑↓ navigate │ Enter select │ L latest │ c ad-policy │ d dl-policy │ e timeline │ / search │ ? help"
	if m.searchMode {
		helpText = fmt.Sprintf("Search: %s█  (Enter: Apply, Esc: Cancel)", m.searchQuery)
	} else if len(pods) > maxVis {
		pct := int(float64(m.podIdx+1) / float64(len(pods)) * 100)
		helpText += fmt.Sprintf(" │ [%d/%d (%d%%)]", m.podIdx+1, len(pods), pct)
	}
	dividerWidth = max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")
	if m.searchMode {
		out.WriteString(tuiSearchStyle.Render("  "+helpText) + "\n")
	} else {
		out.WriteString(tuiDimStyle.Render("  "+helpText) + "\n")
	}

	return out.String()
}

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
	if globalPlayer.Current != nil {
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
				if ep.hasAdsRemoved {
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
				if ep.hasAdsRemoved {
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
				if ep.hasAdsRemoved {
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
				if ep.hasAdsRemoved {
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

	helpText := "↑↓ navigate │ Enter details │ p play │ D download │ v select │ a batch-queue │ t transcript │ c ad-policy │ d dl-policy │ ? help"
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
