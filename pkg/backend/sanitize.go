package backend

import (
	"strings"
	"unicode"
)

func sanitizePodcastName(s string) string {
	res := SanitizePodcastTitle(s)
	if res == "untitled" {
		return "podcast_escaped"
	}
	return res
}

func SanitizePodcastTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "untitled"
	}
	for _, q := range []string{"'", "\"", "`", "’", "‘", "“", "”"} {
		title = strings.ReplaceAll(title, q, "")
	}

	var sb strings.Builder
	lastUnderscore := false

	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			lastUnderscore = false
		} else {
			if !lastUnderscore && sb.Len() > 0 {
				sb.WriteRune('_')
				lastUnderscore = true
			}
		}
	}

	res := strings.Trim(sb.String(), "_")
	if res == "" || res == ".." || res == "." {
		return "untitled"
	}
	runes := []rune(res)
	if len(runes) > 120 {
		res = strings.Trim(string(runes[:120]), "_")
	}
	if res == "" {
		return "untitled"
	}
	return res
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
