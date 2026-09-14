package cli

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"pod/pkg/util"
	"strings"
)

func formatDurationHours(totalSec float64) string {
	if totalSec <= 0 {
		return "0s"
	}
	h := int(totalSec) / 3600
	m := (int(totalSec) % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func formatDiskSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func findCoverImageInDir(dir string) string {
	coverNames := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "cover.webp",
		"folder.jpg", "folder.jpeg", "folder.png",
	}
	for _, name := range coverNames {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

func stripHTMLTags(text string) string {
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = strings.ReplaceAll(text, "<div>", "")
	text = strings.ReplaceAll(text, "</div>", "\n")

	var b strings.Builder
	inTag := false
	for _, r := range text {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	clean := html.UnescapeString(b.String())
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	return clean
}

func cleanAndFormatNotes(raw string, indentSpaces, maxLineWidth int) string {
	clean := stripHTMLTags(strings.TrimSpace(raw))
	if clean == "" {
		return ""
	}

	indent := strings.Repeat(" ", indentSpaces)
	lines := strings.Split(clean, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
			continue
		}
		wrapped := wrapText(trimmed, maxLineWidth-indentSpaces)
		for _, w := range wrapped {
			result = append(result, indent+util.DisplayName(w))
		}
	}
	return strings.Join(result, "\n")
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
