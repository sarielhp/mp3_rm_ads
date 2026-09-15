package podcast

import (
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// SanitizeTitle turns an episode or podcast title into a safe, space-free filename stem.
// Quotes and apostrophes are removed; non-alphanumeric runes (including spaces and punctuation)
// are converted to underscores. Consecutive underscores are collapsed and trimmed.
func SanitizeTitle(title string) string {
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

// FormatEpisodeFilename builds a standardized, collision-resistant filename:
// [YYYY-MM-DD_][ep<NNN>_]<Clean_Title>.mp3
func FormatEpisodeFilename(pubDate time.Time, episode string, title string) string {
	cleanTitle := SanitizeTitle(title)
	var parts []string

	if !pubDate.IsZero() {
		parts = append(parts, pubDate.Format("2006-01-02"))
	}

	cleanEp := strings.TrimSpace(episode)
	if cleanEp != "" && cleanEp != "0" {
		digits := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, cleanEp)
		if digits != "" {
			epTag := "ep" + digits
			lowerTitle := strings.ToLower(cleanTitle)
			if !strings.HasPrefix(lowerTitle, strings.ToLower(epTag)+"_") &&
				!strings.HasPrefix(lowerTitle, "ep") &&
				!strings.HasPrefix(lowerTitle, digits+"_") {
				parts = append(parts, epTag)
			}
		}
	}

	parts = append(parts, cleanTitle)
	stem := strings.Join(parts, "_")
	return stem + ".mp3"
}

// StripEpisodeFilenamePrefix removes date prefix (YYYY-MM-DD_) and optional ep prefix (ep[0-9]+_) from a filename stem.
func StripEpisodeFilenamePrefix(stem string) string {
	stem = strings.TrimSuffix(stem, ".mp3")
	if len(stem) > 11 && stem[4] == '-' && stem[7] == '-' && stem[10] == '_' {
		isDate := true
		for i := 0; i < 10; i++ {
			if i == 4 || i == 7 {
				continue
			}
			if stem[i] < '0' || stem[i] > '9' {
				isDate = false
				break
			}
		}
		if isDate {
			stem = stem[11:]
		}
	}
	if strings.HasPrefix(strings.ToLower(stem), "ep") {
		idx := strings.IndexByte(stem, '_')
		if idx > 2 {
			allDigits := true
			for _, r := range stem[2:idx] {
				if !unicode.IsDigit(r) {
					allDigits = false
					break
				}
			}
			if allDigits {
				stem = stem[idx+1:]
			}
		}
	}
	return stem
}

// ParseDatePrefix extracts a publication date from a filename starting with YYYY-MM-DD.
func ParseDatePrefix(name string) (time.Time, bool) {
	name = filepath.Base(name)
	if len(name) >= 10 && name[4] == '-' && name[7] == '-' {
		t, err := time.Parse("2006-01-02", name[:10])
		if err == nil && !t.IsZero() {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
