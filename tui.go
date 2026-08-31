package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type tuiScreen int

const (
	screenPodcasts tuiScreen = iota
	screenPodcastDetail
	screenEpisodeDetail
	screenPlayer
	screenPlayQueue
	screenAdQueue
	screenTranscript
	screenTimeline
)

type tuiPodcast struct {
	name        string
	dir         string
	author      string
	description string
	feedURL     string
	coverPath   string
	episodes    []tuiEpisode
	absData     *absItem
	config      PodcastConfig
}

func (p tuiPodcast) transcribedCount() int {
	c := 0
	for _, e := range p.episodes {
		if e.hasTranscript {
			c++
		}
	}
	return c
}

func (p tuiPodcast) displayAuthor() string {
	if p.absData != nil && p.absData.Media.Metadata.Author != "" {
		return p.absData.Media.Metadata.Author
	}
	return p.author
}

func (p tuiPodcast) displayDescription() string {
	if p.absData != nil && p.absData.Media.Metadata.Description != "" {
		return p.absData.Media.Metadata.Description
	}
	return p.description
}

type tuiEpisode struct {
	filename      string
	path          string
	title         string
	hasAdsRemoved bool
	hasTranscript bool
	fileSize      int64
	modTime       time.Time
	publishedAt   int64
	duration      float64
	durationDone  bool
	season        string
	episode       string
	absData       *absEpisode
}

func (e tuiEpisode) displayDate() time.Time {
	if e.publishedAt > 0 {
		return time.UnixMilli(e.publishedAt)
	}
	if e.absData != nil {
		if pub := parseABSEpisodePublishedAt(e.absData); pub > 0 {
			return time.UnixMilli(pub)
		}
	}
	return e.modTime
}

func (e tuiEpisode) displayTitle() string {
	if e.title != "" {
		return e.title
	}
	if e.absData != nil && e.absData.Title != "" {
		return e.absData.Title
	}
	return e.filename
}

func (e tuiEpisode) displayEpisodeNum(index int) string {
	if e.episode != "" {
		return "#" + e.episode
	}
	if e.absData != nil && e.absData.Episode != "" {
		return "#" + e.absData.Episode
	}
	if index > 0 {
		return fmt.Sprintf("#%d", index)
	}
	return ""
}

type TuiBackend struct {
	LoadPodcasts func(dir string) ([]tuiPodcast, error)
	LoadQueues   func(pods []tuiPodcast) map[string][]string
	SaveQueue    func(dir string, entries []string)
	GetDuration  func(path string) float64
}

type tuiModel struct {
	width                   int
	height                  int
	ready                   bool
	screen                  tuiScreen
	podcasts                []tuiPodcast
	podIdx                  int
	podScroll               int
	epIdx                   int
	epScroll                int
	queue                   map[string][]string
	loading                 bool
	loadErr                 string
	done                    bool
	bk                      *TuiBackend
	podcastsDir             string
	vp                      viewport.Model
	showCover               bool
	searchMode              bool
	searchQuery             string
	showHelp                bool
	popupMsg                string
	popupTimer              int
	marqueeTick             int
	marqueePos              int
	marqueeDir              int
	lastMarqueeSelection    string
	descScroll              int
	prevScreen              tuiScreen
	pqIdx                   int
	pqScroll                int
	pqGrabbed               bool
	adqIdx                  int
	adqScroll               int
	adqGrabbed              bool
	transcriptScroll        int
	transcriptLines         []string
	transcriptItems         []transcriptItem
	transcriptViewMode      int
	transcriptLoadedFor     string
	timelineScroll          int
	transcriptMatchIdx      int
	toast                   *Toast
	showHelpModal           bool
	showPolicyModal         bool
	policyModalIdx          int
	showDownloadPolicyModal bool
	downloadPolicyModalIdx  int
	downloadPolicyModalK    int
	selectedEpisodes        map[string]bool
	showEpisodePlayerPane   bool
}

type playerTickMsg time.Time

func playerTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return playerTickMsg(t)
	})
}

type loadedPodcastsMsg struct {
	podcasts []tuiPodcast
	queue    map[string][]string
	err      string
}

