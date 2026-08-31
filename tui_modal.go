package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) drawHelpModal() string {
	boxWidth := min(82, max(56, m.width-6))
	colW := (boxWidth - 7) / 2

	var lines []string
	lines = append(lines, tuiTitleStyle.Render("KEYBOARD SHORTCUTS & NAVIGATION"))
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, "")

	leftLines := []string{
		tuiLabelStyle.Render("Navigation & Tabs:"),
		"  1-5 / F1-F3  Switch tabs",
		"  ↑/k, ↓/j     Navigate rows",
		"  Enter        Open / Select",
		"  Esc / q      Back / Close",
		"  /            Search / filter",
		"  ? / h        Toggle help",
		"  F12          Take snapshot",
		"",
		tuiLabelStyle.Render("Episode & Notes:"),
		"  F4           Toggle player",
		"  t            Transcript",
		"  Tab          Cycle format",
		"  ↑/↓          Scroll notes",
	}

	rightLines := []string{
		tuiLabelStyle.Render("Playback & Volume:"),
		"  Space        Play / Pause",
		"  p            Play track",
		"  ← / →        Seek ±30s",
		"  + / -        Volume ±",
		"  m            Mute / Unmute",
		"  s            Cycle sink",
		"  n            Next in queue",
		"",
		tuiLabelStyle.Render("Queues & Policy:"),
		"  c            Ad policy",
		"  d            Download policy",
		"  e / o        Timeline",
		"  v / Space    Multi-select",
		"  r            Queue ad removal",
		"  x            Delete queue item",
		"  i            Cover art",
	}

	totalRows := max(len(leftLines), len(rightLines))
	for k := 0; k < totalRows; k++ {
		lL := ""
		if k < len(leftLines) {
			lL = leftLines[k]
		}
		lPad := max(0, colW-visibleRuneCount(lL))
		fullL := lL + strings.Repeat(" ", lPad)

		rL := ""
		if k < len(rightLines) {
			rL = rightLines[k]
		}
		rPad := max(0, colW-visibleRuneCount(rL))
		fullR := rL + strings.Repeat(" ", rPad)

		lines = append(lines, fullL+tuiDividerStyle.Render(" │ ")+fullR)
	}

	lines = append(lines, "")
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("Press Esc, ?, or q to close this help window"))

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

func (m *tuiModel) drawDownloadPolicyModal() string {
	if m.podIdx >= len(m.podcasts) {
		return ""
	}
	pod := m.podcasts[m.podIdx]
	boxWidth := min(70, max(42, m.width-8))

	var lines []string
	lines = append(lines, tuiTitleStyle.Render("DOWNLOAD POLICY"))
	lines = append(lines, tuiSubtitleStyle.Render(truncate("for "+displayName(pod.name), boxWidth-6)))
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, "")

	kVal := m.downloadPolicyModalK
	if kVal <= 0 {
		kVal = 3
	}

	options := []struct {
		num    int
		policy string
		name   string
		desc   string
	}{
		{1, DownloadPolicyNone, "1. No Automatic Downloads (none)", "Do not automatically download episodes"},
		{2, DownloadPolicyLatest, "2. Latest Episode Only (latest)", "Download only the single newest episode"},
		{3, DownloadPolicyLatestK, fmt.Sprintf("3. Latest K Episodes (latest_%d)", kVal), fmt.Sprintf("Keep the %d newest episodes from feed", kVal)},
		{4, DownloadPolicyMoreK, fmt.Sprintf("4. Next K Undownloaded (more_%d)", kVal), fmt.Sprintf("Download up to %d missing backlog episodes", kVal)},
		{5, DownloadPolicyAll, "5. All Episodes (all)", "Download the entire catalog backlog"},
	}

	curPolicy := normalizeDownloadPolicy(pod.config.DownloadPolicy)
	for i, opt := range options {
		isSel := (i == m.downloadPolicyModalIdx)
		isCurrent := (opt.policy == curPolicy)

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

	if m.downloadPolicyModalIdx == 2 || m.downloadPolicyModalIdx == 3 {
		adjuster := fmt.Sprintf("  Parameter K: [ < %d > ]  (Press + / - or ← / → to adjust)", kVal)
		lines = append(lines, tuiStatStyle.Render(adjuster))
		lines = append(lines, "")
	}

	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("↑/↓ Select │ 1-5 Choose │ +/- Adjust K │ Enter Apply │ Esc Cancel"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(1, 2).
		Width(boxWidth).
		Render(content)
}

func (m *tuiModel) openDownloadPolicyModal() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	cur := normalizeDownloadPolicy(pod.config.DownloadPolicy)
	switch cur {
	case DownloadPolicyLatest:
		m.downloadPolicyModalIdx = 1
	case DownloadPolicyLatestK:
		m.downloadPolicyModalIdx = 2
	case DownloadPolicyMoreK:
		m.downloadPolicyModalIdx = 3
	case DownloadPolicyAll:
		m.downloadPolicyModalIdx = 4
	default:
		m.downloadPolicyModalIdx = 0
	}
	m.downloadPolicyModalK = pod.config.DownloadK
	if m.downloadPolicyModalK <= 0 {
		m.downloadPolicyModalK = 3
	}
	m.showDownloadPolicyModal = true
}

func (m *tuiModel) applyDownloadPolicyModal() {
	if m.podIdx >= len(m.podcasts) {
		m.showDownloadPolicyModal = false
		return
	}
	pod := &m.podcasts[m.podIdx]
	policies := []string{DownloadPolicyNone, DownloadPolicyLatest, DownloadPolicyLatestK, DownloadPolicyMoreK, DownloadPolicyAll}
	if m.downloadPolicyModalIdx >= 0 && m.downloadPolicyModalIdx < len(policies) {
		pod.config.DownloadPolicy = policies[m.downloadPolicyModalIdx]
		if m.downloadPolicyModalK > 0 {
			pod.config.DownloadK = m.downloadPolicyModalK
		} else {
			pod.config.DownloadK = 3
		}
		if err := savePodcastConfig(pod.dir, pod.config); err != nil {
			m.showToast("Failed to save config: "+err.Error(), ToastError)
		} else {
			m.showToast("Saved download policy: "+downloadPolicyLabel(pod.config.DownloadPolicy, pod.config.DownloadK), ToastSuccess)
		}
	}
	m.showDownloadPolicyModal = false
}
