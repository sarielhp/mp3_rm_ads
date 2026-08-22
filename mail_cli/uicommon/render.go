package uicommon

import (
	"fmt"
	"strings"

	"mail_cli/email"
)

type StyBuilder struct {
	segments []Segment
	curFg    ColorID
	curBg    ColorID
}

func NewStyBuilder() *StyBuilder {
	return &StyBuilder{}
}

func (b *StyBuilder) PushString(s string) *StyBuilder {
	if s == "" {
		return b
	}
	if len(b.segments) > 0 {
		last := &b.segments[len(b.segments)-1]
		if last.Fg == b.curFg && last.Bg == b.curBg {
			last.Text += s
			return b
		}
	}
	b.segments = append(b.segments, Segment{Text: s, Fg: b.curFg, Bg: b.curBg})
	return b
}

func (b *StyBuilder) PushSegment(s string, fg, bg ColorID) *StyBuilder {
	b.curFg, b.curBg = fg, bg
	return b.PushString(s)
}

func (b *StyBuilder) Build() ColoredString {
	return ColoredString{Segments: b.segments}
}

func JoinCols(collections ...ColoredString) ColoredString {
	var all []Segment
	for _, cs := range collections {
		all = append(all, cs.Segments...)
	}
	return ColoredString{Segments: all}
}

func RenderEmailRow(e email.Email, totalWidth, subjWidth, idx int, t *Theme, highlight bool, ghosted bool) ColoredString {
	from := BidiDisplay(FormatSender(e.FromRaw))
	if e.ThreadHasReplies && e.ThreadCollapsed && e.ThreadSenderSummary != "" {
		from = e.ThreadSenderSummary
	}
	if from == "" {
		from = "(Unknown Sender)"
	}
	date := e.FormattedDate
	highlightStr := fmt.Sprintf("%4d", idx)
	var counterColored ColoredString
	if highlight {
		counterColored = Plain(highlightStr).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		counterColored = Plain(highlightStr).WithFg(ColorDim)
	}
	var dateCS ColoredString
	if highlight {
		dateCS = Plain(date).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		dateCS = Plain(date).WithFg(ColorDate).WithBg(ColorDefault)
	}
	dateCS = dateCS.TruncateLeft(8).PadLeft(8)
	var senderCS ColoredString
	if highlight {
		senderCS = Plain(from).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		senderCS = Plain(from).WithFg(ColorSender).WithBg(ColorDefault)
	}
	senderCS = senderCS.TruncateRight(20).PadRight(20)
	subjText := e.Subject
	if e.ThreadDepth > 0 || e.ThreadPrefix != "" {
		cleaned := email.StripSubjectPrefix(subjText)
		if cleaned != "" {
			subjText = cleaned
		}
	}
	subjText = BidiDisplay(subjText)
	var subjCS ColoredString
	if e.ThreadPrefix != "" {
		prefix := e.ThreadPrefix
		var prefixCS ColoredString
		if highlight {
			prefixCS = Plain(prefix).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			prefixCS = Plain(prefix).WithFg(ColorHighlightFg).WithBg(ColorDefault)
		}
		var textCS ColoredString
		if highlight {
			textCS = Plain(subjText).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			fgColor := ColorSubject
			if e.IsSpam || e.IsPolitical || e.IsBlacklisted {
				fgColor = getBadgeFg(e)
			}
			textCS = Plain(subjText).WithFg(fgColor).WithBg(ColorDefault)
		}
		subjCS = JoinCols(prefixCS, textCS)
	} else if e.ThreadDepth > 0 {
		prefix := strings.Repeat("  ", e.ThreadDepth-1) + "└─> "
		var prefixCS ColoredString
		if highlight {
			prefixCS = Plain(prefix).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			prefixCS = Plain(prefix).WithFg(ColorHighlightFg).WithBg(ColorDefault)
		}
		var textCS ColoredString
		if highlight {
			textCS = Plain(subjText).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			fgColor := ColorSubject
			if e.IsSpam || e.IsPolitical || e.IsBlacklisted {
				fgColor = getBadgeFg(e)
			}
			textCS = Plain(subjText).WithFg(fgColor).WithBg(ColorDefault)
		}
		subjCS = JoinCols(prefixCS, textCS)
	} else {
		if highlight {
			subjCS = Plain(subjText).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			fgColor := ColorSubject
			if e.IsSpam || e.IsPolitical || e.IsBlacklisted {
				fgColor = getBadgeFg(e)
			}
			subjCS = Plain(subjText).WithFg(fgColor).WithBg(ColorDefault)
		}
	}
	if e.ThreadHasReplies && e.ThreadCollapsed {
		badgeText := fmt.Sprintf(" [+%d]", e.ThreadRepliesCount)
		var badgeCS ColoredString
		if highlight {
			badgeCS = Plain(badgeText).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			badgeCS = Plain(badgeText).WithFg(ColorHighlightFg).WithBg(ColorDefault)
		}
		subjCS = JoinCols(subjCS, badgeCS)
	}

	if e.IsSpam || e.IsPolitical || e.IsBlacklisted {
		badge := getBadgeText(e)
		var badgeCS ColoredString
		var spaceCS ColoredString
		if highlight {
			badgeCS = Plain(badge).WithFg(getBadgeFg(e)).WithBg(ColorHighlightBg)
			spaceCS = Plain(" ").WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
		} else {
			badgeCS = Plain(badge).WithFg(getBadgeFg(e)).WithBg(ColorDefault)
			spaceCS = Plain(" ").WithFg(ColorDefault).WithBg(ColorDefault)
		}
		subjCS = JoinCols(badgeCS, spaceCS, subjCS)
	}

	subjCS = subjCS.TruncateRight(subjWidth)
	var sepCS ColoredString
	if highlight {
		sepCS = Plain("  ").WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		sepCS = Plain("  ")
	}
	var countCS ColoredString
	var countStr string
	showCount := (e.ThreadDepth == 0 && e.ThreadRepliesCount > 0)
	if showCount {
		countStr = fmt.Sprintf("%2d", e.ThreadRepliesCount+1)
	} else {
		countStr = "  "
	}
	if highlight {
		countCS = Plain(countStr).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		if showCount {
			countCS = Plain(countStr).WithFg(ColorBorder).WithBg(ColorDefault)
		} else {
			countCS = Plain(countStr).WithFg(ColorDefault).WithBg(ColorDefault)
		}
	}
	var flagText string
	if e.HasAttachment {
		flagText = "📎"
	} else if e.IsReplied {
		flagText = "r"
	} else if !e.IsRead {
		flagText = "N"
	} else {
		flagText = " "
	}

	var nBadgeCS ColoredString
	if highlight {
		nBadgeCS = Plain(flagText).WithFg(ColorHighlightFg).WithBg(ColorHighlightBg)
	} else {
		fg := ColorDefault
		if e.HasAttachment || e.IsReplied {
			fg = ColorHighlightFg
		} else if !e.IsRead {
			fg = ColorDate
		}
		nBadgeCS = Plain(flagText).WithFg(fg).WithBg(ColorDefault)
	}
	nBadgeCS = nBadgeCS.TruncateRight(2).PadRight(2)
	row := JoinCols(counterColored, sepCS, dateCS, sepCS, senderCS, sepCS, countCS, sepCS, nBadgeCS, sepCS, subjCS)
	if ghosted {
		for i := range row.Segments {
			row.Segments[i].Strike = true
			row.Segments[i].Fg = ColorDim
		}
	} else if e.HasICS && !highlight {
		for i := range row.Segments {
			row.Segments[i].Bg = ColorICS
		}
	}
	if highlight {
		row = row.PadRight(totalWidth)
	}
	return row
}

