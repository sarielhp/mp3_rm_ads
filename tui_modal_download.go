package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		{2, DownloadPolicyLatest, "2. Latest Episode Only (latest)", "Download only the single newest episode if missing"},
		{3, DownloadPolicyLatestK, fmt.Sprintf("3. Latest K Episodes (latest_%d)", kVal), fmt.Sprintf("Keep the %d newest episodes available from feed", kVal)},
		{4, DownloadPolicyAll, "4. All Episodes (all)", "Download all missing episodes in feed"},
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

	if m.downloadPolicyModalIdx == 2 {
		adjuster := fmt.Sprintf("  Parameter K: [ < %d > ]  (Press + / - or ← / → to adjust)", kVal)
		lines = append(lines, tuiStatStyle.Render(adjuster))
		lines = append(lines, "")
	}

	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("↑/↓ Select │ 1-4 Choose │ +/- Adjust K │ Enter Apply │ Esc Cancel"))

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
	case DownloadPolicyAll:
		m.downloadPolicyModalIdx = 3
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
	policies := []string{DownloadPolicyNone, DownloadPolicyLatest, DownloadPolicyLatestK, DownloadPolicyAll}
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
