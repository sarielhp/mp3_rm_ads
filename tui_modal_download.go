package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	testSyncPolicyMu   syncMutex
	testSyncPolicyHook func(pod *tuiPodcast)
)

func getTestSyncPolicyHook() func(pod *tuiPodcast) {
	testSyncPolicyMu.Lock()
	defer testSyncPolicyMu.Unlock()
	return testSyncPolicyHook
}

func setTestSyncPolicyHook(h func(pod *tuiPodcast)) {
	testSyncPolicyMu.Lock()
	defer testSyncPolicyMu.Unlock()
	testSyncPolicyHook = h
}

func (m *tuiModel) drawDownloadPolicyModal() string {
	if m.podIdx >= len(m.podcasts) {
		return ""
	}
	pod := m.podcasts[m.podIdx]
	boxWidth := min(72, max(44, m.width-8))

	var lines []string
	lines = append(lines, tuiTitleStyle.Render("PODCAST POLICY"))
	lines = append(lines, tuiSubtitleStyle.Render(truncate("for "+displayName(pod.name), boxWidth-6)))
	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, "")

	dlVal := "[ Disabled ]"
	if m.policyAutoDownload {
		dlVal = "[ Enabled ]"
	}

	clVal := "[ Disabled ]"
	if m.policyAutoCleanup {
		clVal = "[ Enabled ]"
	}

	daysVal := "[ Keep All (∞) ]"
	if m.policyAutoCleanup && m.policyCleanupDays > 0 {
		daysVal = fmt.Sprintf("[ %d days ]", m.policyCleanupDays)
	}

	adVal := "[ None ]"
	switch m.policyAdRemoval {
	case AdRemovalAll:
		adVal = "[ All Episodes ]"
	case AdRemovalLatest:
		adVal = "[ Latest Only ]"
	}

	items := []struct {
		num   int
		title string
		val   string
		desc  string
	}{
		{1, "Auto-Download", dlVal, "Fetch new episodes automatically from feed"},
		{2, "Auto-Cleanup", clVal, "Delete older episodes past retention period"},
		{3, "Cleanup Days", daysVal, "Days to keep (Press + / - or ← / → to adjust)"},
		{4, "Ad Removal", adVal, "Commercial ad detection & removal (all, latest, none)"},
	}

	for i, it := range items {
		isSel := (i == m.downloadPolicyModalIdx)
		prefix := "( )"
		if isSel {
			prefix = "(•)"
		}
		lineTitle := fmt.Sprintf("%s %d. %-14s %s", prefix, it.num, it.title+":", it.val)
		if isSel {
			lines = append(lines, tuiSelectedStyle.Render(" "+lineTitle+" "))
		} else {
			lines = append(lines, tuiLabelStyle.Render("  "+lineTitle))
		}
		lines = append(lines, tuiDimStyle.Render("      "+it.desc))
		lines = append(lines, "")
	}

	lines = append(lines, tuiDividerStyle.Render(strings.Repeat("─", boxWidth-4)))
	lines = append(lines, tuiDimStyle.Render("↑/↓ Select │ Space/1-4 Toggle │ +/- Adjust Days │ Enter Apply │ Esc Cancel"))

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
	m.policyAutoDownload = pod.config.IsAutoDownloadEnabled()
	m.policyAutoCleanup = pod.config.IsAutoCleanupEnabled()
	m.policyCleanupDays = pod.config.AutoCleanupDays
	if m.policyAutoCleanup && m.policyCleanupDays <= 0 {
		m.policyCleanupDays = 30
	} else if !m.policyAutoCleanup {
		m.policyCleanupDays = -1
	}
	m.policyAdRemoval = normalizeAdRemovalMode(pod.config.AdRemoval)
	m.downloadPolicyModalIdx = 0
	m.downloadPolicyModalK = pod.config.DownloadK
	if m.downloadPolicyModalK <= 0 {
		m.downloadPolicyModalK = 3
	}
	m.showDownloadPolicyModal = true
}

