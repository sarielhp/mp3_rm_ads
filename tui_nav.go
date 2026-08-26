package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type ToastType int

const (
	ToastInfo ToastType = iota
	ToastSuccess
	ToastWarning
	ToastError
)

type Toast struct {
	Message   string
	Type      ToastType
	ExpiresAt time.Time
}

func (m *tuiModel) showToast(msg string, t ToastType) {
	m.toast = &Toast{
		Message:   msg,
		Type:      t,
		ExpiresAt: time.Now().Add(3500 * time.Millisecond),
	}
	m.popupMsg = msg
	m.popupTimer = 15
}

func (m *tuiModel) renderTopNavBar() string {
	out := &strings.Builder{}

	type tabInfo struct {
		num    int
		label  string
		screen tuiScreen
	}

	playQueueCount := len(globalPlayer.GetUnifiedQueue())
	adQueueCount := 0
	for _, q := range m.queue {
		adQueueCount += len(q)
	}

	tabs := []tabInfo{
		{num: 1, label: "Podcasts", screen: screenPodcasts},
		{num: 2, label: "Player", screen: screenPlayer},
		{num: 3, label: fmt.Sprintf("Play Queue (%d)", playQueueCount), screen: screenPlayQueue},
		{num: 4, label: fmt.Sprintf("Ad Queue (%d)", adQueueCount), screen: screenAdQueue},
		{num: 5, label: "Timeline", screen: screenTimeline},
	}

	var tabStrs []string
	for _, t := range tabs {
		isActive := (m.screen == t.screen)
		if (m.screen == screenPodcastDetail || m.screen == screenEpisodeDetail || m.screen == screenTranscript) && t.screen == screenPodcasts {
			isActive = true
		}

		str := fmt.Sprintf("[%d: %s]", t.num, t.label)
		if isActive {
			tabStrs = append(tabStrs, tuiSelectedStyle.Render(str))
		} else {
			tabStrs = append(tabStrs, tuiDimStyle.Render(str))
		}
	}

	out.WriteString("  " + strings.Join(tabStrs, "  ") + "\n")

	// Breadcrumb path
	var pathParts []string
	pathParts = append(pathParts, "Podcasts")
	if m.podIdx >= 0 && m.podIdx < len(m.podcasts) && m.screen != screenPodcasts {
		pod := m.podcasts[m.podIdx]
		pathParts = append(pathParts, truncate(displayName(pod.name), 25))
		if m.screen == screenEpisodeDetail || m.screen == screenTranscript {
			eps := m.filteredEpisodes()
			if m.epIdx >= 0 && m.epIdx < len(eps) {
				pathParts = append(pathParts, truncate(displayName(eps[m.epIdx].displayTitle()), 30))
			}
		}
		if m.screen == screenTranscript {
			pathParts = append(pathParts, "Transcript")
		}
		if m.screen == screenTimeline {
			pathParts = append(pathParts, "Online Availability")
		}
	} else if m.screen == screenPlayer {
		pathParts = []string{"Audio Player"}
	} else if m.screen == screenPlayQueue {
		pathParts = []string{"Playback Queue"}
	} else if m.screen == screenAdQueue {
		pathParts = []string{"Ad Removal Queue"}
	}

	breadcrumb := "  " + tuiDimStyle.Render("Location: ") + tuiSubtitleStyle.Render(strings.Join(pathParts, " > "))
	out.WriteString(breadcrumb + "\n")

	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth) + "\n"))

	return out.String()
}

func (m *tuiModel) renderMiniPlayerBar() string {
	if globalPlayer.Current == nil || m.screen == screenPlayer {
		return ""
	}

	out := &strings.Builder{}
	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth) + "\n"))

	statusIcon := "▶"
	if globalPlayer.IsPaused {
		statusIcon = "⏸"
	}

	curT := formatPlayerTime(globalPlayer.Position)
	totT := formatPlayerTime(globalPlayer.Duration)
	title := truncate(displayName(globalPlayer.Current.Title), max(10, m.width/3))
	pod := truncate(displayName(globalPlayer.Current.Podcast), 20)

	trackInfo := fmt.Sprintf("%s [%s / %s] %s • %s [Vol: %d%%]",
		statusIcon, curT, totT, title, pod, globalPlayer.Volume)

	statusLine := tuiPlayerPlaying.Render(" " + trackInfo + " ")
	if globalPlayer.IsPaused {
		statusLine = tuiPlayerPaused.Render(" " + trackInfo + " ")
	}

	hintLine := tuiDimStyle.Render("  (Space: Pause, ←/→: ±30s, +/-: Vol, s: Spk, 2: Player)")
	out.WriteString("  " + statusLine + hintLine + "\n")

	return out.String()
}

func (m *tuiModel) renderToastNotification() string {
	if m.toast == nil {
		return ""
	}
	if time.Now().After(m.toast.ExpiresAt) {
		m.toast = nil
		return ""
	}

	icon := "ℹ"
	style := tuiCyanStyle
	switch m.toast.Type {
	case ToastSuccess:
		icon = "✓"
		style = tuiGreenStyle
	case ToastWarning:
		icon = "⚠"
		style = tuiYellowStyle
	case ToastError:
		icon = "✗"
		style = tuiRedStyle
	}

	toastBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorDarkBg).
		Background(style.GetForeground()).
		Padding(0, 2).
		Render(fmt.Sprintf(" %s %s ", icon, m.toast.Message))

	return "\n  " + toastBox
}