type episodeDurationMsg struct {
	idx      int
	duration float64
}

func newTuiModel(bk *TuiBackend, podcastsDir string, cfg *Config) *tuiModel {
	if cfg != nil {
		applyTUIColorConfig(cfg.TUIColor)
	}
	return &tuiModel{
		screen:           screenPodcasts,
		loading:          true,
		bk:               bk,
		podcastsDir:      podcastsDir,
		vp:               viewport.New(70, 20),
		showCover:        isKittySupported(),
		selectedEpisodes: make(map[string]bool),
	}
}

func (m *tuiModel) Init() tea.Cmd {
	loadCmd := func() tea.Msg {
		pods, err := m.bk.LoadPodcasts(m.podcastsDir)
		if err != nil {
			return loadedPodcastsMsg{err: err.Error()}
		}
		queue := m.bk.LoadQueues(pods)
		return loadedPodcastsMsg{podcasts: pods, queue: queue}
	}
	return tea.Batch(loadCmd, playerTickCmd())
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vp.Width = max(20, msg.Width-4)
		m.vp.Height = max(5, msg.Height-8)
		m.ready = true

	case loadedPodcastsMsg:
		m.loading = false
		if msg.err != "" {
			m.loadErr = msg.err
		} else {
			m.podcasts = msg.podcasts
			m.queue = msg.queue
			if len(msg.podcasts) > 0 && m.showCover {
				prewarmPodcastCovers(msg.podcasts, 24, 7)
			}
		}

	case episodeDurationMsg:
		if m.podIdx >= 0 && m.podIdx < len(m.podcasts) {
			if msg.idx >= 0 && msg.idx < len(m.podcasts[m.podIdx].episodes) {
				ep := &m.podcasts[m.podIdx].episodes[msg.idx]
				ep.duration = msg.duration
				ep.durationDone = true
			}
		}

	case playerTickMsg:
		globalPlayer.UpdatePosition()
		return m, playerTickCmd()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		m.tickPopup()
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) visibleLines(headerLines int) int {
	n := m.height - headerLines - 2
	if n < 1 {
		n = 1
	}
	return n
}

