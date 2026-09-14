package backend

import (
	"strings"
)

func sanitizePodcastName(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, s)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" || cleaned == ".." || cleaned == "." || strings.Trim(cleaned, ".") == "" || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, ".._") {
		return "podcast_escaped"
	}
	return cleaned
}

func SanitizePodcastTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled Podcast"
	}
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, c := range badChars {
		title = strings.ReplaceAll(title, c, "_")
	}
	title = strings.TrimSpace(title)
	if title == "" || title == ".." || title == "." || strings.Trim(title, ".") == "" || strings.HasPrefix(title, "../") || strings.HasPrefix(title, ".._") {
		return "Untitled Podcast"
	}
	return title
}

func StripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			if c == '&' {
				entityEnd := strings.IndexByte(s[i:], ';')
				if entityEnd >= 0 {
					entity := s[i : i+entityEnd+1]
					switch entity {
					case "&amp;":
						result.WriteByte('&')
					case "&lt;":
						result.WriteByte('<')
					case "&gt;":
						result.WriteByte('>')
					case "&quot;":
						result.WriteByte('"')
					case "&apos;":
						result.WriteByte('\'')
					case "&nbsp;":
						result.WriteByte(' ')
					default:
						result.WriteString(entity)
					}
					i += entityEnd
					continue
				}
			}
			result.WriteByte(c)
		}
	}
	return strings.TrimSpace(result.String())
}
