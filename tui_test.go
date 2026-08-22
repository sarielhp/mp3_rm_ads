package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUITruncate(t *testing.T) {
	cases := []struct {
		in  string
		max int
		out string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "he..."},
		{"hello world", 4, "h..."},
		{"hello world", 3, "hel"},
		{"hello world", 2, "he"},
		{"hello world", 1, "h"},
		{"hello world", 0, ""},
		{"", 10, ""},
		{"", 0, ""},
		{"a", 1, "a"},
		{"ab", 2, "ab"},
	}
	for _, c := range cases {
		got := truncate(c.in, c.max)
		if got != c.out {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.max, got, c.out)
		}
	}
}

func TestTUIFileSize(t *testing.T) {
	cases := []struct {
		in  int64
		out string
	}{
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{0, "0 B"},
		{1, "1 B"},
	}
	for _, c := range cases {
		got := formatFileSize(c.in)
		if got != c.out {
			t.Errorf("formatFileSize(%d) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestTUIFormatDurationShort(t *testing.T) {
	cases := []struct {
		in  float64
		out string
	}{
		{0, "0:00"},
		{30, "0:30"},
		{60, "1:00"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{7322, "2:02:02"},
		{59, "0:59"},
		{3599, "59:59"},
	}
	for _, c := range cases {
		got := formatDurationShort(c.in)
		if got != c.out {
			t.Errorf("formatDurationShort(%f) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestTUILoadPodcasts(t *testing.T) {
	d := t.TempDir()

	os.MkdirAll(d+"/pod1", 0755)
	os.MkdirAll(d+"/pod2", 0755)
	os.MkdirAll(d+"/.work", 0755)
	os.MkdirAll(d+"/.hidden", 0755)

	os.WriteFile(d+"/not_a_dir.mp3", []byte("x"), 0644)

	os.WriteFile(d+"/pod1/ep1.mp3", []byte("aaa"), 0644)
	os.WriteFile(d+"/pod1/ep2.mp3", []byte("bbb"), 0644)
	os.WriteFile(d+"/pod2/epA.mp3", []byte("ccc"), 0644)
	os.WriteFile(d+"/pod2/epB.mp3", []byte("ddd"), 0644)

	os.WriteFile(d+"/pod2/epA.cuts.json", []byte(`{}`), 0644)

	pods, err := loadTUIPodcasts(d)
	if err != nil {
		t.Fatalf("loadTUIPodcasts error: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 podcasts, got %d", len(pods))
	}

	if pods[0].name != "pod1" || pods[1].name != "pod2" {
		t.Errorf("expected pod1, pod2 sorted, got %q, %q", pods[0].name, pods[1].name)
	}

	if len(pods[0].episodes) != 2 {
		t.Errorf("pod1 expected 2 episodes, got %d", len(pods[0].episodes))
	}
	if len(pods[1].episodes) != 2 {
		t.Errorf("pod2 expected 2 episodes, got %d", len(pods[1].episodes))
	}

	if pods[0].episodes[0].filename != "ep1.mp3" {
		t.Errorf("expected ep1.mp3 first, got %q", pods[0].episodes[0].filename)
	}
	if !pods[1].episodes[0].hasAdsRemoved {
		t.Error("epA should have ads removed (cuts.json present)")
	}
	if pods[1].episodes[1].hasAdsRemoved {
		t.Error("epB should not have ads removed")
	}
}

func TestTUILoadPodcastsNonExistentDir(t *testing.T) {
	_, err := loadTUIPodcasts("/nonexistent/path/xyz123")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestTUILoadAllQueues(t *testing.T) {
	d := t.TempDir()

	os.MkdirAll(d+"/pod1", 0755)
	os.MkdirAll(d+"/pod2", 0755)
	os.MkdirAll(d+"/pod3", 0755)

	os.WriteFile(d+"/pod1/queue.json", []byte(`["ep1.mp3", "ep2.mp3"]`), 0644)
	os.WriteFile(d+"/pod2/queue.json", []byte(`["ep3.mp3", "note.txt"]`), 0644)

	pods := []tuiPodcast{
		{name: "pod1", dir: d + "/pod1"},
		{name: "pod2", dir: d + "/pod2"},
		{name: "pod3", dir: d + "/pod3"},
	}

	q := loadAllQueues(pods)

	if len(q[pods[0].dir]) != 2 {
		t.Errorf("pod1 expected 2 queue entries, got %d", len(q[pods[0].dir]))
	}
	if len(q[pods[1].dir]) != 1 {
		t.Errorf("pod2 expected 1 queue entry (note.txt filtered), got %d", len(q[pods[1].dir]))
	}
	if q[pods[2].dir] != nil {
		t.Errorf("pod3 expected nil queue (no queue.json), got %v", q[pods[2].dir])
	}

	if q[pods[0].dir][0] != "ep1.mp3" || q[pods[0].dir][1] != "ep2.mp3" {
		t.Errorf("pod1 queue contents wrong: %v", q[pods[0].dir])
	}
}

func TestTUISaveQueue(t *testing.T) {
	d := t.TempDir()

	saveQueue(d, []string{"ep1.mp3", "ep2.mp3"})

	data, err := os.ReadFile(d + "/queue.json")
	if err != nil {
		t.Fatalf("queue.json not written: %v", err)
	}

	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(entries) != 2 || entries[0] != "ep1.mp3" || entries[1] != "ep2.mp3" {
		t.Errorf("unexpected queue contents: %v", entries)
	}
}

func TestTUISaveQueueEmpty(t *testing.T) {
	d := t.TempDir()
	saveQueue(d, []string{})
	data, _ := os.ReadFile(d + "/queue.json")
	var entries []string
	json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue, got %v", entries)
	}
}

func TestTUISaveQueueNil(t *testing.T) {
	d := t.TempDir()
	saveQueue(d, nil)
	data, _ := os.ReadFile(d + "/queue.json")
	var entries []string
	json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty queue for nil, got %v", entries)
	}
}

func makeTestModel() *tuiModel {
	return &tuiModel{
		ready:  true,
		screen: screenPodcasts,
		width:  80,
		height: 24,
		podcasts: []tuiPodcast{
			{
				name: "Tech Podcast",
				dir:  "/tmp/pods/tech",
				episodes: []tuiEpisode{
					{filename: "ep101.mp3", path: "/tmp/pods/tech/ep101.mp3", hasAdsRemoved: true, fileSize: 2048, duration: 1800, durationDone: true},
					{filename: "ep102.mp3", path: "/tmp/pods/tech/ep102.mp3", hasAdsRemoved: false, fileSize: 1024, duration: 2400, durationDone: true},
					{filename: "ep103.mp3", path: "/tmp/pods/tech/ep103.mp3", hasAdsRemoved: true, fileSize: 4096, duration: 3600, durationDone: true},
				},
			},
			{
				name: "News Daily",
				dir:  "/tmp/pods/news",
				episodes: []tuiEpisode{
					{filename: "today.mp3", path: "/tmp/pods/news/today.mp3", hasAdsRemoved: false, fileSize: 512},
				},
			},
		},
		podIdx:    0,
		podScroll: 0,
		epIdx:     1,
		epScroll:  0,
		queue: map[string][]string{
			"/tmp/pods/tech": {"ep102.mp3"},
			"/tmp/pods/news": nil,
		},
		bk: &TuiBackend{},
	}
}

func sendKey(m *tuiModel, s string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestTUINavigationUpDown(t *testing.T) {
	m := makeTestModel()

	m.handleDown()
	if m.podIdx != 1 {
		t.Errorf("down should advance podIdx to 1, got %d", m.podIdx)
	}

	m.handleDown()
	if m.podIdx != 1 {
		t.Errorf("at bottom, podIdx should stay 1, got %d", m.podIdx)
	}

	m.handleUp()
	if m.podIdx != 0 {
		t.Errorf("up should go back to 0, got %d", m.podIdx)
	}

	m.handleUp()
	if m.podIdx != 0 {
		t.Errorf("at top, podIdx should stay 0, got %d", m.podIdx)
	}
}

func TestTUINavigationEpisodes(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcastDetail

	m.handleDown()
	if m.epIdx != 2 {
		t.Errorf("epIdx should be 2 after down from 1, got %d", m.epIdx)
	}

	m.handleDown()
	if m.epIdx != 2 {
		t.Errorf("at bottom epIdx should stay 2, got %d", m.epIdx)
	}

	m.handleUp()
	if m.epIdx != 1 {
		t.Errorf("epIdx should be 1 after up, got %d", m.epIdx)
	}
}

func TestTUINavigationEnter(t *testing.T) {
	m := makeTestModel()

	m.handleEnter()
	if m.screen != screenPodcastDetail {
		t.Errorf("enter should go to podcast detail, got %d", m.screen)
	}
	if m.epIdx != 0 {
		t.Errorf("epIdx should reset to 0, got %d", m.epIdx)
	}

	m.handleEnter()
	if m.screen != screenEpisodeDetail {
		t.Errorf("enter should go to episode detail, got %d", m.screen)
	}
}

func TestTUINavigationEscape(t *testing.T) {
	m := makeTestModel()

	m.screen = screenEpisodeDetail
	m.handleEscape()
	if m.screen != screenPodcastDetail {
		t.Errorf("esc from episode should go to podcast detail, got %d", m.screen)
	}

	m.handleEscape()
	if m.screen != screenPodcasts {
		t.Errorf("esc from podcast detail should go to podcasts, got %d", m.screen)
	}

	m.handleEscape()
	if m.screen != screenPodcasts {
		t.Errorf("esc at root should stay on podcasts, got %d", m.screen)
	}
}

func TestTUIKeyQuit(t *testing.T) {
	m := makeTestModel()
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("q at podcast screen should trigger quit")
	}

	m2 := makeTestModel()
	m2.screen = screenPodcastDetail
	_, cmd2 := m2.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd2 != nil {
		t.Error("q at detail screen should not quit")
	}
}

func TestTUIKeyCtrlC(t *testing.T) {
	m := makeTestModel()
	m.done = false
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.done {
		t.Error("ctrl+c should set done")
	}
}

func TestTUIQueueToggle(t *testing.T) {
	d := t.TempDir()
	m := makeTestModel()
	m.podcasts[0].dir = d
	m.queue[d] = []string{}
	m.screen = screenPodcastDetail

	m.handleQueueToggle()
	if len(m.queue[d]) != 1 || m.queue[d][0] != "ep102.mp3" {
		t.Errorf("r should add ep102.mp3, got %v", m.queue[d])
	}

	m.handleQueueToggle()
	if len(m.queue[d]) != 0 {
		t.Errorf("r should remove ep102.mp3, got %v", m.queue[d])
	}
}

func TestTUIQueueToggleOutOfRange(t *testing.T) {
	m := makeTestModel()
	m.screen = screenEpisodeDetail
	m.epIdx = 999
	m.handleQueueToggle()
}

func TestTUIQueueToggleOnPodcastScreen(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcasts
	was := len(m.queue["/tmp/pods/tech"])
	m.handleQueueToggle()
	if len(m.queue["/tmp/pods/tech"]) != was {
		t.Error("queue toggle should not work on podcast list screen")
	}
}

func TestTUILoadPodcastsMsg(t *testing.T) {
	m := makeTestModel()
	m.loading = true
	m.podcasts = nil

	msg := loadedPodcastsMsg{
		podcasts: []tuiPodcast{
			{name: "Test", dir: "/tmp/test", episodes: []tuiEpisode{{filename: "e.mp3"}}},
		},
		queue: map[string][]string{"/tmp/test": {"e.mp3"}},
	}

	m.Update(msg)
	if m.loading {
		t.Error("loading should be false after msg")
	}
	if len(m.podcasts) != 1 {
		t.Errorf("expected 1 podcast, got %d", len(m.podcasts))
	}
	if m.queue["/tmp/test"][0] != "e.mp3" {
		t.Error("queue not set correctly")
	}
}

func TestTUILoadPodcastsError(t *testing.T) {
	m := makeTestModel()
	m.Update(loadedPodcastsMsg{err: "test error"})
	if m.loadErr != "test error" {
		t.Errorf("expected loadErr 'test error', got %q", m.loadErr)
	}
}

func TestTUIModelInit(t *testing.T) {
	bk := &TuiBackend{
		LoadPodcasts: func(dir string) ([]tuiPodcast, error) {
			return nil, nil
		},
		LoadQueues: func(pods []tuiPodcast) map[string][]string {
			return map[string][]string{}
		},
	}
	m := newTuiModel(bk, "/tmp/test")

	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
	if !m.loading {
		t.Error("model should be loading after Init")
	}

	msg := cmd()
	switch v := msg.(type) {
	case loadedPodcastsMsg:
		if v.err != "" {
			t.Errorf("unexpected error: %s", v.err)
		}
	default:
		t.Errorf("unexpected message type: %T", v)
	}
}

func TestTUIModelInitError(t *testing.T) {
	bk := &TuiBackend{
		LoadPodcasts: func(dir string) ([]tuiPodcast, error) {
			return nil, os.ErrNotExist
		},
		LoadQueues: func(pods []tuiPodcast) map[string][]string {
			return map[string][]string{}
		},
	}
	m := newTuiModel(bk, "/tmp/test")
	cmd := m.Init()

	msg := cmd()
	switch v := msg.(type) {
	case loadedPodcastsMsg:
		if v.err == "" {
			t.Error("expected error")
		}
	default:
		t.Errorf("expected loadedPodcastsMsg, got %T", v)
	}
}

func TestTUIEpisodeDurationMsg(t *testing.T) {
	m := makeTestModel()
	m.podcasts[0].episodes[0].durationDone = false
	m.podcasts[0].episodes[0].duration = 0

	m.Update(episodeDurationMsg{idx: 0, duration: 1234.5})
	if !m.podcasts[0].episodes[0].durationDone {
		t.Error("durationDone should be true")
	}
	if m.podcasts[0].episodes[0].duration != 1234.5 {
		t.Errorf("duration should be 1234.5, got %f", m.podcasts[0].episodes[0].duration)
	}
}

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

	if strings.Count(view, "\u2713") != 2 {
		t.Errorf("expected 2 checkmarks for ad-free episodes, found %d", strings.Count(view, "\u2713"))
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
	m.screen = screenEpisodeDetail

	view := m.View()

	if !strings.Contains(view, "ep102.mp3") {
		t.Error("should show episode filename")
	}
	if !strings.Contains(view, "1.0 KB") {
		t.Error("should show file size")
	}
	if !strings.Contains(view, "Tech Podcast/ep102.mp3") {
		t.Error("should show path")
	}
	if !strings.Contains(view, "Not removed") {
		t.Error("should show not removed status")
	}
	if !strings.Contains(view, "In queue") {
		t.Error("should show queued status")
	}
}

func TestTUIViewEpisodeDetailAdRemoved(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 0
	m.screen = screenEpisodeDetail

	view := m.View()

	if !strings.Contains(view, "Removed") {
		t.Error("should show Removed status")
	}
	if !strings.Contains(view, "\u2713") {
		t.Error("should show checkmark for ad-free")
	}
}

func TestTUIViewEpisodeDetailNotQueued(t *testing.T) {
	m := makeTestModel()
	m.epIdx = 0
	m.queue["/tmp/pods/tech"] = nil
	m.screen = screenEpisodeDetail

	view := m.View()

	if strings.Contains(view, "In queue") {
		t.Error("should not show queued for non-queued episode")
	}
	if !strings.Contains(view, "Not queued") {
		t.Error("should show Not queued")
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

	// Enter search mode
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searchMode {
		t.Error("should be in search mode after /")
	}

	// Type characters
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m.searchQuery != "tes" {
		t.Errorf("search query should be 'tes', got %q", m.searchQuery)
	}

	// Backspace
	m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchQuery != "te" {
		t.Errorf("search query should be 'te' after backspace, got %q", m.searchQuery)
	}

	// Escape clears and exits search
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

	// Check initial order: ep101, ep102, ep103 (by modTime)
	if m.podcasts[0].episodes[0].filename != "ep101.mp3" {
		t.Errorf("expected ep101 first, got %s", m.podcasts[0].episodes[0].filename)
	}

	m.handleSortToggle()

	// After toggle, order should be reversed
	if m.podcasts[0].episodes[0].filename != "ep103.mp3" {
		t.Errorf("expected ep103 first after sort toggle, got %s", m.podcasts[0].episodes[0].filename)
	}

	// Toggle again
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
