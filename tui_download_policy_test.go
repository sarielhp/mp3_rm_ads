package main

import (
	"strings"
	"testing"

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

func testDownloadPolicyKAdjustment(t *testing.T, m *tuiModel) {
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'='}})
	if m.downloadPolicyModalK != 4 {
		t.Errorf("expected K 4 after '=', got %d", m.downloadPolicyModalK)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'_'}})
	if m.downloadPolicyModalK != 3 {
		t.Errorf("expected K 3 after '_', got %d", m.downloadPolicyModalK)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.downloadPolicyModalK != 4 {
		t.Errorf("expected K 4 after 'l', got %d", m.downloadPolicyModalK)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.downloadPolicyModalK != 3 {
		t.Errorf("expected K 3 after 'h', got %d", m.downloadPolicyModalK)
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
	testDownloadPolicyKAdjustment(t, m)

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.showDownloadPolicyModal {
		t.Errorf("expected modal to close after Enter")
	}
	if m.podcasts[0].config.DownloadPolicy != DownloadPolicyLatestK || m.podcasts[0].config.DownloadK != 3 {
		t.Errorf("expected policy latest_k 3, got %s %d", m.podcasts[0].config.DownloadPolicy, m.podcasts[0].config.DownloadK)
	}

	saved := loadPodcastConfig(tempDir)
	if saved.DownloadPolicy != DownloadPolicyLatestK || saved.DownloadK != 3 {
		t.Errorf("expected saved config to match latest_k 3, got %+v", saved)
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
	if m.downloadPolicyModalIdx != 3 {
		t.Errorf("expected initial idx 3 for all, got %d", m.downloadPolicyModalIdx)
	}

	rendered := m.drawDownloadPolicyModal()
	if !strings.Contains(rendered, "DOWNLOAD POLICY") {
		t.Errorf("expected title in modal, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "1. No Automatic Downloads") || !strings.Contains(rendered, "4. All Episodes") {
		t.Errorf("expected 4 options in modal, got:\n%s", rendered)
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
