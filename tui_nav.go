package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	dlQueueCount := len(GetDownloadQueueItems())

	tabs := []tabInfo{
		{num: 1, label: "Podcasts", screen: screenPodcasts},
		{num: 2, label: "Player", screen: screenPlayer},
		{num: 3, label: fmt.Sprintf("Play Queue (%d)", playQueueCount), screen: screenPlayQueue},
		{num: 4, label: fmt.Sprintf("Ad Queue (%d)", adQueueCount), screen: screenAdQueue},
		{num: 5, label: fmt.Sprintf("DL Queue (%d)", dlQueueCount), screen: screenDownloadQueue},
		{num: 6, label: "Timeline", screen: screenTimeline},
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

	var pathParts []string
	pathParts = append(pathParts, "Podcasts")
	if m.podIdx >= 0 && m.podIdx < len(m.podcasts) && m.screen != screenPodcasts && m.screen != screenLatestEpisodes && m.screen != screenDownloadQueue {
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
	} else if m.screen == screenDownloadQueue {
		pathParts = []string{"Download Queue"}
	} else if m.screen == screenLatestEpisodes {
		pathParts = []string{"Latest Episodes"}
	}

	breadcrumb := "  " + tuiDimStyle.Render("Location: ") + tuiSubtitleStyle.Render(strings.Join(pathParts, " > "))
	out.WriteString(breadcrumb + "\n")

	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

	return out.String()
}

func (m *tuiModel) renderMiniPlayerBar() string {
	if globalPlayer.Current == nil || m.screen == screenPlayer {
		return ""
	}

	out := &strings.Builder{}
	dividerWidth := max(20, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

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

func (m *tuiModel) handleSortToggle() {
	if m.screen != screenPodcastDetail {
		return
	}
	if m.podIdx >= len(m.podcasts) {
		return
	}
	eps := m.podcasts[m.podIdx].episodes
	for i, j := 0, len(eps)-1; i < j; i, j = i+1, j-1 {
		eps[i], eps[j] = eps[j], eps[i]
	}
	m.podcasts[m.podIdx].episodes = eps
	m.showPopup("Sort order reversed")
}

func (m *tuiModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		switch m.screen {
		case screenEpisodeDetail:
			if m.descScroll > 0 {
				m.descScroll--
			}
		case screenTranscript:
			if m.transcriptScroll > 0 {
				m.transcriptScroll--
			}
		case screenTimeline:
			if m.timelineScroll > 0 {
				m.timelineScroll--
			}
		default:
			m.handleUp()
		}
	case tea.MouseWheelDown:
		switch m.screen {
		case screenEpisodeDetail:
			m.descScroll++
		case screenTranscript:
			m.transcriptScroll++
		case screenTimeline:
			m.timelineScroll++
		default:
			m.handleDown()
		}
	}
	return m, nil
}

func (m *tuiModel) handleQueueToggle() {
	if m.screen != screenPodcastDetail && m.screen != screenEpisodeDetail {
		return
	}
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]
	entries := m.queue[pod.dir]
	if entries == nil {
		entries = []string{}
	}
	found := false
	for i, e := range entries {
		if e == ep.filename {
			m.queue[pod.dir] = append(entries[:i], entries[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		if ep.hasAdsRemoved {
			m.showPopup("Episode already has ads removed")
			return
		}
		m.queue[pod.dir] = append(entries, ep.filename)
		m.showPopup("Added to ad removal queue")
	} else {
		m.showPopup("Removed from ad removal queue")
	}
	if m.bk != nil && m.bk.SaveQueue != nil {
		m.bk.SaveQueue(pod.dir, m.queue[pod.dir])
	}
}
