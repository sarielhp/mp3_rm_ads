package app

import "github.com/fatih/color"

var (
	ColorCyan      = color.New(color.FgCyan)
	ColorGreen     = color.New(color.FgGreen)
	ColorYellow    = color.New(color.FgYellow)
	ColorRed       = color.New(color.FgRed, color.Bold)
	ColorPoli      = color.RGB(255, 127, 127).Add(color.Bold)
	ColorPurple    = color.New(color.FgMagenta)
	ColorBlacklist = color.New(color.FgRed, color.Bold)
	ColorWhite     = color.New(color.FgWhite)
	ColorBold      = color.New(color.Bold)

	ColorBoldCyan   = color.New(color.FgCyan, color.Bold)
	ColorBoldGreen  = color.New(color.FgGreen, color.Bold)
	ColorBoldYellow = color.New(color.FgYellow, color.Bold)
	ColorBoldRed    = color.New(color.FgRed, color.Bold)
	ColorBoldPurple = color.New(color.FgMagenta, color.Bold)
)
