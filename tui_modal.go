package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) drawHelpModal() string {
	boxWidth := min(72, max(40, m.width-8))

	var lines []string
	lines = append(lines, tuiTitleStyle.Render("KEYBOARD SHORTCUTS & NAVIGATION"))
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, "")

	col1 := []string{
		tuiLabelStyle.Render("Navigation & Tabs:"),
		"  1-5 / F1-F4   Switch top tabs",
		"  ↑/k, ↓/j      Move selection",
		"  Enter         Open / Select / Details",
		"  Esc           Back to previous view",
		"  /             Fuzzy search / filter",
		"  ? / h         Toggle this help modal",
		"  q             Quit application",
	}

	col2 := []string{
		tuiLabelStyle.Render("Playback & Volume:"),
		"  Space         Play / Pause",
		"  p             Play selected episode",
		"  ← / →         Seek backward / forward 30s",
		"  + / -         Volume Up / Down",
		"  m             Mute / Unmute",
		"  s             Cycle audio speaker/sink",
		"  n             Next track in queue",
	}

	col3 := []string{
		tuiLabelStyle.Render("Podcast & Queues:"),
		"  c             Ad removal policy modal",
		"  t             Open full transcript view",
		"  e / o         Online release timeline",
		"  v / Space     Multi-select episode",
		"  r             Queue for ad removal",
		"  d / x         Remove item from queue",
		"  i             Toggle cover art (Kitty)",
	}

	for _, sec := range [][]string{col1, col2, col3} {
		lines = append(lines, sec...)
		lines = append(lines, "")
	}

	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("Press Esc or ? to close this help window"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(1, 2).
		Width(boxWidth).
		Render(content)
}

func (m *tuiModel) drawAdPolicyModal() string {
	if m.podIdx >= len(m.podcasts) {
		return ""
	}
	pod := m.podcasts[m.podIdx]
	boxWidth := min(65, max(40, m.width-8))

	var lines []string
	lines = append(lines, tuiTitleStyle.Render("AD REMOVAL POLICY"))
	lines = append(lines, tuiSubtitleStyle.Render(truncate("for "+displayName(pod.name), boxWidth-6)))
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, "")

	options := []struct {
		num  int
		mode string
		name string
		desc string
	}{
		{1, AdRemovalNone, "1. No Ad Removal (none)", "Leave audio files completely untouched (default)"},
		{2, AdRemovalLatest, "2. Latest Episode Only (latest)", "Auto-clean only the newest downloaded episode"},
		{3, AdRemovalAll, "3. All Episodes (all)", "Remove ads from every episode in this directory"},
	}

	curMode := normalizeAdRemovalMode(pod.config.AdRemoval)
	for i, opt := range options {
		isSel := (i == m.policyModalIdx)
		isCurrent := (opt.mode == curMode)

		check := "( )"
		if isCurrent {
			check = "(•)"
		}

		optTitle := fmt.Sprintf("%s %s", check, opt.name)
		optDesc := fmt.Sprintf("    %s", tuiDimStyle.Render(opt.desc))

		if isSel {
			lines = append(lines, tuiSelectedStyle.Render(" "+optTitle+" "))
		} else {
			lines = append(lines, tuiLabelStyle.Render(optTitle))
		}
		lines = append(lines, optDesc)
		lines = append(lines, "")
	}

	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("↑/↓ Select │ 1-3 Choose │ Enter Apply & Save │ Esc Cancel"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(1, 2).
		Width(boxWidth).
		Render(content)
}

func renderVisualAdCutTimeline(totalDuration float64, cuts []CutEntry, barWidth int) string {
	if totalDuration <= 0 || barWidth < 10 {
		return ""
	}

	if barWidth > 60 {
		barWidth = 60
	}

	grid := make([]rune, barWidth)
	for i := range grid {
		grid[i] = '█'
	}

	var totalCutSec float64
	for _, c := range cuts {
		st := max(0, min(totalDuration, c.StartSec))
		en := max(0, min(totalDuration, c.EndSec))
		if en > st {
			totalCutSec += (en - st)
			stIdx := int((st / totalDuration) * float64(barWidth))
			enIdx := int((en / totalDuration) * float64(barWidth))
			for k := stIdx; k <= enIdx && k < barWidth; k++ {
				grid[k] = '░'
			}
		}
	}

	out := &strings.Builder{}
	out.WriteString(tuiLabelStyle.Render("Audio Cut Map: ") + "[")
	for _, r := range grid {
		if r == '█' {
			out.WriteString(tuiGreenStyle.Render(string(r)))
		} else {
			out.WriteString(tuiRedStyle.Render(string(r)))
		}
	}
	out.WriteString("]\n")

	pctCut := 0.0
	if totalDuration > 0 {
		pctCut = (totalCutSec / totalDuration) * 100
	}
	summary := fmt.Sprintf("  %s %s (%.1f%% cut, %d ad blocks)",
		tuiGreenStyle.Render("█ Keep"),
		tuiRedStyle.Render("░ Removed"),
		pctCut,
		len(cuts),
	)
	out.WriteString(tuiDimStyle.Render(summary))
	return out.String()
}

func (m *tuiModel) handlePodcastConfigToggle() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	pod.config.AdRemoval = cycleAdRemovalMode(pod.config.AdRemoval)
	if err := savePodcastConfig(pod.dir, pod.config); err != nil {
		m.showToast("Failed to save config: "+err.Error(), ToastError)
		return
	}
	m.showToast("Ad removal: "+adRemovalModeLabel(pod.config.AdRemoval)+" (saved)", ToastSuccess)
}

func (m *tuiModel) openPolicyModal() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	cur := normalizeAdRemovalMode(pod.config.AdRemoval)
	if cur == AdRemovalLatest {
		m.policyModalIdx = 1
	} else if cur == AdRemovalAll {
		m.policyModalIdx = 2
	} else {
		m.policyModalIdx = 0
	}
	m.showPolicyModal = true
}

func (m *tuiModel) applyPolicyModal() {
	if m.podIdx >= len(m.podcasts) {
		m.showPolicyModal = false
		return
	}
	pod := &m.podcasts[m.podIdx]
	modes := []string{AdRemovalNone, AdRemovalLatest, AdRemovalAll}
	if m.policyModalIdx >= 0 && m.policyModalIdx < len(modes) {
		pod.config.AdRemoval = modes[m.policyModalIdx]
		if err := savePodcastConfig(pod.dir, pod.config); err != nil {
			m.showToast("Failed to save config: "+err.Error(), ToastError)
		} else {
			m.showToast("Saved ad policy: "+adRemovalModeLabel(pod.config.AdRemoval), ToastSuccess)
		}
	}
	m.showPolicyModal = false
}