func (m *tuiModel) filteredPodcasts() []tuiPodcast {
	if m.searchQuery == "" {
		return m.podcasts
	}
	q := strings.ToLower(m.searchQuery)
	var exact []tuiPodcast
	for _, p := range m.podcasts {
		if strings.Contains(strings.ToLower(p.name), q) || strings.Contains(strings.ToLower(p.displayAuthor()), q) {
			exact = append(exact, p)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	var fuzzy []tuiPodcast
	for _, p := range m.podcasts {
		if matched, _, _ := fuzzyMatch(m.searchQuery, p.name); matched {
			fuzzy = append(fuzzy, p)
		}
	}
	return fuzzy
}

func (m *tuiModel) filteredEpisodes() []tuiEpisode {
	if m.podIdx >= len(m.podcasts) {
		return nil
	}
	if m.searchQuery == "" {
		return m.podcasts[m.podIdx].episodes
	}
	q := strings.ToLower(m.searchQuery)
	var exact []tuiEpisode
	for _, e := range m.podcasts[m.podIdx].episodes {
		if strings.Contains(strings.ToLower(e.displayTitle()), q) || strings.Contains(strings.ToLower(e.filename), q) {
			exact = append(exact, e)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	var fuzzy []tuiEpisode
	for _, e := range m.podcasts[m.podIdx].episodes {
		matched, _, _ := fuzzyMatch(m.searchQuery, e.displayTitle())
		if !matched {
			matched, _, _ = fuzzyMatch(m.searchQuery, e.filename)
		}
		if matched {
			fuzzy = append(fuzzy, e)
		}
	}
	return fuzzy
}

func (m *tuiModel) showPopup(msg string) {
	m.popupMsg = msg
	m.popupTimer = 10
}

func (m *tuiModel) tickPopup() {
	if m.popupTimer > 0 {
		m.popupTimer--
		if m.popupTimer == 0 {
			m.popupMsg = ""
		}
	}
}

func (m *tuiModel) drawPopup() string {
	if m.popupMsg == "" {
		return ""
	}
	return tuiPopupStyle.Render("  " + m.popupMsg)
}

func (m *tuiModel) marqueeText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}
	selKey := fmt.Sprintf("%d-%d", m.podIdx, m.epIdx)
	if selKey != m.lastMarqueeSelection {
		m.lastMarqueeSelection = selKey
		m.marqueePos = 0
		m.marqueeDir = 1
		m.marqueeTick = 0
	}
	m.marqueeTick++
	if m.marqueeTick%3 == 0 {
		m.marqueePos += m.marqueeDir
		if m.marqueePos >= len(text)-maxWidth {
			m.marqueeDir = -1
		}
		if m.marqueePos <= 0 {
			m.marqueeDir = 1
		}
	}
	start := m.marqueePos
	if start < 0 {
		start = 0
	}
	if start+maxWidth > len(text) {
		start = len(text) - maxWidth
	}
	return text[start : start+maxWidth]
}

func (m *tuiModel) setTerminalTitle(title string) {
	fmt.Printf("\033]0;%s\007", title)
}

func (m *tuiModel) drawErrorScreen() string {
	out := &strings.Builder{}
	out.WriteString(tuiTitleStyle.Render("  Connection Error"))
	out.WriteByte('\n')
	out.WriteByte('\n')
	out.WriteString(tuiDimStyle.Render("  " + m.loadErr))
	out.WriteByte('\n')
	out.WriteByte('\n')
	out.WriteString(tuiHelpStyle.Render("  R - Retry  Q - Quit"))
	out.WriteByte('\n')
	return out.String()
}

func (m *tuiModel) searchBar() string {
	if !m.searchMode {
		return ""
	}
	prompt := fmt.Sprintf("  Search: %s█", m.searchQuery)
	if m.screen == screenTranscript {
		prompt = fmt.Sprintf("  Search Transcript: %s█", m.searchQuery)
	}
	return tuiSearchStyle.Render(prompt)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func (m *tuiModel) View() string {
	if m.loading {
		return "Loading podcasts..."
	}
	if m.loadErr != "" {
		return m.drawErrorScreen()
	}
	if !m.ready {
		return "Initializing..."
	}

	if m.screen == screenPodcastDetail && m.podIdx < len(m.podcasts) {
		m.setTerminalTitle("abs - " + m.podcasts[m.podIdx].name)
	} else if m.screen == screenEpisodeDetail && m.podIdx < len(m.podcasts) && m.epIdx < len(m.podcasts[m.podIdx].episodes) {
		m.setTerminalTitle("abs - " + m.podcasts[m.podIdx].episodes[m.epIdx].filename)
	} else {
		m.setTerminalTitle("abs")
	}

	if m.showHelpModal {
		return m.drawHelpModal()
	}
	if m.showPolicyModal {
		return m.drawAdPolicyModal()
	}
	if m.showDownloadPolicyModal {
		return m.drawDownloadPolicyModal()
	}

	var body strings.Builder
	if isKittyTerminal() {
		body.WriteString(kittyClearGraphics())
	}
	body.WriteString(m.renderTopNavBar())

	switch m.screen {
	case screenPodcasts:
		body.WriteString(m.drawPodcastsList())
	case screenPodcastDetail:
		body.WriteString(m.drawPodcastDetail())
	case screenEpisodeDetail:
		body.WriteString(m.drawEpisodeDetail())
	case screenPlayer:
		body.WriteString(m.drawPlayerScreen())
	case screenPlayQueue:
		body.WriteString(m.drawPlayQueueScreen())
	case screenAdQueue:
		body.WriteString(m.drawAdQueueScreen())
	case screenTranscript:
		body.WriteString(m.drawTranscriptScreen())
	case screenTimeline:
		body.WriteString(m.drawTimelineScreen())
	}

	miniPlayer := m.renderMiniPlayerBar()
	if miniPlayer != "" {
		body.WriteString(miniPlayer)
	}

	toast := m.renderToastNotification()
	if toast != "" {
		body.WriteString(toast)
	} else {
		popup := m.drawPopup()
		if popup != "" {
			body.WriteString("\n" + popup)
		}
	}

	return body.String()
}
