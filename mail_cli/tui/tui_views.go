package tui

import (
	"fmt"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/uicommon"

	"github.com/charmbracelet/lipgloss"
)

func renderTopBar(m *tuiModel) string {
	t := m.theme.Theme()
	account := "default"
	if m.Config() != nil {
		for _, a := range m.Config().Accounts {
			if strings.EqualFold(a.Name, m.Config().SelectedAccount) ||
				(m.Config().SelectedAccount == "" && len(m.Config().Accounts) > 0 && strings.EqualFold(m.Config().Accounts[0].Name, a.Name)) {
				account = a.GetDisplayName()
				break
			}
		}
	} else if m.cfg != nil {
		for _, a := range m.cfg.Accounts {
			if strings.EqualFold(a.Name, m.cfg.SelectedAccount) ||
				(m.cfg.SelectedAccount == "" && len(m.cfg.Accounts) > 0 && strings.EqualFold(m.cfg.Accounts[0].Name, a.Name)) {
				account = a.GetDisplayName()
				break
			}
		}
	}
	folder := m.currentF
	if folder == "" && len(m.folders) > 0 {
		folder = m.folders[0].FullName
	}
	unread := 0
	for _, e := range m.rawEmails {
		if !e.IsRead {
			unread++
		}
	}
	unreadStr := fmt.Sprintf("%d unread", unread)

	menuLabel := " [Menu] "
	var menuCS uicommon.ColoredString
	if m.menuOpen {
		menuCS = uicommon.Plain(menuLabel).WithFg(uicommon.ColorTopBg).WithBg(uicommon.ColorHighlightFg)
	} else {
		menuCS = uicommon.Plain(menuLabel).WithFg(uicommon.ColorHighlightFg).WithBg(uicommon.ColorTopBg)
	}

	left := joinCol(menuCS, uicommon.Plain(account).WithFg(uicommon.ColorFg).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	if m.isReadOnly() {
		left = joinCol(left, uicommon.Plain(" [READ-ONLY]").WithFg(uicommon.ColorHighlightFg).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	}
	left = joinCol(left, uicommon.Plain(" - ").WithFg(uicommon.ColorFg).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	left = joinCol(left, uicommon.Plain(folder).WithFg(uicommon.ColorFg).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	left = joinCol(left, uicommon.Plain(" | ").WithFg(uicommon.ColorFg).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	left = joinCol(left, uicommon.Plain(unreadStr).WithFg(uicommon.ColorDate).WithBg(uicommon.ColorTopBg)).WithBgAll(uicommon.ColorTopBg)
	right := uicommon.Plain("h/?: help").WithFg(uicommon.ColorHighlightFg).WithBg(uicommon.ColorTopBg)

	leftW := left.Width()
	rightW := right.Width()
	leftStr := left.Render(t)
	rightStr := right.Render(t)

	topBgStyle := lipgloss.NewStyle().Background(lipgloss.Color(t.Get(uicommon.ColorTopBg)))

	if m.inSearch || m.searchQuery != "" {
		searchPrefix := "/"
		if m.fuzzySearch && m.fuzzyGlobal {
			searchPrefix = "~"
		} else if m.fuzzySearch {
			searchPrefix = "!"
		}
		cursorChar := ""
		if m.inSearch {
			cursorChar = "█"
		}
		rawSearchText := searchPrefix + m.searchQuery + cursorChar
		searchBoxText := " " + rawSearchText + " "

		maxBoxW := max(6, m.width-leftW-rightW-4)
		if len([]rune(searchBoxText)) > maxBoxW {
			runes := []rune(searchBoxText)
			searchBoxText = string(runes[:max(1, maxBoxW-1)]) + "… "
		}
		boxW := len([]rune(searchBoxText))

		// Search line in red on light gray
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cc1111")).
			Background(lipgloss.Color("#d8d8d8")).
			Bold(true)
		searchBoxStr := searchStyle.Render(searchBoxText)

		remaining := max(0, m.width-leftW-rightW-boxW)
		leftPad := 1
		if remaining >= 2 {
			leftPad = 2
		}
		rightPad := max(0, remaining-leftPad)

		var sb strings.Builder
		sb.WriteString(leftStr)
		sb.WriteString(topBgStyle.Render(strings.Repeat(" ", leftPad)))
		sb.WriteString(searchBoxStr)
		sb.WriteString(topBgStyle.Render(strings.Repeat(" ", rightPad)))
		sb.WriteString(rightStr)
		return sb.String()
	}

	spacesNeeded := max(0, m.width-leftW-rightW)
	return leftStr + topBgStyle.Render(strings.Repeat(" ", spacesNeeded)) + rightStr
}

func joinCol(cols ...uicommon.ColoredString) uicommon.ColoredString {
	var all []uicommon.Segment
	for _, c := range cols {
		all = append(all, c.Segments...)
	}
	return uicommon.ColoredString{Segments: all}
}

func renderIndex(m *tuiModel) string {
	width := m.width
	if m.menuOpen {
		width = m.width - 19
	}
	headerWidth := 4 + 2 + 8 + 2 + 20 + 2 + 2 + 2
	subjWidth := width - headerWidth
	t := m.theme.Theme()

	numLines := m.height - 2
	if m.showError {
		numLines--
	}
	if numLines <= 0 {
		numLines = 1
	}

	if len(m.emails) == 0 {
		return renderRainbowBannerEmptyState(width, numLines, t)
	}

	var lines []string
	if m.mode == ModeDetail {
		start := max(0, m.eIdx-2)
		end := start + 5
		if end > len(m.emails) {
			end = len(m.emails)
		}
		for i := start; i < end; i++ {
			idxVal := m.emails[i].ThreadIndex
			if idxVal == 0 {
				idxVal = i + 1
			}
			ghosted := m.isPending(m.emails[i].ID)
			row := uicommon.RenderEmailRow(m.emails[i], width, subjWidth, idxVal, t, i == m.eIdx, ghosted)
			lines = append(lines, row.Render(t))
		}
		return strings.Join(lines, "\n")
	}

	start := m.indexStart
	end := start + numLines
	if end > len(m.emails) {
		end = len(m.emails)
	}
	for i := start; i < end; i++ {
		idxVal := m.emails[i].ThreadIndex
		if idxVal == 0 {
			idxVal = i + 1
		}
		ghosted := m.isPending(m.emails[i].ID)
		row := uicommon.RenderEmailRow(m.emails[i], width, subjWidth, idxVal, t, i == m.eIdx, ghosted)
		lines = append(lines, row.Render(t))
	}
	for len(lines) < numLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderHelp(m *tuiModel) string {
	t := m.theme.Theme()
	borderColor := t.Get(uicommon.ColorBorder)
	helpBg := lipgloss.Color("#1a1a35")

	leftCol := []string{
		"  j/↓          Move down",
		"  k/↑          Move up",
		"  Space        Toggle split preview",
		"  ←/→          Prev/Next email (split)",
		"  ↑/↓          Scroll preview (split)",
		"  e/Enter      Read full email",
		"  q            Back / Quit",
		"  F3/A         Switch account",
		"  1-9          Jump to account",
		"  [/]          Prev/Next account",
	}
	rightCol := []string{
		"  E            Archive email",
		"  s            Mark as spam",
		"  U            Mark as unspam",
		"  d            Delete email",
		"  r/g          Reply / Group reply",
		"  F            Toggle folder tree",
		"  Tab          Expand subtree / Search",
		"  R            Refresh folder",
		"  /            Search/filter emails",
		"  ~/!          Fuzzy search (all/folder)",
	}

	navCol := []string{
		"  / (detail)   Search message body",
		"  m/Alt        Open menu",
		"  a            Manage attachments",
		"  F1/h/?       Toggle this help",
		"  Esc          Close / Clear filter",
		"  Ctrl+D       Diagnostics panel",
		"  Ctrl+C       Quit",
	}

	maxLeft := 0
	for _, s := range leftCol {
		if len(s) > maxLeft {
			maxLeft = len(s)
		}
	}
	maxRight := 0
	for _, s := range rightCol {
		if len(s) > maxRight {
			maxRight = len(s)
		}
	}

	colW := max(maxLeft, maxRight) + 4

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Background(helpBg).Render("  ⚡ KEYBINDINGS  "))
	lines = append(lines, "")

	for i := 0; i < len(leftCol) || i < len(rightCol); i++ {
		l := ""
		if i < len(leftCol) {
			l = leftCol[i]
		}
		r := ""
		if i < len(rightCol) {
			r = rightCol[i]
		}
		lStyled := lipgloss.NewStyle().Background(helpBg).Render(l)
		rStyled := lipgloss.NewStyle().Background(helpBg).Render(r)
		pad := colW - len(l)
		if pad < 0 {
			pad = 0
		}
		line := lStyled + lipgloss.NewStyle().Background(helpBg).Render(strings.Repeat(" ", pad)) + rStyled
		lines = append(lines, line)
	}

	lines = append(lines, "")
	for _, s := range navCol {
		lines = append(lines, lipgloss.NewStyle().Background(helpBg).Render(s))
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Background(helpBg).Render("  Press F1/h/? to dismiss  "))

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Background(helpBg).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 2).
		Render(content)
}

func renderGrayoutOverlay(width, height int, content string) string {
	contentLines := strings.Split(content, "\n")
	contentHeight := len(contentLines)

	if contentHeight > height {
		return content
	}

	grayLine := fmt.Sprintf("\x1b[48;2;26;26;26m%s\x1b[0m", strings.Repeat(" ", width))
	topPad := (height - contentHeight) / 2
	bottomPad := height - contentHeight - topPad

	var result []string
	for i := 0; i < topPad; i++ {
		result = append(result, grayLine)
	}
	result = append(result, contentLines...)
	for i := 0; i < bottomPad; i++ {
		result = append(result, grayLine)
	}

	return strings.Join(result, "\n")
}

func renderErrorBanner(m *tuiModel) string {
	if m.err == nil {
		return ""
	}
	errStr := fmt.Sprintf("⚠️ Error: %v | Press R to retry, Esc to dismiss", m.err)
	return lipgloss.NewStyle().
		Width(m.width).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#d32f2f")).
		Bold(true).
		Render(errStr)
}

func renderDiag(m *tuiModel) string {
	logs := app.GetDiagLogs()
	content := strings.Join(logs, "\n")

	m.diagVp.Width = max(20, m.width-2)
	m.diagVp.Height = max(5, m.height-4)
	m.diagVp.SetContent(content)
	m.diagVp.GotoBottom()

	panel := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - 3).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(m.theme.Theme().Get(uicommon.ColorBorder))).
		Render(m.diagVp.View())

	bar := lipgloss.NewStyle().
		Width(m.width).
		Foreground(lipgloss.Color(m.theme.Theme().Get(uicommon.ColorHighlightFg))).
		Render(" Diagnostics Panel | Ctrl+D: Close | j/k: Scroll | End: Bottom")

	return lipgloss.JoinVertical(lipgloss.Bottom, panel, bar)
}

func renderAttachmentDrawer(m *tuiModel) string {
	if len(m.attachments) == 0 {
		return ""
	}
	width := m.width
	if m.menuOpen {
		width = m.width - 19
	}
	var lines []string
	t := m.theme.Theme()

	for i, att := range m.attachments {
		kb := len(att.Data) / 1024
		kbStr := fmt.Sprintf("%d KB", kb)
		if kb > 1024 {
			kbStr = fmt.Sprintf("%.1f MB", float64(kb)/1024.0)
		}

		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Get(uicommon.ColorFg)))
		if i == m.attCursor {
			prefix = "> "
			style = style.Bold(true).Foreground(lipgloss.Color(t.Get(uicommon.ColorHighlightFg)))
		}
		lines = append(lines, prefix+style.Render(fmt.Sprintf("%d. %s (%s)", i+1, att.Filename, kbStr)))
	}

	content := strings.Join(lines, "\n")
	drawerPanel := lipgloss.NewStyle().
		Width(width - 2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(t.Get(uicommon.ColorBorder))).
		Render(content)

	statusText := " [Enter/o] Open | [s] Save to ~/Downloads | [a/Esc/q] Close"
	if m.saveStatus != "" {
		if time.Since(m.saveStatusTime) < 4*time.Second {
			statusText = " Status: " + m.saveStatus
		}
	}

	statusBar := lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color(t.Get(uicommon.ColorDate))).
		Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Bottom, drawerPanel, statusBar)
}

