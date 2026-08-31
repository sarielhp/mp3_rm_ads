package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
	"golang.org/x/text/unicode/bidi"
)

var (
	bold      = color.New(color.Bold).Sprint
	cyan      = color.New(color.FgCyan).Sprint
	green     = color.New(color.FgGreen).Sprint
	yellow    = color.New(color.FgYellow).Sprint
	red       = color.New(color.FgRed).Sprint
	boldCyan  = color.New(color.FgCyan, color.Bold).Sprint
	boldGreen = color.New(color.FgGreen, color.Bold).Sprint
)

func boldYellow(s string) string {
	return color.New(color.FgYellow, color.Bold).Sprint(s)
}

func boldRed(s string) string {
	return color.New(color.FgRed, color.Bold).Sprint(s)
}

func printSeparator() {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 1 {
		w = 80
	}
	fmt.Println(green(repeatStr("─", w-1)))
}

func hasRTL(s string) bool {
	for _, r := range s {
		if (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F) || (r >= 0x0600 && r <= 0x06FF) {
			return true
		}
	}
	return false
}

func reverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func displayName(name string) string {
	if !hasRTL(name) {
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
			sb.WriteString(reverseRunes(str))
		} else {
			sb.WriteString(str)
		}
	}
	return sb.String()
}
