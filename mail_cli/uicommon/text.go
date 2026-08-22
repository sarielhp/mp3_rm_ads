package uicommon

import (
	"mime"
	"net/mail"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/text/unicode/bidi"
)

func hasRTL(s string) bool {
	for _, r := range s {
		if r >= 0x0590 && r <= 0x06FF {
			return true
		}
	}
	return false
}

func BidiDisplay(s string) string {
	if s == "" || !hasRTL(s) {
		return s
	}
	var p bidi.Paragraph
	if _, err := p.SetString(s, bidi.DefaultDirection(bidi.RightToLeft)); err != nil {
		return s
	}
	o, err := p.Order()
	if err != nil {
		return s
	}
	var sb strings.Builder
	for i := 0; i < o.NumRuns(); i++ {
		r := o.Run(i)
		sb.WriteString(r.String())
	}
	return sb.String()
}

func ReverseHebrewRuns(s string) string {
	var result []rune
	var run []rune
	inRun := false

	isHebrew := func(r rune) bool {
		return r >= 0x0590 && r <= 0x05FF
	}

	isNeutral := func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == ':' || r == ',' || r == '.' || r == '!' || r == '?' || (r >= '0' && r <= '9')
	}

	flushRun := func(atEnd bool) {
		if len(run) > 0 {
			hasHebrew := false
			for _, r := range run {
				if isHebrew(r) {
					hasHebrew = true
					break
				}
			}

			if hasHebrew {
				trimIdx := len(run)
				for trimIdx > 0 && isNeutral(run[trimIdx-1]) {
					trimIdx--
				}

				for i := trimIdx - 1; i >= 0; i-- {
					result = append(result, run[i])
				}
				for i := trimIdx; i < len(run); i++ {
					result = append(result, run[i])
				}
			} else {
				result = append(result, run...)
			}
			run = nil
		}
		inRun = false
	}

	for _, r := range s {
		if isHebrew(r) || (inRun && isNeutral(r)) {
			inRun = true
			run = append(run, r)
		} else {
			flushRun(false)
			result = append(result, r)
		}
	}
	flushRun(true)

	return string(result)
}

func DecDecode(dec *mime.WordDecoder, v string) string {
	d, err := dec.DecodeHeader(v)
	if err != nil {
		return v
	}
	return d
}

func FormatSender(fromRaw string) string {
	name, email := parseSenderInfo(fromRaw)
	if name != "" {
		return name
	}
	return email
}

func FormatSenderVerbose(fromRaw string) string {
	name, email := parseSenderInfo(fromRaw)
	if name != "" {
		return name + " <" + email + ">"
	}
	return email
}

func parseSenderInfo(fromHeader string) (name string, email string) {
	addr, err := mail.ParseAddress(fromHeader)
	if err == nil && addr != nil {
		return strings.TrimSpace(addr.Name), strings.ToLower(strings.TrimSpace(addr.Address))
	}
	return "", strings.ToLower(strings.TrimSpace(fromHeader))
}

func ParseSenderInfo(fromHeader string) (name string, email string) {
	return parseSenderInfo(fromHeader)
}

func WrapText(text string, width int) string {
	var lines []string
	for _, l := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' }) {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		wrapped := TruncateLines(trimmed, width)
		lines = append(lines, wrapped)
	}
	return strings.Join(lines, "\n")
}

func TruncateLines(s string, maxLen int) string {
	var out strings.Builder
	words := strings.Fields(s)
	var line []rune
	for _, word := range words {
		wordRunes := []rune(word)
		lineWidth := runewidth.StringWidth(string(line))
		wordWidth := runewidth.StringWidth(word)
		if len(line) > 0 && lineWidth+1+wordWidth > maxLen {
			if len(line) > 0 {
				out.WriteString(string(line) + "\n")
			}
			wordRuneWidth := runewidth.StringWidth(string(wordRunes))
			if wordRuneWidth > maxLen {
				var truncated []rune
				w := 0
				for _, r := range wordRunes {
					if w+runewidth.RuneWidth(r) > maxLen {
						break
					}
					truncated = append(truncated, r)
					w += runewidth.RuneWidth(r)
				}
				out.WriteString(string(truncated))
				line = nil
			} else {
				out.WriteString(word)
				line = nil
			}
		} else {
			if len(line) > 0 {
				line = append(line, ' ')
			}
			line = append(line, []rune(word)...)
		}
	}
	if len(line) > 0 {
		out.WriteString(string(line))
	}
	return out.String()
}

func ColorReplyLines(wrapped string) ColoredString {
	lines := strings.Split(wrapped, "\n")
	var segments []Segment
	for i, line := range lines {
		depth := CountQuoteDepth(line)
		var fg ColorID
		switch depth {
		case 0:
			fg = ColorDefault
		case 1:
			fg = ColorSender
		case 2:
			fg = ColorQuote2
		default:
			fg = ColorQuote3
		}
		displayLine := BidiDisplay(line)
		if i > 0 {
			displayLine = "\n" + displayLine
		}
		segments = append(segments, Segment{Text: displayLine, Fg: fg, Bg: ColorDefault})
	}
	return ColoredString{Segments: segments}
}

func CountQuoteDepth(line string) int {
	runes := []rune(line)
	count := 0
	for i, r := range runes {
		if i > 20 {
			break
		}
		if r == '>' {
			count++
		}
	}
	return count
}

func ColorReplyLinesRaw(body string, wrapWidth int) ColoredString {
	return ColorReplyLinesWrap(body, wrapWidth)
}

func ColorReplyLinesWrap(rawBody string, wrapWidth int) ColoredString {
	var segments []Segment
	isFirst := true
	for _, line := range strings.Split(rawBody, "\n") {
		depth := CountQuoteDepth(line)
		var fg ColorID
		switch depth {
		case 0:
			fg = ColorDefault
		case 1:
			fg = ColorSender
		case 2:
			fg = ColorQuote2
		default:
			fg = ColorQuote3
		}
		if strings.TrimSpace(line) == "" {
			text := BidiDisplay(line)
			if !isFirst {
				text = "\n" + text
			}
			segments = append(segments, Segment{Text: text, Fg: ColorDefault, Bg: ColorDefault})
			isFirst = false
			continue
		}
		words := strings.Fields(line)
		wrappedLine := ""
		for _, word := range words {
			if wrappedLine == "" {
				wrappedLine = word
			} else if runewidth.StringWidth(wrappedLine)+1+runewidth.StringWidth(word) > wrapWidth {
				text := BidiDisplay(wrappedLine)
				if !isFirst {
					text = "\n" + text
				}
				segments = append(segments, Segment{Text: text, Fg: fg, Bg: ColorDefault})
				isFirst = false
				wrappedLine = word
			} else {
				wrappedLine += " " + word
			}
		}
		if wrappedLine != "" {
			text := BidiDisplay(wrappedLine)
			if !isFirst {
				text = "\n" + text
			}
			segments = append(segments, Segment{Text: text, Fg: fg, Bg: ColorDefault})
			isFirst = false
		}
	}
	return ColoredString{Segments: segments}
}
