package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUISearchInteractivePodcasts(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcasts

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searchMode {
		t.Fatal("expected searchMode to be true after '/'")
	}

	for _, r := range "News" {
		m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.searchQuery != "News" {
		t.Fatalf("expected query 'News', got %q", m.searchQuery)
	}

	filtered := m.filteredPodcasts()
	if len(filtered) != 1 || filtered[0].name != "News Daily" {
		t.Fatalf("expected 1 filtered podcast 'News Daily', got %v", filtered)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searchMode {
		t.Error("expected searchMode to be false after enter")
	}
	if m.screen != screenPodcastDetail {
		t.Fatalf("expected screenPodcastDetail, got %v", m.screen)
	}
	if m.podcasts[m.podIdx].name != "News Daily" {
		t.Fatalf("expected selected podcast 'News Daily', got %s", m.podcasts[m.podIdx].name)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.screen != screenPodcasts {
		t.Fatalf("expected return to screenPodcasts, got %v", m.screen)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "xyz" {
		m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchMode || m.searchQuery != "" {
		t.Errorf("expected search cancelled on escape, got mode=%v query=%q", m.searchMode, m.searchQuery)
	}
}

func TestTUISearchInteractiveEpisodes(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail
	m.podIdx = 0

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searchMode {
		t.Fatal("expected searchMode to be true")
	}

	for _, r := range "103" {
		m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.searchQuery != "103" {
		t.Fatalf("expected query '103', got %q", m.searchQuery)
	}

	eps := m.filteredEpisodes()
	if len(eps) != 1 || eps[0].filename != "ep103.mp3" {
		t.Fatalf("expected 1 episode 'ep103.mp3', got %v", eps)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searchMode {
		t.Error("expected searchMode to exit on enter")
	}
	if m.screen != screenEpisodeDetail {
		t.Fatalf("expected screenEpisodeDetail, got %v", m.screen)
	}
	if m.podcasts[0].episodes[m.epIdx].filename != "ep103.mp3" {
		t.Fatalf("expected ep103.mp3, got %s", m.podcasts[0].episodes[m.epIdx].filename)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.screen != screenPodcastDetail {
		t.Fatalf("expected screenPodcastDetail, got %v", m.screen)
	}
}

func TestTUISearchTranscriptViewer(t *testing.T) {
	m := makeTestModel()
	m.screen = screenTranscript
	m.transcriptLines = []string{
		"[00:00.0 -> 00:05.0] Intro to podcast",
		"[00:05.0 -> 00:10.0] Talking about Go programming",
		"[00:10.0 -> 00:15.0] Sponsor message from CloudCorp",
		"[00:15.0 -> 00:20.0] More on Go programming",
		"[00:20.0 -> 00:25.0] Another note on CloudCorp sponsor",
		"[00:25.0 -> 00:30.0] Outro",
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searchMode {
		t.Fatal("expected searchMode to be true in transcript viewer")
	}

	for _, r := range "sponsor" {
		m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.searchQuery != "sponsor" {
		t.Fatalf("expected query 'sponsor', got %q", m.searchQuery)
	}

	matches := m.matchingTranscriptIndices()
	if len(matches) != 2 || matches[0] != 2 || matches[1] != 4 {
		t.Fatalf("expected matches [2, 4], got %v", matches)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searchMode {
		t.Error("expected searchMode false after enter")
	}
	if m.transcriptMatchIdx != 1 {
		t.Errorf("expected match index 1 after enter, got %d", m.transcriptMatchIdx)
	}

	m.handleTranscriptKey("n")
	if m.transcriptMatchIdx != 0 {
		t.Errorf("expected match index 0 after wrapping next, got %d", m.transcriptMatchIdx)
	}

	m.handleTranscriptKey("N")
	if m.transcriptMatchIdx != 1 {
		t.Errorf("expected match index 1 after prev, got %d", m.transcriptMatchIdx)
	}

	view := m.drawTranscriptScreen()
	if !strings.Contains(view, "TRANSCRIPT") {
		t.Error("expected transcript header in view")
	}
	if !strings.Contains(view, "[2/2 matches]") {
		t.Errorf("expected match indicator in view, got: %s", view)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchQuery != "" {
		t.Errorf("expected cleared search query on escape, got %q", m.searchQuery)
	}
}

func TestTUITranscriptExport(t *testing.T) {
	tempDir := t.TempDir()
	epPath := filepath.Join(tempDir, "episode.mp3")
	jsonPath := filepath.Join(tempDir, "episode.transcript.json")

	td := TranscriptionData{
		Language: "en",
		Segments: []TranscriptionSegment{
			{Start: 0.0, End: 5.0, Text: "Hello and welcome"},
			{Start: 5.0, End: 10.0, Text: "This is a test transcript"},
		},
	}
	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		t.Fatalf("write json error: %v", err)
	}

	m := makeTestModel()
	m.screen = screenTranscript
	m.transcriptLoadedFor = epPath
	m.transcriptLines = []string{
		"[00:00.0 -> 00:05.0] Hello and welcome",
		"[00:05.0 -> 00:10.0] This is a test transcript",
	}

	m.handleTranscriptKey("s")
	if m.toast == nil || m.toast.Type != ToastSuccess {
		t.Errorf("expected success toast on 's' export, got %+v", m.toast)
	}

	srtPath := filepath.Join(tempDir, "episode.srt")
	txtPath := filepath.Join(tempDir, "episode.transcript.txt")
	if _, err := os.Stat(srtPath); err != nil {
		t.Errorf("expected srt file to exist: %v", err)
	}
	if _, err := os.Stat(txtPath); err != nil {
		t.Errorf("expected txt file to exist: %v", err)
	}

	_ = os.Remove(srtPath)
	_ = os.Remove(txtPath)
	m.handleTranscriptKey("e")
	if m.toast == nil || m.toast.Type != ToastSuccess {
		t.Errorf("expected success toast on 'e' export, got %+v", m.toast)
	}
	if _, err := os.Stat(srtPath); err != nil {
		t.Errorf("expected srt file to exist after 'e': %v", err)
	}

	noJsonDir := filepath.Join(tempDir, "nojson")
	_ = os.MkdirAll(noJsonDir, 0755)
	m.transcriptLoadedFor = filepath.Join(noJsonDir, "custom.mp3")
	m.exportTranscript()

	customSrt := filepath.Join(noJsonDir, "custom.srt")
	customTxt := filepath.Join(noJsonDir, "custom.transcript.txt")
	if _, err := os.Stat(customSrt); err != nil {
		t.Errorf("expected custom srt file to exist: %v", err)
	}
	if _, err := os.Stat(customTxt); err != nil {
		t.Errorf("expected custom txt file to exist: %v", err)
	}
}

func TestTUITranscriptExportError(t *testing.T) {
	m := makeTestModel()
	m.screen = screenTranscript
	m.transcriptLoadedFor = ""
	m.podIdx = 999
	m.transcriptLines = nil

	m.exportTranscript()
	if m.toast == nil || m.toast.Type != ToastError {
		t.Errorf("expected error toast on invalid export, got %+v", m.toast)
	}
}
