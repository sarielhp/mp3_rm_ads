package tui

import (
	"fmt"
	"strings"

	"mail_cli/uicommon"

	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) buildDetailContent(vw int) string {
	var hdrs strings.Builder
	if m.eIdx >= 0 && m.eIdx < len(m.emails) {
		em := m.emails[m.eIdx]
		if em.IsSpam || em.IsPolitical || em.IsBlacklisted {
			badge := "[SPAM]"
			if em.IsPolitical {
				badge = "[POLI]"
			}
			if em.IsBlacklisted {
				badge = "[BLACKLIST]"
			}
			hdrs.WriteString(uicommon.Plain(badge).WithFg(uicommon.ColorSpam).Render(m.theme.Theme()) + "\n")
		}
	}
	if len(m.attachments) > 0 {
		var names []string
		for _, a := range m.attachments {
			names = append(names, a.Filename)
		}
		badge := fmt.Sprintf("📎 Attachments (%d): %s (Press 'a' to manage)", len(m.attachments), strings.Join(names, ", "))
		hdrs.WriteString(uicommon.Plain(badge).WithFg(uicommon.ColorDate).Render(m.theme.Theme()) + "\n")
	}
	for _, line := range strings.Split(m.detailH, "\n") {
		if line == "" {
			continue
		}
		idx := strings.Index(line, ": ")
		if idx > 0 {
			hdrs.WriteString(uicommon.Plain(line[:idx+2]).WithFg(uicommon.ColorHighlightFg).Render(m.theme.Theme()))
			hdrs.WriteString(uicommon.Plain(uicommon.BidiDisplay(line[idx+2:])).WithFg(uicommon.ColorFg).Render(m.theme.Theme()))
		} else {
			hdrs.WriteString(uicommon.Plain(uicommon.BidiDisplay(line)).WithFg(uicommon.ColorFg).Render(m.theme.Theme()))
		}
		hdrs.WriteString("\n")
	}
	if vw <= 0 {
		vw = 78
	}

	bodyText := m.detail
	if m.inDetailSearch || m.detailSearch != "" {
		cursorChar := ""
		if m.inDetailSearch {
			cursorChar = "█"
		}
		searchLabel := fmt.Sprintf("\n /%s%s \n", m.detailSearch, cursorChar)
		hdrs.WriteString(uicommon.Plain(searchLabel).
			WithFg(uicommon.ColorHighlightFg).WithBg(uicommon.ColorBg).Render(m.theme.Theme()))

		if m.detailSearch != "" {
			var matchedLines []string
			lowerQuery := strings.ToLower(m.detailSearch)
			for _, line := range strings.Split(bodyText, "\n") {
				if strings.Contains(strings.ToLower(line), lowerQuery) {
					matchedLines = append(matchedLines, line)
				}
			}
			if len(matchedLines) > 0 {
				result := strings.Join(matchedLines, "\n")
				bodyCS := uicommon.ColorReplyLinesRaw(result, vw)
				bodyText = bodyCS.Render(m.theme.Theme())
			} else {
				bodyText = uicommon.Plain("(no matches)").WithFg(uicommon.ColorDim).Render(m.theme.Theme())
			}
		}
	} else {
		bodyCS := uicommon.ColorReplyLinesRaw(bodyText, vw)
		bodyText = bodyCS.Render(m.theme.Theme())
	}

	endBar := uicommon.RenderEndOfMsgGreen(vw, m.theme.Theme())
	return hdrs.String() + "\n" + bodyText + "\n\n" + endBar
}

func renderSplitView(m *tuiModel) string {
	width := m.width
	if m.menuOpen {
		width = m.width - 19
	}
	t := m.theme.Theme()

	availH := m.height - 2
	if m.showError {
		availH--
	}
	if availH < 6 {
		availH = 6
	}

	topLines := max(3, (availH-1)/2)
	bottomLines := max(3, availH-topLines-1)

	headerWidth := 4 + 2 + 8 + 2 + 20 + 2 + 2 + 2
	subjWidth := width - headerWidth

	start := m.indexStart
	end := start + topLines
	if end > len(m.emails) {
		end = len(m.emails)
	}

	var indexRows []string
	for i := start; i < end; i++ {
		idxVal := m.emails[i].ThreadIndex
		if idxVal == 0 {
			idxVal = i + 1
		}
		ghosted := m.isPending(m.emails[i].ID)
		row := uicommon.RenderEmailRow(m.emails[i], width, subjWidth, idxVal, t, i == m.eIdx, ghosted)
		indexRows = append(indexRows, row.Render(t))
	}
	for len(indexRows) < topLines {
		indexRows = append(indexRows, "")
	}
	topIndexStr := strings.Join(indexRows, "\n")

	// Divider line: Neomutt style
	var dividerTitle string
	if m.eIdx >= 0 && m.eIdx < len(m.emails) {
		em := m.emails[m.eIdx]
		dividerTitle = fmt.Sprintf("── [Preview: %s] ──", em.Subject)
	} else {
		dividerTitle = "── [Preview] ──"
	}
	if len([]rune(dividerTitle)) > width {
		runes := []rune(dividerTitle)
		dividerTitle = string(runes[:max(4, width-1)])
	}
	divPad := max(0, width-len([]rune(dividerTitle)))
	fullDivider := dividerTitle + strings.Repeat("─", divPad)
	dividerStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Get(uicommon.ColorBorder))).
		Bold(true).
		Render(fullDivider)

	// Bottom Viewport (Message Preview)
	vw := max(20, width-2)
	vh := bottomLines
	if m.detailVpDirty || m.vp.Width != vw || m.vp.Height != vh {
		content := m.buildDetailContent(vw)
		m.vp.SetContent(content)
		m.vp.Width = vw
		m.vp.Height = vh
		m.detailVpDirty = false
	}
	bottomPreviewStr := m.vp.View()

	return lipgloss.JoinVertical(lipgloss.Top, topIndexStr, dividerStr, bottomPreviewStr)
}
