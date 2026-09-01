package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUISearchFilter(t *testing.T) {
	m := makeTestModel()
	m.searchQuery = "tech"
	pods := m.filteredPodcasts()
	if len(pods) != 1 || pods[0].name != "Tech Podcast" {
		t.Errorf("expected 1 podcast (Tech Podcast), got %d: %v", len(pods), pods)
	}

	m.searchQuery = "nonexistent"
	pods = m.filteredPodcasts()
	if len(pods) != 0 {
		t.Errorf("expected 0 podcasts, got %d", len(pods))
	}

	m.searchQuery = ""
	pods = m.filteredPodcasts()
	if len(pods) != 2 {
		t.Errorf("expected 2 podcasts with empty query, got %d", len(pods))
	}
}

func TestTUISearchFilterEpisodes(t *testing.T) {
	m := makeTestModel()
	m.searchQuery = "ep101"
	eps := m.filteredEpisodes()
	if len(eps) != 1 || eps[0].filename != "ep101.mp3" {
		t.Errorf("expected 1 episode (ep101.mp3), got %d", len(eps))
	}

	m.searchQuery = ""
	eps = m.filteredEpisodes()
	if len(eps) != 3 {
		t.Errorf("expected 3 episodes with empty query, got %d", len(eps))
	}
}

func TestTUISearchModeKeyHandling(t *testing.T) {
	m := makeTestModel()

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searchMode {
		t.Error("should be in search mode after /")
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m.searchQuery != "tes" {
		t.Errorf("search query should be 'tes', got %q", m.searchQuery)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchQuery != "te" {
		t.Errorf("search query should be 'te' after backspace, got %q", m.searchQuery)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchMode {
		t.Error("should exit search mode on escape")
	}
	if m.searchQuery != "" {
		t.Errorf("search query should be empty after escape, got %q", m.searchQuery)
	}
}

func TestTUISearchModeEnter(t *testing.T) {
	m := makeTestModel()
	m.searchMode = true
	m.searchQuery = "test"

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searchMode {
		t.Error("should exit search mode on enter")
	}
}

func TestTUISortToggle(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail

	if m.podcasts[0].episodes[0].filename != "ep101.mp3" {
		t.Errorf("expected ep101 first, got %s", m.podcasts[0].episodes[0].filename)
	}

	m.handleSortToggle()

	if m.podcasts[0].episodes[0].filename != "ep103.mp3" {
		t.Errorf("expected ep103 first after sort toggle, got %s", m.podcasts[0].episodes[0].filename)
	}

	m.handleSortToggle()
	if m.podcasts[0].episodes[0].filename != "ep101.mp3" {
		t.Errorf("expected ep101 first after second toggle, got %s", m.podcasts[0].episodes[0].filename)
	}
}

func TestTUISortToggleOnPodcastScreen(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcasts
	original := m.podcasts[0].episodes[0].filename
	m.handleSortToggle()
	if m.podcasts[0].episodes[0].filename != original {
		t.Error("sort toggle should not work on podcast list screen")
	}
}

func TestTUIHelpToggle(t *testing.T) {
	m := makeTestModel()

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if !m.showHelp {
		t.Error("showHelp should be true after B key")
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
	if m.showHelp {
		t.Error("showHelp should be false after second B key")
	}
}

func TestTUISearchBarDisplay(t *testing.T) {
	m := makeTestModel()
	m.searchMode = false
	bar := m.searchBar()
	if bar != "" {
		t.Errorf("search bar should be empty when not in search mode, got %q", bar)
	}

	m.searchMode = true
	m.searchQuery = "test"
	bar = m.searchBar()
	if !strings.Contains(bar, "test") {
		t.Errorf("search bar should contain query, got %q", bar)
	}
}

func TestTUIFilteredPodcastsOutOfRange(t *testing.T) {
	m := makeTestModel()
	m.podIdx = 999
	eps := m.filteredEpisodes()
	if eps != nil {
		t.Error("filteredEpisodes should return nil for out-of-range podIdx")
	}
}

func TestTUIRelativeAge(t *testing.T) {
	now := time.Now()

	cases := []struct {
		t   time.Time
		out string
	}{
		{now, "Today"},
		{now.Add(-25 * time.Hour), "1Day"},
		{now.Add(-72 * time.Hour), "3Days"},
		{now.Add(-10 * 24 * time.Hour), "1Week"},
		{now.Add(-60 * 24 * time.Hour), "2Months"},
		{now.Add(-400 * 24 * time.Hour), "1Year"},
		{now.Add(24 * time.Hour), "Future"},
	}

	for _, c := range cases {
		got := formatRelativeAge(c.t)
		if got != c.out {
			t.Errorf("formatRelativeAge(%v) = %q, want %q", c.t, got, c.out)
		}
	}
}

func TestTUIRenderHTML(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"Hello World", "Hello World"},
		{"<b>bold</b>", "\x1b[1mbold\x1b[22m"},
		{"<i>italic</i>", "\x1b[3mitalic\x1b[23m"},
		{"Line1<br>Line2", "Line1\nLine2"},
		{"<p>Para1</p><p>Para2</p>", "\nPara1\n\nPara2\n"},
		{"<li>Item1</li>", "\n  - Item1\n"},
		{"Hello &amp; World", "Hello & World"},
		{"a &lt; b &gt; c", "a < b > c"},
		{"&quot;quoted&quot;", "\"quoted\""},
		{"&nbsp;space", " space"},
	}

	for _, c := range cases {
		got := renderHTML(c.in)
		if got != c.out {
			t.Errorf("renderHTML(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestTUIPopupMessage(t *testing.T) {
	m := makeTestModel()

	m.showPopup("Test message")
	if m.popupMsg != "Test message" {
		t.Errorf("popupMsg should be 'Test message', got %q", m.popupMsg)
	}
	if m.popupTimer != 10 {
		t.Errorf("popupTimer should be 10, got %d", m.popupTimer)
	}

	for i := 0; i < 10; i++ {
		m.tickPopup()
	}
	if m.popupMsg != "" {
		t.Error("popupMsg should be empty after 10 ticks")
	}
}

func TestTUIPopupDraw(t *testing.T) {
	m := makeTestModel()
	m.popupMsg = ""
	popup := m.drawPopup()
	if popup != "" {
		t.Errorf("empty popup should return empty string, got %q", popup)
	}

	m.popupMsg = "Saving..."
	popup = m.drawPopup()
	if !strings.Contains(popup, "Saving...") {
		t.Errorf("popup should contain message, got %q", popup)
	}
}

func TestTUIFileSizeGB(t *testing.T) {
	cases := []struct {
		in  int64
		out string
	}{
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
		{2147483648, "2.0 GB"},
	}
	for _, c := range cases {
		got := formatFileSize(c.in)
		if got != c.out {
			t.Errorf("formatFileSize(%d) = %q, want %q", c.in, got, c.out)
		}
	}
}
