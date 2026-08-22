package main

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Base Palette (Cyberpunk / Nord / Catppuccin inspired vibrant tones)
	colorCyan     = lipgloss.Color("#00f5d4")
	colorPurple   = lipgloss.Color("#7b2cbf")
	colorMagenta  = lipgloss.Color("#f72585")
	colorPink     = lipgloss.Color("#ff70a6")
	colorYellow   = lipgloss.Color("#ffd166")
	colorGreen    = lipgloss.Color("#06d6a0")
	colorRed      = lipgloss.Color("#ef476f")
	colorBlue     = lipgloss.Color("#4cc9f0")
	colorLavender = lipgloss.Color("#c77dff")
	colorDarkBg   = lipgloss.Color("#1a1b26")
	colorCardBg   = lipgloss.Color("#24283b")
	colorBorder   = lipgloss.Color("#414868")
	colorSubtext  = lipgloss.Color("#a9b1d6")
	colorDim      = lipgloss.Color("#565f89")

	// Text & Header Styles
	tuiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	tuiHeaderBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDarkBg).
			Background(colorCyan).
			Padding(0, 1)

	tuiSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Italic(true)

	tuiStatStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	tuiDimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	tuiSubtextStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	tuiGreenStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	tuiRedStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	tuiYellowStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	tuiBlueStyle = lipgloss.NewStyle().
			Foreground(colorBlue)

	tuiMagentaStyle = lipgloss.NewStyle().
			Foreground(colorMagenta)

	tuiLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	tuiSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorDarkBg).
				Background(colorCyan)

	tuiHelpStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	tuiSearchStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDarkBg).
			Background(colorYellow).
			Padding(0, 1)

	tuiPopupStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDarkBg).
			Background(colorPurple).
			Padding(0, 1)

	// Status Badges & Pills
	tuiBadgeAdFree = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDarkBg).
			Background(colorGreen).
			Padding(0, 1)

	tuiBadgeHasAds = lipgloss.NewStyle().
			Foreground(colorRed).
			Background(lipgloss.Color("#3b2230")).
			Padding(0, 1)

	tuiBadgeQueued = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDarkBg).
			Background(colorYellow).
			Padding(0, 1)

	tuiBadgeType = lipgloss.NewStyle().
			Foreground(colorLavender).
			Background(lipgloss.Color("#2e2a4a")).
			Padding(0, 1)

	tuiBadgeDuration = lipgloss.NewStyle().
				Foreground(colorBlue).
				Background(lipgloss.Color("#1f2d3d")).
				Padding(0, 1)

	tuiBadgeCount = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMagenta).
			Background(lipgloss.Color("#361f36")).
			Padding(0, 1)

	// Section Boxes / Cards
	tuiSectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMagenta)

	tuiCardBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	tuiDividerStyle = lipgloss.NewStyle().
			Foreground(colorBorder)
)

func applyTUIColorConfig(cfg *TUIColorConfig) {
	if cfg == nil {
		return
	}
	if cfg.Cyan != "" {
		colorCyan = lipgloss.Color(cfg.Cyan)
	}
	if cfg.Purple != "" {
		colorPurple = lipgloss.Color(cfg.Purple)
	}
	if cfg.Magenta != "" {
		colorMagenta = lipgloss.Color(cfg.Magenta)
	}
	if cfg.Pink != "" {
		colorPink = lipgloss.Color(cfg.Pink)
	}
	if cfg.Yellow != "" {
		colorYellow = lipgloss.Color(cfg.Yellow)
	}
	if cfg.Green != "" {
		colorGreen = lipgloss.Color(cfg.Green)
	}
	if cfg.Red != "" {
		colorRed = lipgloss.Color(cfg.Red)
	}
	if cfg.Blue != "" {
		colorBlue = lipgloss.Color(cfg.Blue)
	}
	if cfg.Lavender != "" {
		colorLavender = lipgloss.Color(cfg.Lavender)
	}
	if cfg.DarkBg != "" {
		colorDarkBg = lipgloss.Color(cfg.DarkBg)
	}
	if cfg.CardBg != "" {
		colorCardBg = lipgloss.Color(cfg.CardBg)
	}
	if cfg.Border != "" {
		colorBorder = lipgloss.Color(cfg.Border)
	}
	if cfg.Subtext != "" {
		colorSubtext = lipgloss.Color(cfg.Subtext)
	}
	if cfg.Dim != "" {
		colorDim = lipgloss.Color(cfg.Dim)
	}
}