func (m *tuiModel) togglePolicyModalField(idx int) {
	switch idx {
	case 0:
		m.policyAutoDownload = !m.policyAutoDownload
	case 1:
		m.policyAutoCleanup = !m.policyAutoCleanup
		if m.policyAutoCleanup && m.policyCleanupDays <= 0 {
			m.policyCleanupDays = 30
		} else if !m.policyAutoCleanup {
			m.policyCleanupDays = -1
		}
	case 2:
		if m.policyCleanupDays <= 0 {
			m.policyAutoCleanup = true
			m.policyCleanupDays = 30
		} else {
			m.policyAutoCleanup = false
			m.policyCleanupDays = -1
		}
	case 3:
		m.policyAdRemoval = cycleAdRemovalMode(m.policyAdRemoval)
	}
}

func (m *tuiModel) adjustPolicyModalField(delta int) {
	switch m.downloadPolicyModalIdx {
	case 0:
		m.policyAutoDownload = (delta > 0)
	case 1:
		m.policyAutoCleanup = (delta > 0)
		if m.policyAutoCleanup && m.policyCleanupDays <= 0 {
			m.policyCleanupDays = 30
		} else if !m.policyAutoCleanup {
			m.policyCleanupDays = -1
		}
	case 2:
		m.adjustCleanupDaysField(delta)
	case 3:
		m.policyAdRemoval = cycleAdRemovalMode(m.policyAdRemoval)
	}
}

func (m *tuiModel) adjustCleanupDaysField(delta int) {
	if delta > 0 {
		m.policyAutoCleanup = true
		if m.policyCleanupDays <= 0 {
			m.policyCleanupDays = 1
		} else if m.policyCleanupDays < 3650 {
			m.policyCleanupDays += delta
		}
	} else {
		if m.policyCleanupDays > 1 {
			m.policyCleanupDays += delta
		} else {
			m.policyCleanupDays = -1
			m.policyAutoCleanup = false
		}
	}
}

func (m *tuiModel) applyDownloadPolicyModal() {
	if m.podIdx >= len(m.podcasts) {
		m.showDownloadPolicyModal = false
		return
	}
	pod := &m.podcasts[m.podIdx]
	pod.config.SetAutoDownload(m.policyAutoDownload)
	pod.config.SetAutoCleanup(m.policyAutoCleanup)
	if m.policyAutoCleanup && m.policyCleanupDays > 0 {
		pod.config.AutoCleanupDays = m.policyCleanupDays
	} else {
		pod.config.AutoCleanupDays = -1
	}
	pod.config.AdRemoval = m.policyAdRemoval
	if m.downloadPolicyModalK > 0 {
		pod.config.DownloadK = m.downloadPolicyModalK
	}

	if err := savePodcastConfig(pod.dir, pod.config); err != nil {
		m.showToast("Failed to save config: "+err.Error(), ToastError)
	} else {
		m.showToast("Policy saved: DL="+boolStatus(m.policyAutoDownload)+", Cleanup="+boolStatus(m.policyAutoCleanup)+", Ads="+m.policyAdRemoval, ToastSuccess)
	}

	autoDownload := m.policyAutoDownload
	autoCleanup := m.policyAutoCleanup
	cleanupDays := pod.config.AutoCleanupDays
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic syncing policy to backend: %v\n%s", r, debug.Stack())
			}
		}()
		syncPolicyToBackend(pod, autoDownload, autoCleanup, cleanupDays)
	}()
	m.showDownloadPolicyModal = false
}

func syncPolicyToBackend(pod *tuiPodcast, autoDownload, autoCleanup bool, autoCleanupDays int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in syncPolicyToBackend: %v\n%s", r, debug.Stack())
		}
	}()
	if hook := getTestSyncPolicyHook(); hook != nil {
		hook(pod)
		return
	}
	cfg := loadConfig()
	b, err := getBackend(cfg, true)
	if err != nil || b == nil {
		return
	}
	id := pod.config.ID
	if id == "" && pod.absData != nil {
		id = pod.absData.ID
	}
	if id == "" {
		id = filepath.Base(pod.dir)
	}
	_ = b.UpdatePodcastSettings(id, autoDownload, autoCleanup, autoCleanupDays)
}

func boolStatus(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
