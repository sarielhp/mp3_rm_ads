package tui

import (
	"fmt"
	"strings"

	"mail_cli/uicommon"

	"github.com/charmbracelet/lipgloss"
)

func folderVisibleHeight(m *tuiModel) int {
	h := m.height - 4
	lipglossBorder := 2
	visible := h - lipglossBorder
	if visible < 1 {
		visible = 1
	}
	return visible
}

func renderFolderTree(m *tuiModel) string {
	if len(m.entries) == 0 {
		return "(no folders)"
	}
	vis := folderVisibleHeight(m)
	start := m.cursor - vis/2
	if start < 0 {
		start = 0
	}
	end := start + vis
	if end > len(m.entries) {
		end = len(m.entries)
		start = end - vis
		if start < 0 {
			start = 0
		}
	}
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, uicommon.MakeLabelLine(m.entries[i], m.cursor == i, m.theme.Theme(), m.width))
	}
	return strings.Join(lines, "\n")
}

func renderFolderOverlay(m *tuiModel, content string) string {
	panel := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - 4).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(m.theme.Theme().Get(uicommon.ColorBorder))).
		Render(content)
	var barText string
	if m.inFolderSearch {
		barText = fmt.Sprintf(" Search: %s█  (Esc:Exit search, Arrow keys:Nav, Enter:Select)", m.folderSearch)
	} else {
		barText = " F:Close  /:Search  Tab:Subtree  Enter:Select  j/k:Nav  q:Quit"
	}
	bar := lipgloss.NewStyle().Width(m.width).Render(barText)
	return lipgloss.JoinVertical(lipgloss.Bottom, panel, bar)
}

func renderDownloadingDialog(width, height int) string {
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4a9e6e")).
		Padding(1, 3).
		Bold(true).
		Foreground(lipgloss.Color("#ffdd44")).
		Background(lipgloss.Color("#2b2b2b")).
		Render(" Downloading... ")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(dialog)
}

func renderUnspammingDialog(width, height int) string {
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4a9e6e")).
		Padding(1, 3).
		Bold(true).
		Foreground(lipgloss.Color("#ffdd44")).
		Background(lipgloss.Color("#2b2b2b")).
		Render(" Unspamming... ")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(dialog)
}