func renderMenu(m *tuiModel) string {
	t := m.theme.Theme()
	borderColor := t.Get(uicommon.ColorBorder)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))

	var lines []string
	lines = append(lines, " "+borderStyle.Render("┌────────────────┐"))
	for i, item := range m.menuItems {
		var line string
		if i == m.menuCursor {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Get(uicommon.ColorHighlightFg))).
				Bold(true).
				Render(fmt.Sprintf(" > %-12s ", item))
		} else {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Get(uicommon.ColorFg))).
				Render(fmt.Sprintf("   %-12s ", item))
		}
		lines = append(lines, " "+borderStyle.Render("│")+line+borderStyle.Render("│"))
	}
	lines = append(lines, " "+borderStyle.Render("└────────────────┘"))
	return strings.Join(lines, "\n")
}

type rgb struct {
	r, g, b int
}

func interpolateColor(x, width int) string {
	colors := []rgb{
		{255, 0, 0},   // Red
		{255, 127, 0}, // Orange
		{255, 255, 0}, // Yellow
		{0, 255, 0},   // Green
		{0, 191, 255}, // Sky Blue
		{0, 0, 255},   // Blue
		{139, 0, 255}, // Violet
	}

	if x <= 0 {
		return fmt.Sprintf("#%02x%02x%02x", colors[0].r, colors[0].g, colors[0].b)
	}
	if x >= width-1 {
		last := colors[len(colors)-1]
		return fmt.Sprintf("#%02x%02x%02x", last.r, last.g, last.b)
	}

	numSegments := len(colors) - 1
	segmentWidth := float64(width-1) / float64(numSegments)
	segment := int(float64(x) / segmentWidth)

	t := (float64(x) - float64(segment)*segmentWidth) / segmentWidth

	c1 := colors[segment]
	c2 := colors[segment+1]

	r := int(float64(c1.r) + t*float64(c2.r-c1.r))
	g := int(float64(c1.g) + t*float64(c2.g-c1.g))
	b := int(float64(c1.b) + t*float64(c2.b-c1.b))

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func renderRainbowBannerEmptyState(width, numLines int, t *uicommon.Theme) string {
	bannerLines := []string{
		"  ██    ███████  ███    ███  ██████   ████████  ██    ██     ██  ",
		" ██     ██       ████  ████  ██   ██     ██      ██  ██       ██ ",
		"██      █████    ██ ████ ██  ██████      ██       ████         ██",
		" ██     ██       ██  ██  ██  ██          ██        ██         ██ ",
		"  ██    ███████  ██      ██  ██          ██        ██        ██  ",
	}

	bannerWidth := 61
	bannerHeight := 5

	if width < bannerWidth+4 || numLines < bannerHeight+2 {
		msgText := " 📪 Mailbox Empty "
		runes := []rune(msgText)
		var sb strings.Builder
		for x, r := range runes {
			colorStr := interpolateColor(x, len(runes))
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorStr)).Render(string(r)))
		}

		emptyMsg := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Get(uicommon.ColorBorder))).
			Padding(1, 3).
			Render(sb.String())

		return lipgloss.NewStyle().
			Width(width).
			Height(numLines).
			Align(lipgloss.Center, lipgloss.Center).
			Render(emptyMsg)
	}

	var coloredLines []string
	for _, line := range bannerLines {
		runes := []rune(line)
		var sb strings.Builder
		for x, r := range runes {
			colorStr := interpolateColor(x, len(runes))
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorStr)).Render(string(r)))
		}
		coloredLines = append(coloredLines, sb.String())
	}
	bannerStr := strings.Join(coloredLines, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(numLines).
		Align(lipgloss.Center, lipgloss.Center).
		Render(bannerStr)
}
