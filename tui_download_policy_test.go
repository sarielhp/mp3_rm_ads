package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testDownloadPolicyIndexNavigation(t *testing.T, m *tuiModel) {
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.downloadPolicyModalIdx != 1 {
		t.Errorf("expected idx 1 after Down, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.downloadPolicyModalIdx != 2 {
		t.Errorf("expected idx 2 after j, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.downloadPolicyModalIdx != 3 {
		t.Errorf("expected idx 3 after Down, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.downloadPolicyModalIdx != 3 {
		t.Errorf("expected idx to remain 3 at bottom, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.downloadPolicyModalIdx != 2 {
		t.Errorf("expected idx 2 after Up, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.downloadPolicyModalIdx != 1 {
		t.Errorf("expected idx 1 after k, got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if m.downloadPolicyModalIdx != 3 {
		t.Errorf("expected idx 3 after pressing '4', got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.downloadPolicyModalIdx != 0 {
		t.Errorf("expected idx 0 after pressing '1', got %d", m.downloadPolicyModalIdx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if m.downloadPolicyModalIdx != 2 {
		t.Errorf("expected idx 2 after pressing '3', got %d", m.downloadPolicyModalIdx)
	}
}

func testDownloadPolicyDaysAdjustment(t *testing.T, m *tuiModel) {
	m.downloadPolicyModalIdx = 2
	m.policyAutoCleanup = true
	m.policyCleanupDays = 30
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	if m.policyCleanupDays != 31 {
		t.Errorf("expected days 31 after '=', got %d", m.policyCleanupDays)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'_'}})
	if m.policyCleanupDays != 30 {
		t.Errorf("expected days 30 after '_', got %d", m.policyCleanupDays)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.policyCleanupDays != 31 {
		t.Errorf("expected days 31 after 'l', got %d", m.policyCleanupDays)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.policyCleanupDays != 30 {
		t.Errorf("expected days 30 after 'h', got %d", m.policyCleanupDays)
	}
}

func TestTUIDownloadPolicyModalNavigationAndKeys(t *testing.T) {
	tempDir := t.TempDir()
	m := makeTestModel()
	m.podcasts[0].dir = tempDir
	m.podIdx = 0
	m.width = 80
	m.screen = screenPodcasts

	m.openDownloadPolicyModal()
	if !m.showDownloadPolicyModal || m.downloadPolicyModalIdx != 0 {
		t.Fatalf("expected modal open with idx 0, got show=%v idx=%d", m.showDownloadPolicyModal, m.downloadPolicyModalIdx)
	}

	testDownloadPolicyIndexNavigation(t, m)
	testDownloadPolicyDaysAdjustment(t, m)

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.showDownloadPolicyModal {
		t.Errorf("expected modal to close after Enter")
	}
	if !m.podcasts[0].config.IsAutoCleanupEnabled() || m.podcasts[0].config.AutoCleanupDays != 30 {
		t.Errorf("expected auto cleanup enabled 30 days, got %+v", m.podcasts[0].config)
	}

	saved := loadPodcastConfig(tempDir)
	if !saved.IsAutoCleanupEnabled() || saved.AutoCleanupDays != 30 {
		t.Errorf("expected saved config to match cleanup 30 days, got %+v", saved)
	}
}

func TestTUIDownloadPolicyModalRenderingAndCancel(t *testing.T) {
	tempDir := t.TempDir()
	m := makeTestModel()
	m.podcasts[0].dir = tempDir
	m.podcasts[0].config.DownloadPolicy = DownloadPolicyAll
	m.podIdx = 0
	m.width = 80

	m.openDownloadPolicyModal()
	rendered := m.drawDownloadPolicyModal()
	if !strings.Contains(rendered, "PODCAST POLICY") {
		t.Errorf("expected title in modal, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "1. Auto-Download") || !strings.Contains(rendered, "4. Ad Removal") {
		t.Errorf("expected options in modal, got:\n%s", rendered)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.showDownloadPolicyModal {
		t.Errorf("expected modal to close on 'q'")
	}
	if m.podcasts[0].config.DownloadPolicy != DownloadPolicyAll {
		t.Errorf("expected unchanged policy after cancel, got %s", m.podcasts[0].config.DownloadPolicy)
	}
}

func TestTUIDownloadPolicyKeyPress(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcasts
	m.podIdx = 0
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m.showDownloadPolicyModal {
		t.Errorf("expected pressing 'd' to open download policy modal")
	}
}

func TestSyncPolicyToBackend_PanicRecovery(t *testing.T) {
	tempDir := t.TempDir()
	m := makeTestModel()
	m.podcasts[0].dir = tempDir
	m.podIdx = 0

	ch := make(chan struct{}, 1)
	setTestSyncPolicyHook(func(pod *tuiPodcast) {
		select {
		case ch <- struct{}{}:
		default:
		}
		panic("simulated backend panic during policy sync")
	})
	defer setTestSyncPolicyHook(nil)

	m.openDownloadPolicyModal()
	m.applyDownloadPolicyModal()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected testSyncPolicyHook to run")
	}

	time.Sleep(10 * time.Millisecond)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("syncPolicyToBackend failed to catch internal panic: %v", r)
		}
	}()
	syncPolicyToBackend(&m.podcasts[0], true, false, 0)
}
