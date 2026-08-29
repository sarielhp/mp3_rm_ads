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
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
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
	t := time.Unix(sec, 0).Local()
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

func formatRelativeAge(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		return "Future"
	}

	switch {
	case diff < 24*time.Hour:
		return "Today"
	case diff < 48*time.Hour:
		return "1Day"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1Day"
		}
		return fmt.Sprintf("%dDays", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1Week"
		}
		return fmt.Sprintf("%dWeeks", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1Month"
		}
		return fmt.Sprintf("%dMonths", months)
	default:
		years := int(diff.Hours() / (24 * 365))
		if years == 1 {
			return "1Year"
		}
		return fmt.Sprintf("%dYears", years)
	}
}

func renderHTML(html string) string {
	if html == "" {
		return ""
	}

	var result strings.Builder
	inTag := false
	inBold := false
	inItalic := false
	var tagBuf strings.Builder
	var textBuf strings.Builder

	flushText := func() {
		if textBuf.Len() == 0 {
			return
		}
		text := textBuf.String()
		textBuf.Reset()
		if inBold {
			result.WriteString("\033[1m")
			result.WriteString(text)
			result.WriteString("\033[22m")
		} else if inItalic {
			result.WriteString("\033[3m")
			result.WriteString(text)
			result.WriteString("\033[23m")
		} else {
			result.WriteString(text)
		}
	}

	decodeEntity := func(entity string) string {
		switch entity {
		case "&amp;":
			return "&"
		case "&lt;":
			return "<"
		case "&gt;":
			return ">"
		case "&quot;":
			return "\""
		case "&apos;":
			return "'"
		case "&nbsp;":
			return " "
		}
		return entity
	}

	for i := 0; i < len(html); i++ {
		c := html[i]
		if c == '<' {
			flushText()
			inTag = true
			tagBuf.Reset()
			continue
		}
		if c == '>' && inTag {
			inTag = false
			tag := strings.ToLower(strings.TrimSpace(tagBuf.String()))
			switch tag {
			case "b", "strong":
				inBold = true
			case "/b", "/strong":
				inBold = false
			case "i", "em":
				inItalic = true
			case "/i", "/em":
				inItalic = false
			case "br", "br/", "br /":
				result.WriteByte('\n')
			case "p":
				result.WriteByte('\n')
			case "/p":
				result.WriteByte('\n')
			case "li":
				result.WriteString("\n  - ")
			case "/li":
				result.WriteByte('\n')
			case "/div":
				result.WriteByte('\n')
			}
			continue
		}
		if inTag {
			tagBuf.WriteByte(c)
			continue
		}
		if c == '&' {
			entityEnd := strings.IndexByte(html[i:], ';')
			if entityEnd >= 0 {
				entity := html[i : i+entityEnd+1]
				decoded := decodeEntity(entity)
				textBuf.WriteString(decoded)
				i += entityEnd
				continue
			}
		}
		textBuf.WriteByte(c)
	}
	flushText()

	return result.String()
}
