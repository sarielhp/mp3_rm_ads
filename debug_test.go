package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDebugLoggerAndSnapshots(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempDir)
	t.Setenv("ABS_DEBUG", "1")

	initDebugLogger(true)
	debugLog("Test log message: %s", "hello_debug")

	logFile := filepath.Join(tempDir, "abs", "debug", "abs_debug.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("expected debug log file to exist: %v", err)
	}
	if !strings.Contains(string(data), "hello_debug") {
		t.Errorf("expected debug log to contain message, got: %s", string(data))
	}

	m := makeTestModel()
	snapPath := m.takeSnapshot()
	if snapPath == "" {
		t.Fatalf("expected snapshot path to be non-empty")
	}
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("expected snapshot file to exist: %v", err)
	}

	snapContent, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("failed to read snapshot file: %v", err)
	}
	if !strings.Contains(string(snapContent), "=== Snapshot #") {
		t.Errorf("expected snapshot header, got: %s", string(snapContent))
	}
	if !strings.Contains(string(snapContent), "Screen: screenPodcasts") {
		t.Errorf("expected screen name in snapshot, got: %s", string(snapContent))
	}

	// Test F12 key press triggers snapshot in handleKey
	m2 := makeTestModel()
	_, _ = m2.handleKey(tea.KeyMsg{Type: tea.KeyF12})
	if m2.toast == nil && m2.popupMsg == "" {
		t.Errorf("expected popup or toast confirmation for snapshot")
	}
}

func TestQAndEscapeParity(t *testing.T) {
	// Test on root screen (screenPodcasts): both q and esc quit
	m1 := makeTestModel()
	m1.screen = screenPodcasts
	_, cmd1 := m1.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd1 == nil || !m1.done {
		t.Errorf("expected q on root screen to quit")
	}

	m2 := makeTestModel()
	m2.screen = screenPodcasts
	_, cmd2 := m2.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd2 == nil || !m2.done {
		t.Errorf("expected esc on root screen to quit")
	}

	// Test on sub-screen (screenPodcastDetail): both q and esc return to screenPodcasts
	m3 := makeTestModel()
	m3.screen = screenPodcastDetail
	_, _ = m3.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m3.screen != screenPodcasts {
		t.Errorf("expected q on screenPodcastDetail to return to screenPodcasts, got %v", m3.screen)
	}

	m4 := makeTestModel()
	m4.screen = screenPodcastDetail
	_, _ = m4.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m4.screen != screenPodcasts {
		t.Errorf("expected esc on screenPodcastDetail to return to screenPodcasts, got %v", m4.screen)
	}

	// Test on modal: both q and esc close help modal
	m5 := makeTestModel()
	m5.showHelpModal = true
	_, _ = m5.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m5.showHelpModal {
		t.Errorf("expected q to close help modal")
	}

	m6 := makeTestModel()
	m6.showHelpModal = true
	_, _ = m6.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m6.showHelpModal {
		t.Errorf("expected esc to close help modal")
	}
}
