package main

import "github.com/charmbracelet/lipgloss"

var (
	// Base Palette (Cyberpunk / Nord / Catppuccin inspired vibrant tones)
	colorCyan     = lipgloss.Color("#00f5d4") // Neon Cyan / Mint
	colorPurple   = lipgloss.Color("#7b2cbf") // Deep Violet
	colorMagenta  = lipgloss.Color("#f72585") // Neon Magenta / Pink
	colorPink     = lipgloss.Color("#ff70a6") // Soft Pink / Coral
	colorYellow   = lipgloss.Color("#ffd166") // Amber Gold
	colorGreen    = lipgloss.Color("#06d6a0") // Emerald Green
	colorRed      = lipgloss.Color("#ef476f") // Bright Coral Red
	colorBlue     = lipgloss.Color("#4cc9f0") // Electric Sky Blue
	colorLavender = lipgloss.Color("#c77dff") // Light Lavender
	colorDarkBg   = lipgloss.Color("#1a1b26") // Deep Background
	colorCardBg   = lipgloss.Color("#24283b") // Surface / Card
	colorBorder   = lipgloss.Color("#414868") // Subtle Border
	colorSubtext  = lipgloss.Color("#a9b1d6") // Crisp Subtext
	colorDim      = lipgloss.Color("#565f89") // Muted Dim

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