func getBadgeText(e email.Email) string {
	if e.IsBlacklisted {
		return "[BLACKLIST]"
	}
	if e.IsPolitical {
		return "[POLI]"
	}
	if e.IsSpam {
		return "[SPAM]"
	}
	return ""
}

func getBadgeFg(e email.Email) ColorID {
	if e.IsBlacklisted {
		return ColorSpam
	}
	if e.IsPolitical {
		return ColorPoli
	}
	if e.IsSpam {
		return ColorSpam
	}
	return ColorSubject
}

func RenderEndOfMsgGreen(width int, t *Theme) string {
	const label = " < End of message > "
	b := NewStyBuilder()
	pad := max(0, (width-len(label))/2)
	b.PushSegment(strings.Repeat(" ", pad), ColorDefault, ColorMsgEndBg)
	b.PushSegment(label, ColorDefault, ColorMsgEndBg)
	b.PushSegment(strings.Repeat(" ", width-len(label)-pad), ColorDefault, ColorMsgEndBg)
	return b.Build().Render(t)
}

func MakeLabelLine(e TreeEntry, isCursor bool, t *Theme, width int) string {
	var segs []Segment
	if isCursor {
		segs = append(segs, Segment{Text: "  ", Fg: ColorHighlightFg, Bg: ColorHighlightBg})
	} else {
		segs = append(segs, Segment{Text: "  ", Fg: ColorDefault, Bg: ColorDefault})
	}
	frame := e.Prefix
	if frame != "" {
		var fg, bg ColorID = ColorBorder, ColorDefault
		if isCursor {
			fg, bg = ColorHighlightFg, ColorHighlightBg
		}
		segs = append(segs, Segment{Text: frame, Fg: fg, Bg: bg})
	}
	var arrow string
	if e.Node != nil && len(e.Node.Children) > 0 {
		if e.Node.Expanded {
			arrow = "[-] "
		} else {
			arrow = "[+] "
		}
	}
	if arrow != "" {
		if isCursor {
			segs = append(segs, Segment{Text: arrow, Fg: ColorHighlightFg, Bg: ColorHighlightBg})
		} else {
			segs = append(segs, Segment{Text: arrow, Fg: ColorDefault, Bg: ColorDefault})
		}
	}
	folderText := e.Text
	if e.Node != nil && len(e.Node.Children) > 0 {
		folderText += fmt.Sprintf(" [%d]", CountAllDescendants(e.Node))
	}
	if isCursor {
		segs = append(segs, Segment{Text: folderText, Fg: ColorHighlightFg, Bg: ColorHighlightBg})
	} else {
		segs = append(segs, Segment{Text: folderText, Fg: ColorDefault, Bg: ColorDefault})
	}
	var count string
	if e.Node != nil {
		if e.Node.MessagesUnread > 0 {
			count = fmt.Sprintf(" (%d/%d)", e.Node.MessagesUnread, e.Node.MessagesTotal)
		} else {
			count = fmt.Sprintf(" (%d)", e.Node.MessagesTotal)
		}
	}
	if count != "" {
		if isCursor {
			segs = append(segs, Segment{Text: count, Fg: ColorHighlightFg, Bg: ColorHighlightBg})
		} else {
			segs = append(segs, Segment{Text: count, Fg: ColorSubject, Bg: ColorDefault})
		}
	}
	row := ColoredString{Segments: segs}
	if isCursor {
		row = row.PadRight(max(0, width-2))
	}
	return row.Render(t)
}
