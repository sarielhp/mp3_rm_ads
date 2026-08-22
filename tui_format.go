package main

import (
	"fmt"
	"strings"
	"time"
)

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}

func formatDurationShort(secs float64) string {
	m := int(secs) / 60
	s := int(secs) % 60
	if m >= 60 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatABSDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.Format("2006-01-02")
	}
	return dateStr
}

func formatTimestamp(ms int64) string {
	if ms <= 0 {
		return ""
	}
	sec := ms
	if ms > 1e11 {
		sec = ms / 1000
	}
	t := time.Unix(sec, 0).UTC()
	return t.Format("2006-01-02 15:04")
}

func splitText(text string, maxWidth int) []string {
	if maxWidth <= 0 || text == "" {
		return []string{text}
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(p)
		if len(words) == 0 {
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= maxWidth {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
}

func absEpisodeTitle(ep *absEpisode) string {
	if ep == nil {
		return ""
	}
	if ep.Title != "" {
		return ep.Title
	}
	if ep.Season != "" && ep.Episode != "" {
		return fmt.Sprintf("S%sE%s", ep.Season, ep.Episode)
	}
	return ""
}

func absEpisodeDescription(ep *absEpisode) string {
	if ep == nil {
		return ""
	}
	if ep.Description != "" {
		desc := stripHTML(ep.Description)
		if len(desc) > 120 {
			return desc[:120] + "..."
		}
		return desc
	}
	return ""
}

func absEpisodeDuration(ep *absEpisode) string {
	if ep == nil || ep.Duration <= 0 {
		return ""
	}
	return formatDurationShort(ep.Duration)
}
