package main

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func wrapDescription(descClean string, maxW int) []string {
	if len(descClean) == 0 {
		return []string{tuiDimStyle.Render("No description available.")}
	}
	var res []string
	lastWasEmpty := false
	for _, line := range strings.Split(descClean, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !lastWasEmpty && len(res) > 0 {
				res = append(res, "")
				lastWasEmpty = true
			}
			continue
		}
		lastWasEmpty = false
		wrapped := wrapText(trimmed, maxW)
		res = append(res, wrapped...)
	}
	for len(res) > 0 && res[len(res)-1] == "" {
		res = res[:len(res)-1]
	}
	if len(res) == 0 {
		return []string{tuiDimStyle.Render("No description available.")}
	}
	return res
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
