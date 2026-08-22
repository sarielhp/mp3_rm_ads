package uicommon

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

type Segment struct {
	Text   string
	Fg     ColorID
	Bg     ColorID
	Strike bool
}

type ColoredString struct {
	Segments []Segment
}

func (c ColoredString) Render(t *Theme) string {
	if len(c.Segments) == 0 {
		return ""
	}
	var result string
	for _, seg := range c.Segments {
		result += segmentRender(seg, t)
	}
	return result
}

func segmentRender(seg Segment, t *Theme) string {
	var sb strings.Builder
	esc := string([]byte{27})
	if seg.Fg >= 0 || seg.Bg >= 0 {
		sb.WriteString(esc + "[")
		var codes []string
		if seg.Fg >= 0 {
			codes = append(codes, fmtFgCode(t.Get(seg.Fg)))
		}
		if seg.Bg >= 0 {
			codes = append(codes, fmtBgCode(t.Get(seg.Bg)))
		}
		sb.WriteString(strings.Join(codes, ";"))
		sb.WriteString("m")
	}
	if seg.Strike {
		sb.WriteString(esc + "[9m")
	}
	sb.WriteString(seg.Text)
	if seg.Strike {
		sb.WriteString(esc + "[29m")
	}
	if seg.Fg >= 0 || seg.Bg >= 0 {
		sb.WriteString(esc + "[0m")
	}
	return sb.String()
}

func fmtFgCode(color string) string {
	if strings.HasPrefix(color, "#") {
		h := color[1:]
		if len(h) == 6 {
			var r, g, b int
			fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b)
			return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
		}
	}
	return fmt.Sprintf("38;5;%s", color)
}

func fmtBgCode(color string) string {
	if strings.HasPrefix(color, "#") {
		h := color[1:]
		if len(h) == 6 {
			var r, g, b int
			fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b)
			return fmt.Sprintf("48;2;%d;%d;%d", r, g, b)
		}
	}
	return fmt.Sprintf("48;5;%s", color)
}

func (c ColoredString) Plain() string {
	var sb strings.Builder
	for _, seg := range c.Segments {
		sb.WriteString(seg.Text)
	}
	return sb.String()
}

func (c ColoredString) Width() int {
	w := 0
	for _, seg := range c.Segments {
		w += runewidth.StringWidth(seg.Text)
	}
	return w
}

func (c ColoredString) TruncateLeft(width int) ColoredString {
	if width <= 0 {
		for i := range c.Segments {
			c.Segments[i].Text = ""
		}
		return c
	}
	for i := range c.Segments {
		w := runewidth.StringWidth(c.Segments[i].Text)
		if w > width {
			runes := []rune(c.Segments[i].Text)
			for w > width {
				runes = runes[1:]
				w = runewidth.StringWidth(string(runes))
			}
			c.Segments[i].Text = string(runes)
		}
	}
	return c
}

func (c ColoredString) TruncateRight(width int) ColoredString {
	if width <= 0 {
		for i := range c.Segments {
			c.Segments[i].Text = ""
		}
		return c
	}
	for i := range c.Segments {
		w := runewidth.StringWidth(c.Segments[i].Text)
		if w > width {
			runes := []rune(c.Segments[i].Text)
			for w > width {
				runes = runes[:len(runes)-1]
				w = runewidth.StringWidth(string(runes))
			}
			c.Segments[i].Text = string(runes)
		}
	}
	return c
}

func (c ColoredString) PadLeft(width int) ColoredString {
	curW := c.Width()
	if curW >= width {
		return c
	}
	count := width - curW
	var firstFG, firstBG ColorID
	if len(c.Segments) > 0 {
		first := c.Segments[0]
		firstFG = first.Fg
		firstBG = first.Bg
	}
	c.Segments = append([]Segment{{Text: strings.Repeat(" ", count), Fg: firstFG, Bg: firstBG}}, c.Segments...)
	return c
}

func (c ColoredString) PadRight(width int) ColoredString {
	curW := c.Width()
	if curW >= width {
		return c
	}
	count := width - curW
	var lastFG, lastBG ColorID
	if len(c.Segments) > 0 {
		last := c.Segments[len(c.Segments)-1]
		lastFG = last.Fg
		lastBG = last.Bg
	}
	c.Segments = append(c.Segments, Segment{Text: strings.Repeat(" ", count), Fg: lastFG, Bg: lastBG})
	return c
}

func Plain(s string) ColoredString {
	if s == "" {
		return ColoredString{}
	}
	return ColoredString{Segments: []Segment{{Text: s, Fg: ColorDefault, Bg: ColorDefault}}}
}

func (c ColoredString) WithFg(fg ColorID) ColoredString {
	for i := range c.Segments {
		if c.Segments[i].Fg == ColorDefault {
			c.Segments[i].Fg = fg
		}
	}
	return c
}

func (c ColoredString) WithBg(bg ColorID) ColoredString {
	for i := range c.Segments {
		if c.Segments[i].Bg == ColorDefault {
			c.Segments[i].Bg = bg
		}
	}
	return c
}

func (c ColoredString) WithBgAll(bg ColorID) ColoredString {
	for i := range c.Segments {
		if c.Segments[i].Bg == ColorDefault {
			c.Segments[i].Bg = bg
		}
	}
	return c
}

func (c ColoredString) Len() int {
	var w int
	for _, s := range c.Segments {
		w += len(s.Text)
	}
	return w
}
