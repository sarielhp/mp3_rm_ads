package main

import (
	"fmt"
	"os"

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

func printSeparator() {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 1 {
		w = 80
	}
	fmt.Println(green(repeatStr("─", w-1)))
}

func displayName(name string) string {
	p := bidi.Paragraph{}
	p.SetString(name)
	order, err := p.Order()
	if err != nil {
		return name
	}
	if order.Direction() == bidi.RightToLeft {
		return "\u200E" + name
	}
	return name
}
