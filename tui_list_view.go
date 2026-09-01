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

		var rightLines []string
		if m.podIdx < len(pods) {
			selPod := pods[m.podIdx]

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

			title := displayName(selPod.name)
			rightLines = append(rightLines, tuiTitleStyle.Render(truncate(title, rightW-2)))

			if author := selPod.displayAuthor(); author != "" {
				rightLines = append(rightLines, tuiSubtitleStyle.Render(truncate("by "+displayName(author), rightW-2)))
			}

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

	helpText := "↑↓ navigate │ Enter select │ F fetch-feed │ D dl-all │ L latest │ c ad-policy │ d dl-policy │ ? help"
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
