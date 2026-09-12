package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
	"golang.org/x/text/unicode/bidi"
)

var (
	Bold      = color.New(color.Bold).Sprint
	Cyan      = color.New(color.FgCyan).Sprint
	Blue      = color.New(color.FgBlue).Sprint
	Green     = color.New(color.FgGreen).Sprint
	Yellow    = color.New(color.FgYellow).Sprint
	Red       = color.New(color.FgRed).Sprint
	BoldCyan  = color.New(color.FgCyan, color.Bold).Sprint
	BoldBlue  = color.New(color.FgBlue, color.Bold).Sprint
	BoldGreen = color.New(color.FgGreen, color.Bold).Sprint
	Dim       = color.New(color.Faint).Sprint
)

func BoldYellow(s string) string {
	return color.New(color.FgYellow, color.Bold).Sprint(s)
}

func BoldRed(s string) string {
	return color.New(color.FgRed, color.Bold).Sprint(s)
}

func PrintSeparator() {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 1 {
		w = 80
	}
	fmt.Println(Green(RepeatStr("─", w-1)))
}

func HasRTL(s string) bool {
	for _, r := range s {
		if (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F) || (r >= 0x0600 && r <= 0x06FF) {
			return true
		}
	}
	return false
}

func ReverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func reverseRTLRun(str string) string {
	leading := 0
	for leading < len(str) && (str[leading] == ' ' || str[leading] == '\t') {
		leading++
	}
	trailing := len(str)
	for trailing > leading && (str[trailing-1] == ' ' || str[trailing-1] == '\t') {
		trailing--
	}
	leadSpaces := str[:leading]
	trailSpaces := str[trailing:]
	core := str[leading:trailing]
	return leadSpaces + ReverseRunes(core) + trailSpaces
}

func IsBaseRTL(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return false
		}
		if (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F) || (r >= 0x0600 && r <= 0x06FF) {
			return true
		}
	}
	return false
}

func DisplayName(name string) string {
	if !HasRTL(name) {
		return name
	}
	var p bidi.Paragraph
	if _, err := p.SetString(name, bidi.DefaultDirection(bidi.LeftToRight)); err != nil {
		return name
	}
	ordering, err := p.Order()
	if err != nil {
		return name
	}
	var sb strings.Builder
	for i := 0; i < ordering.NumRuns(); i++ {
		run := ordering.Run(i)
		str := run.String()
		if run.Direction() == bidi.RightToLeft {
			sb.WriteString(reverseRTLRun(str))
		} else {
			sb.WriteString(str)
		}
	}
	return sb.String()
}

func TruncateDisplayName(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return DisplayName(s)
	}
	if maxRunes <= 3 {
		return "..."
	}
	if IsBaseRTL(s) {
		sub := string(runes[:maxRunes-3])
		return "..." + DisplayName(sub)
	}
	return DisplayName(string(runes[:maxRunes-3])) + "..."
}
