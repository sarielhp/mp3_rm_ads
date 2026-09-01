package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIViewPodcasts(t *testing.T) {
	m := makeTestModel()
	m.ready = true
	m.screen = screenPodcasts

	view := m.View()

	if !strings.Contains(view, " PODCASTS ") {
		t.Error("should contain ' PODCASTS ' heading")
	}
	if !strings.Contains(view, "Tech Podcast") {
		t.Error("should contain podcast name")
	}
	if !strings.Contains(view, "News Daily") {
		t.Error("should contain podcast name")
	}
	if !strings.Contains(view, "2 podcasts") {
		t.Error("should show podcast count")
	}
	if !strings.Contains(view, "4 episodes") {
		t.Error("should show total episode count")
	}
	if !strings.Contains(view, "2 ad-free") {
		t.Error("should show ad-free count")
	}
}

func TestTUIViewLoading(t *testing.T) {
	m := makeTestModel()
	m.loading = true
	m.ready = false
	view := m.View()
	if view != "Loading podcasts..." {
		t.Errorf("expected loading, got %q", view)
	}
}

func TestTUIViewError(t *testing.T) {
	m := makeTestModel()
	m.loadErr = "something went wrong"
	view := m.View()
	if !strings.Contains(view, "something went wrong") {
		t.Errorf("expected error message in view, got %q", view)
	}
}

func TestTUIViewPodcastDetail(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail

	view := m.View()

	if !strings.Contains(view, "Tech Podcast") {
		t.Error("should show podcast name")
	}
	if !strings.Contains(view, "3 episodes") {
		t.Error("should show episode count")
	}
	if !strings.Contains(view, "2 ad-free") {
		t.Error("should show ad-free count")
	}
	if !strings.Contains(view, "1 queued") {
		t.Error("should show queued count")
	}
	if !strings.Contains(view, "ep102.mp3") {
		t.Error("should show episode filename")
	}
}

func TestTUIViewPodcastDetailCheckmark(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail

	view := m.View()

	if strings.Count(view, "✓") != 2 {
		t.Errorf("expected 2 checkmarks for ad-free episodes, found %d", strings.Count(view, "✓"))
	}
}

func TestTUIViewPodcastDetailQ(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail

	view := m.View()

	if !strings.Contains(view, "[Q]") {
		t.Error("queued episode should show [Q] indicator")
	}
}

func TestTUIViewEpisodeDetail(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 1
	m.screen = screenEpisodeDetail

	view := m.View()

	if !strings.Contains(view, "ep102.mp3") {
		t.Error("should show episode filename")
	}
	if !strings.Contains(view, "Show Notes") {
		t.Error("should show Show Notes")
	}
	if !strings.Contains(view, "Has Ads") {
		t.Error("should show Has Ads status")
	}
	if !strings.Contains(view, "F4 Show Player") {
		t.Error("should show F4 Show Player legend by default")
	}

	m.showEpisodePlayerPane = true
	viewWithPlayer := m.View()
	if !strings.Contains(viewWithPlayer, "AUDIO PLAYER") {
		t.Error("should show AUDIO PLAYER when player pane is enabled")
	}
	if !strings.Contains(viewWithPlayer, "F4 Hide Player") {
		t.Error("should show F4 Hide Player legend when player pane is enabled")
	}
}

func TestTUIViewEpisodeDetailAdRemoved(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 0
	m.screen = screenEpisodeDetail

	view := m.View()

	if !strings.Contains(view, "Ad-Free") {
		t.Error("should show Ad-Free status")
	}
	if !strings.Contains(view, "✓") {
		t.Error("should show checkmark for ad-free")
	}
}

func TestTUIViewEpisodeDetailNotQueued(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 0
	m.queue["/tmp/pods/tech"] = nil
	m.screen = screenEpisodeDetail
	m.showEpisodePlayerPane = true

	view := m.View()

	if !strings.Contains(view, "AUDIO PLAYER") {
		t.Error("should show player section")
	}
}

func TestTUIViewEpisodeDetailFallback(t *testing.T) {
	m := makeTestModel()
	m.podcasts = append(m.podcasts, tuiPodcast{name: "Empty", dir: "/tmp/empty"})
	m.podIdx = 2
	m.epIdx = 5
	m.screen = screenEpisodeDetail

	view := m.View()

	if m.screen != screenPodcastDetail {
		t.Error("should fall back to podcast detail when epIdx out of range")
	}
	if !strings.Contains(view, "Empty") {
		t.Error("should show empty podcast name after fallback")
	}
}

func TestTUIViewEpisodeDetailDuration(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 0
	m.screen = screenEpisodeDetail

	view := m.View()

	if !strings.Contains(view, "30:00") {
		t.Errorf("should show duration 30:00, got %s", view)
	}
}

func TestTUIScrollIndicator(t *testing.T) {
	m := makeTestModel()
	m.podcasts = make([]tuiPodcast, 20)
	for i := range m.podcasts {
		m.podcasts[i] = tuiPodcast{
			name:     string(rune('A' + i%26)),
			episodes: []tuiEpisode{{filename: "ep.mp3"}},
		}
	}

	view := m.View()

	if !strings.Contains(view, "%") {
		t.Error("scroll indicator should show percentage")
	}
}

func TestTUIWindowSize(t *testing.T) {
	m := makeTestModel()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
	if !m.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
}

func TestTUIHandleKeyUp(t *testing.T) {
	m := makeTestModel()
	m.podIdx = 0

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
	if m.podIdx != 0 {
		t.Errorf("up at top should keep podIdx=0, got %d", m.podIdx)
	}

	sendKey(m, "up")
	if m.podIdx != 0 {
		t.Errorf("k at top should keep podIdx=0, got %d", m.podIdx)
	}
}

func TestTUIHandleKeyDown(t *testing.T) {
	m := makeTestModel()
	m.podIdx = 0

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("down")})
	if m.podIdx != 1 {
		t.Errorf("down should advance to 1, got %d", m.podIdx)
	}

	sendKey(m, "j")
	if m.podIdx != 1 {
		t.Errorf("j at bottom should stay at 1, got %d", m.podIdx)
	}
}

func TestTUIHandleKeyEnter(t *testing.T) {
	m := makeTestModel()
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenPodcastDetail {
		t.Errorf("enter should go to detail, got %d", m.screen)
	}
}

func TestTUIHandleKeyEscape(t *testing.T) {
	m := makeTestModel()
	m.screen = screenEpisodeDetail
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.screen != screenPodcastDetail {
		t.Errorf("esc should go back, got %d", m.screen)
	}
}

func TestTUIHandleKeyR(t *testing.T) {
	d := t.TempDir()
	m := makeTestModel()
	m.podcasts[0].dir = d
	m.queue[d] = []string{}
	m.screen = screenPodcastDetail

	saveCalled := false
	m.bk.SaveQueue = func(dir string, entries []string) {
		saveCalled = true
	}

	sendKey(m, "r")
	if len(m.queue[d]) != 1 || m.queue[d][0] != "ep102.mp3" {
		t.Errorf("r should add ep102.mp3, got %v", m.queue[d])
	}
	if !saveCalled {
		t.Error("SaveQueue should have been called")
	}
}

func TestTUIHandleKeyCtrlC(t *testing.T) {
	m := makeTestModel()
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.done {
		t.Error("ctrl+c should set done")
	}
}

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
