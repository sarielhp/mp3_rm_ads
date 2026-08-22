package email

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/ianaindex"
)

var (
	encodedWordRegexp           = regexp.MustCompile(`=\?[a-zA-Z0-9_-]+\?[qQbB]\?[^?]*\?=`)
	incompleteEncodedWordRegexp = regexp.MustCompile(`(?i)\s*=\?[a-z0-9_-]*(?:\?[a-z0-9_-]*(?:\?[^?]*)?)?$`)
)

func ComputeShortID(id string) string {
	h := sha256.Sum256([]byte(id))
	return strings.ToUpper(hex.EncodeToString(h[:4]))
}

func defaultCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	enc, err := ianaindex.MIME.Encoding(charset)
	if err != nil || enc == nil {
		return input, nil
	}
	decoder := enc.NewDecoder()
	if decoder == nil {
		return input, nil
	}
	return decoder.Reader(input), nil
}

// DecodeHeader decodes MIME encoded-word headers using the provided decoder.
// It handles UTF-8, ISO-8859-1, and other character sets.
func DecodeHeader(dec *mime.WordDecoder, val string) string {
	if dec == nil {
		dec = &mime.WordDecoder{
			CharsetReader: defaultCharsetReader,
		}
	} else if dec.CharsetReader == nil {
		dec.CharsetReader = defaultCharsetReader
	}

	normalized := val
	normalized = strings.ReplaceAll(normalized, "=?UTF8?", "=?UTF-8?")
	normalized = strings.ReplaceAll(normalized, "=?utf8?", "=?utf-8?")

	// Strip any trailing incomplete/truncated encoded word
	normalized = incompleteEncodedWordRegexp.ReplaceAllString(normalized, "")

	locs := encodedWordRegexp.FindAllStringIndex(normalized, -1)
	if len(locs) == 0 {
		return normalized
	}

	type token struct {
		text      string
		isDecoded bool
	}

	var tokens []token
	lastEnd := 0

	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > lastEnd {
			tokens = append(tokens, token{
				text:      normalized[lastEnd:start],
				isDecoded: false,
			})
		}
		word := normalized[start:end]
		decoded, err := dec.Decode(word)
		if err == nil {
			tokens = append(tokens, token{
				text:      decoded,
				isDecoded: true,
			})
		} else {
			tokens = append(tokens, token{
				text:      word,
				isDecoded: false,
			})
		}
		lastEnd = end
	}
	if lastEnd < len(normalized) {
		tokens = append(tokens, token{
			text:      normalized[lastEnd:],
			isDecoded: false,
		})
	}

	// Filter out whitespace-only non-decoded tokens between two decoded tokens.
	// Use strings.Builder for efficient concatenation
	var sb strings.Builder
	for i := 0; i < len(tokens); i++ {
		if tokens[i].isDecoded && i+2 < len(tokens) && tokens[i+2].isDecoded {
			if !tokens[i+1].isDecoded && strings.TrimSpace(tokens[i+1].text) == "" {
				sb.WriteString(tokens[i].text)
				i++ // skip the middle whitespace token
				continue
			}
		}
		sb.WriteString(tokens[i].text)
	}

	return sb.String()
}

func ParseEmailAddress(fromHeader string) string {
	addr, err := mail.ParseAddress(fromHeader)
	if err == nil && addr != nil {
		return strings.ToLower(strings.TrimSpace(addr.Address))
	}
	if start := strings.Index(fromHeader, "<"); start != -1 {
		if end := strings.Index(fromHeader[start:], ">"); end != -1 {
			return strings.ToLower(strings.TrimSpace(fromHeader[start+1 : start+end]))
		}
	}
	return strings.ToLower(strings.TrimSpace(fromHeader))
}

func FormatEmailDate(t time.Time) string {
	return fmt.Sprintf("%2d/%2d/%02d", t.Month(), t.Day(), t.Year()%100)
}

func CleanSubject(s string) string {
	s = strings.ToLower(s)
	for {
		oldLen := len(s)
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "re:")
		s = strings.TrimPrefix(s, "fwd:")
		s = strings.TrimPrefix(s, "fw:")
		s = strings.TrimPrefix(s, "aw:")
		s = strings.TrimPrefix(s, "reply:")
		if len(s) == oldLen {
			break
		}
	}
	return strings.TrimSpace(s)
}

func StripSubjectPrefix(s string) string {
	for {
		trimmed := strings.TrimSpace(s)
		lower := strings.ToLower(trimmed)
		prefixes := []string{"re:", "fwd:", "fw:", "aw:", "reply:"}
		matched := false
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				s = trimmed[len(p):]
				matched = true
				break
			}
		}
		if !matched {
			break
		}
	}
	return strings.TrimSpace(s)
}
