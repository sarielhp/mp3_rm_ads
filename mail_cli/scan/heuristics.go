package scan

import (
	"regexp"
	"strings"
	"unicode"
)

func isRuneAllowed(r rune, whitelist []string) bool {
	if !unicode.IsLetter(r) {
		return true
	}

	for _, lang := range whitelist {
		l := strings.ToLower(strings.TrimSpace(lang))
		switch l {
		case "english", "german", "french":
			if unicode.Is(unicode.Latin, r) {
				return true
			}
		case "hebrew":
			if unicode.Is(unicode.Hebrew, r) {
				return true
			}
		}
	}

	return false
}

func detectScriptLabel(r rune) string {
	if unicode.Is(unicode.Cyrillic, r) {
		return "Cyrillic/Russian"
	}
	if unicode.Is(unicode.Greek, r) {
		return "Greek"
	}
	if unicode.Is(unicode.Han, r) {
		return "Han"
	}
	if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
		return "Japanese"
	}
	if unicode.Is(unicode.Hangul, r) {
		return "Korean"
	}
	if unicode.Is(unicode.Arabic, r) {
		return "Arabic"
	}
	if unicode.Is(unicode.Devanagari, r) {
		return "Hindi/Devanagari"
	}
	if unicode.Is(unicode.Thai, r) {
		return "Thai"
	}
	return "Unknown Script"
}

var urlRegex = regexp.MustCompile(`https?://[^\s<>]+|www\.[^\s<>]+`)

func cleanTextForNLP(text string) string {
	text = urlRegex.ReplaceAllString(text, "")
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			sb.WriteRune(r)
		}
	}
	return strings.TrimSpace(sb.String())
}

func isDetectedLanguageWhitelisted(detectedLang string, iso6391 string, whitelist []string) bool {
	detName := strings.ToLower(detectedLang)
	detISO := strings.ToLower(iso6391)

	for _, lang := range whitelist {
		l := strings.ToLower(strings.TrimSpace(lang))
		if l == detName || l == detISO {
			return true
		}
		if (l == "english" && detISO == "en") ||
			(l == "german" && detISO == "de") ||
			(l == "french" && detISO == "fr") ||
			(l == "hebrew" && (detISO == "he" || detISO == "iw")) {
			return true
		}
	}
	return false
}

func isContiguousGreekRunes(runes []rune, idx int) bool {
	if idx > 0 && unicode.Is(unicode.Greek, runes[idx-1]) {
		return true
	}
	if idx < len(runes)-1 && unicode.Is(unicode.Greek, runes[idx+1]) {
		return true
	}
	return false
}
