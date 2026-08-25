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

type TuiBackend struct {
	LoadPodcasts func(dir string) ([]tuiPodcast, error)
	LoadQueues   func(pods []tuiPodcast) map[string][]string
	SaveQueue    func(dir string, entries []string)
	GetDuration  func(path string) float64
}

type tuiModel struct {
	width                int
	height               int
	ready                bool
	screen               tuiScreen
	podcasts             []tuiPodcast
	podIdx               int
	podScroll            int
	epIdx                int
	epScroll             int
	queue                map[string][]string
	loading              bool
	loadErr              string
	done                 bool
	bk                   *TuiBackend
	podcastsDir          string
	vp                   viewport.Model
	showCover            bool
	searchMode           bool
	searchQuery          string
	showHelp             bool
	popupMsg             string
	popupTimer           int
	marqueeTick          int
	marqueePos           int
	marqueeDir           int
	lastMarqueeSelection string
	descScroll           int
	prevScreen           tuiScreen
	pqIdx                int
	pqScroll             int
	pqGrabbed            bool
	adqIdx               int
	adqScroll            int
	adqGrabbed           bool
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
		screen:      screenPodcasts,
		loading:     true,
		bk:          bk,
		podcastsDir: podcastsDir,
		vp:          viewport.New(70, 20),
		showCover:   isKittySupported(),
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

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		m.tickPopup()
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	s := msg.String()

	// Handle error screen keys
	if m.loadErr != "" {
		switch s {
		case "r", "R":
			m.loadErr = ""
			m.loading = true
			return m, m.Init()
		case "q", "Q", "ctrl+c", "ctrl+d":
			m.done = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch s {
	case "q", "Q", "ctrl+c", "ctrl+d":
		globalPlayer.Stop()
		m.done = true
		return m, tea.Quit

	case "f1", "F1":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenPlayer
		return m, nil

	case "f2", "F2":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenPlayQueue
		m.pqGrabbed = false
		return m, nil

	case "f3", "F3":
		if m.screen != screenPlayer && m.screen != screenPlayQueue && m.screen != screenAdQueue {
			m.prevScreen = m.screen
		}
		m.screen = screenAdQueue
		m.adqGrabbed = false
		return m, nil
	}

	if m.screen == screenPlayer {
		switch s {
		case "esc":
			m.handleEscape()
			return m, nil
		}
	}

	if m.screen == screenPlayQueue {
		unified := globalPlayer.GetUnifiedQueue()
		total := len(unified)

		switch s {
		case "up", "k":
			if m.pqGrabbed && m.pqIdx > 0 {
				globalPlayer.MoveUnifiedItem(m.pqIdx, m.pqIdx-1)
				m.pqIdx--
			} else if m.pqIdx > 0 {
				m.pqIdx--
			}
		case "down", "j":
			if m.pqGrabbed && m.pqIdx < total-1 {
				globalPlayer.MoveUnifiedItem(m.pqIdx, m.pqIdx+1)
				m.pqIdx++
			} else if m.pqIdx < total-1 {
				m.pqIdx++
			}
		case " ":
			m.pqGrabbed = !m.pqGrabbed
			if m.pqGrabbed {
				m.showPopup("Grabbed (↑/↓ to move)")
			} else {
				m.showPopup("Placed item")
			}
		case "enter":
			if m.pqGrabbed {
				m.pqGrabbed = false
				m.showPopup("Placed item")
			} else if m.pqIdx < total {
				item := unified[m.pqIdx]
				if item.IsCurrent {
					globalPlayer.TogglePause()
					if globalPlayer.IsPaused {
						m.showPopup("Paused")
					} else {
						m.showPopup("Resumed")
					}
				} else {
					globalPlayer.RemoveUnifiedItem(m.pqIdx)
					globalPlayer.PlayTrack(item.Track)
					m.showPopup("Playing: " + truncate(item.Track.Title, 25))
				}
			}
		case "d", "x", "delete", "backspace":
			if m.pqIdx < total {
				item := unified[m.pqIdx]
				globalPlayer.RemoveUnifiedItem(m.pqIdx)
				m.showPopup("Removed: " + truncate(item.Track.Title, 25))
				if m.pqIdx >= total-1 && m.pqIdx > 0 {
					m.pqIdx--
				}
			}
		case "c", "C":
			globalPlayer.ClearQueue()
			m.showPopup("Play queue cleared")
		case "esc":
			m.handleEscape()
		}
		return m, nil
	}

	if m.screen == screenAdQueue {
		items := getAllAdQueueItems(m.podcasts, m.queue)
		switch s {
		case "up", "k":
			if m.adqGrabbed && m.adqIdx > 0 {
				moveAdQueueItem(items, m.adqIdx, m.adqIdx-1, m.queue, m.bk.SaveQueue)
				m.adqIdx--
			} else if m.adqIdx > 0 {
				m.adqIdx--
			}
		case "down", "j":
			if m.adqGrabbed && m.adqIdx < len(items)-1 {
				moveAdQueueItem(items, m.adqIdx, m.adqIdx+1, m.queue, m.bk.SaveQueue)
				m.adqIdx++
			} else if m.adqIdx < len(items)-1 {
				m.adqIdx++
			}
		case " ":
			m.adqGrabbed = !m.adqGrabbed
			if m.adqGrabbed {
				m.showPopup("Grabbed (↑/↓ to move)")
			} else {
				m.showPopup("Placed item")
			}
		case "enter":
			m.adqGrabbed = false
		case "d", "x", "delete", "backspace", "r", "R":
			if m.adqIdx < len(items) {
				removeAdQueueItem(items[m.adqIdx], m.queue, m.bk.SaveQueue)
				m.showPopup("Removed from ad queue")
				if m.adqIdx >= len(items)-1 && m.adqIdx > 0 {
					m.adqIdx--
				}
			}
		case "esc":
			m.handleEscape()
		}
		return m, nil
	}

	switch s {

	case "p", "P":
		if m.screen == screenPodcastDetail || m.screen == screenEpisodeDetail {
			m.playSelectedEpisode()
		}

	case " ":
		globalPlayer.TogglePause()
		if globalPlayer.IsPaused {
			m.showPopup("Paused")
		} else if globalPlayer.IsPlaying {
			m.showPopup("Resumed")
		}

	case "right", "l", ">":
		if globalPlayer.IsPlaying {
			globalPlayer.Seek(30)
			m.showPopup("+30s (" + formatPlayerTime(globalPlayer.Position) + ")")
		}

	case "left", "h", "<":
		if globalPlayer.IsPlaying {
			globalPlayer.Seek(-30)
			m.showPopup("-30s (" + formatPlayerTime(globalPlayer.Position) + ")")
		}

	case "+", "=", "]":
		globalPlayer.VolumeUp()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.Volume))

	case "-", "_", "[":
		globalPlayer.VolumeDown()
		m.showPopup(fmt.Sprintf("Volume: %d%%", globalPlayer.Volume))

	case "m", "M":
		globalPlayer.ToggleMute()
		if globalPlayer.Muted {
			m.showPopup("Muted")
		} else {
			m.showPopup("Unmuted")
		}

	case "s", "S":
		globalPlayer.CycleSpeaker()
		m.showPopup("Speaker: " + globalPlayer.CurrentSpeaker)

	case "n", "N":
		globalPlayer.Next()
		m.showPopup("Next track")

	case "c", "C":
		globalPlayer.ClearQueue()
		m.showPopup("Queue cleared")

	case "up", "k":
		if m.screen == screenEpisodeDetail {
			if m.descScroll > 0 {
				m.descScroll--
			}
		} else {
			m.handleUp()
		}

	case "down", "j":
		if m.screen == screenEpisodeDetail {
			m.descScroll++
		} else {
			m.handleDown()
		}

	case "enter":
		return m.handleEnter()

	case "esc":
		m.handleEscape()

	case "r", "R":
		m.handleQueueToggle()

	case "i", "I":
		m.showCover = !m.showCover

	case "b", "B":
		m.showHelp = !m.showHelp

	case "d", "D":
		m.handleSortToggle()

	case "/":
		m.searchMode = true
		m.searchQuery = ""
	}

	return m, nil
}

func (m *tuiModel) playSelectedEpisode() {
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]
	track := PlayerTrack{
		Title:    ep.displayTitle(),
		Podcast:  pod.name,
		Path:     ep.path,
		Duration: ep.duration,
	}
	addedToPlayer := globalPlayer.EnqueueAndPlay(track)

	addedToAdQueue := false
	if !ep.hasAdsRemoved {
		entries := m.queue[pod.dir]
		found := false
		for _, q := range entries {
			if q == ep.filename {
				found = true
				break
			}
		}
		if !found {
			m.queue[pod.dir] = append(entries, ep.filename)
			if m.bk.SaveQueue != nil {
				m.bk.SaveQueue(pod.dir, m.queue[pod.dir])
			}
			addedToAdQueue = true
		}
	}

	if addedToPlayer {
		if addedToAdQueue {
			m.showPopup("Queued (play & ad removal): " + truncate(ep.displayTitle(), 25))
		} else {
			m.showPopup("Queued for playback: " + truncate(ep.displayTitle(), 25))
		}
	} else {
		if addedToAdQueue {
			m.showPopup("Queued for ad removal: " + truncate(ep.displayTitle(), 25))
		} else {
			m.showPopup("Already playing or queued")
		}
	}
}

func (m *tuiModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.searchMode = false
		m.podIdx = 0
		m.podScroll = 0
		m.epIdx = 0
		m.epScroll = 0
	case tea.KeyEscape:
		m.searchMode = false
		m.searchQuery = ""
	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.podIdx = 0
		m.podScroll = 0
		m.epIdx = 0
		m.epScroll = 0
	}
	return m, nil
}

func (m *tuiModel) handleSortToggle() {
	if m.screen != screenPodcastDetail {
		return
	}
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	// Reverse episode order
	for i, j := 0, len(pod.episodes)-1; i < j; i, j = i+1, j-1 {
		pod.episodes[i], pod.episodes[j] = pod.episodes[j], pod.episodes[i]
	}
	m.epIdx = 0
	m.epScroll = 0
}

func (m *tuiModel) handleUp() {
	switch m.screen {
	case screenPodcasts:
		pods := m.filteredPodcasts()
		if m.podIdx > 0 && m.podIdx < len(pods) {
			m.podIdx--
			if m.podIdx < m.podScroll {
				m.podScroll = m.podIdx
			}
		}
	case screenPodcastDetail:
		if m.epIdx > 0 {
			m.epIdx--
			if m.epIdx < m.epScroll {
				m.epScroll = m.epIdx
			}
		}
	}
}

func (m *tuiModel) handleDown() {
	switch m.screen {
	case screenPodcasts:
		pods := m.filteredPodcasts()
		if m.podIdx < len(pods)-1 {
			m.podIdx++
			maxVis := m.visibleLines(4)
			if m.podIdx-m.podScroll >= maxVis {
				m.podScroll = m.podIdx - maxVis + 1
			}
		}
	case screenPodcastDetail:
		eps := m.filteredEpisodes()
		if m.epIdx < len(eps)-1 {
			m.epIdx++
			maxVis := m.visibleLines(5)
			if m.epIdx-m.epScroll >= maxVis {
				m.epScroll = m.epIdx - maxVis + 1
			}
		}
	}
}

func (m *tuiModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenPodcasts:
		pods := m.filteredPodcasts()
		if len(pods) > 0 && m.podIdx < len(pods) {
			// Map filtered index back to actual podcast index
			selectedPod := pods[m.podIdx]
			for i, p := range m.podcasts {
				if p.dir == selectedPod.dir {
					m.podIdx = i
					break
				}
			}
			m.epIdx = 0
			m.epScroll = 0
			m.screen = screenPodcastDetail
		}

	case screenPodcastDetail:
		eps := m.filteredEpisodes()
		if m.podIdx < len(m.podcasts) && m.epIdx < len(eps) {
			m.screen = screenEpisodeDetail
			selectedEp := eps[m.epIdx]
			// Map filtered episode index back to actual episode index
			pod := &m.podcasts[m.podIdx]
			for i, e := range pod.episodes {
				if e.path == selectedEp.path {
					m.epIdx = i
					break
				}
			}
			ep := &pod.episodes[m.epIdx]
			if !ep.durationDone {
				idx := m.epIdx
				return m, func() tea.Msg {
					dur := m.bk.GetDuration(ep.path)
					return episodeDurationMsg{idx: idx, duration: dur}
				}
			}
		}
	}
	return m, nil
}

func (m *tuiModel) handleEscape() {
	switch m.screen {
	case screenPodcastDetail:
		m.screen = screenPodcasts
	case screenEpisodeDetail:
		m.screen = screenPodcastDetail
	case screenPlayer, screenPlayQueue, screenAdQueue:
		m.screen = m.prevScreen
		m.pqGrabbed = false
		m.adqGrabbed = false
	}
}

func (m *tuiModel) handleQueueToggle() {
	if m.screen != screenPodcastDetail && m.screen != screenEpisodeDetail {
		return
	}
	if m.podIdx >= len(m.podcasts) {
		return
	}
	pod := &m.podcasts[m.podIdx]
	eps := m.filteredEpisodes()
	if m.epIdx >= len(eps) {
		return
	}
	ep := eps[m.epIdx]
	entries := m.queue[pod.dir]
	if entries == nil {
		entries = []string{}
	}
	found := false
	for i, e := range entries {
		if e == ep.filename {
			m.queue[pod.dir] = append(entries[:i], entries[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		if ep.hasAdsRemoved {
			m.showPopup("Episode already has ads removed")
			return
		}
		m.queue[pod.dir] = append(entries, ep.filename)
		m.showPopup("Added to ad removal queue")
	} else {
		m.showPopup("Removed from ad removal queue")
	}
	if m.bk.SaveQueue != nil {
		m.bk.SaveQueue(pod.dir, m.queue[pod.dir])
	}
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
	var filtered []tuiPodcast
	for _, p := range m.podcasts {
		if strings.Contains(strings.ToLower(p.name), q) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (m *tuiModel) filteredEpisodes() []tuiEpisode {
	if m.podIdx >= len(m.podcasts) {
		return nil
	}
	if m.searchQuery == "" {
		return m.podcasts[m.podIdx].episodes
	}
	q := strings.ToLower(m.searchQuery)
	var filtered []tuiEpisode
	for _, e := range m.podcasts[m.podIdx].episodes {
		if strings.Contains(strings.ToLower(e.filename), q) {
			filtered = append(filtered, e)
		}
	}
	return filtered
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
	// Reset marquee when selection changes
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

	// Set terminal title
	if m.screen == screenPodcastDetail && m.podIdx < len(m.podcasts) {
		m.setTerminalTitle("abs - " + m.podcasts[m.podIdx].name)
	} else if m.screen == screenEpisodeDetail && m.podIdx < len(m.podcasts) && m.epIdx < len(m.podcasts[m.podIdx].episodes) {
		m.setTerminalTitle("abs - " + m.podcasts[m.podIdx].episodes[m.epIdx].filename)
	} else {
		m.setTerminalTitle("abs")
	}

	var body string
	switch m.screen {
	case screenPodcasts:
		body = m.drawPodcastsList()
	case screenPodcastDetail:
		body = m.drawPodcastDetail()
	case screenEpisodeDetail:
		body = m.drawEpisodeDetail()
	case screenPlayer:
		body = m.drawPlayerScreen()
	case screenPlayQueue:
		body = m.drawPlayQueueScreen()
	case screenAdQueue:
		body = m.drawAdQueueScreen()
	}

	if m.searchMode {
		body += "\n" + m.searchBar()
	}

	popup := m.drawPopup()
	if popup != "" {
		body += "\n" + popup
	}

	return body
}

func (m *tuiModel) drawPodcastsList() string {
	out := &strings.Builder{}

	titleBanner := tuiHeaderBanner.Render(" PODCASTS ")
	out.WriteString(fmt.Sprintf("  %s\n", titleBanner))

	pods := m.filteredPodcasts()

	totalEps := 0
	totalDone := 0
	for _, p := range pods {
		totalEps += len(p.episodes)
		for _, e := range p.episodes {
			if e.hasAdsRemoved {
				totalDone++
			}
		}
	}

	statPill := tuiStatStyle.Render(fmt.Sprintf("  %d podcasts, %d episodes, %d ad-free", len(pods), totalEps, totalDone))
	out.WriteString(statPill)
	out.WriteByte('\n')

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	out.WriteByte('\n')

	maxVis := m.visibleLines(4)
	start := m.podScroll
	end := start + maxVis
	if end > len(pods) {
		end = len(pods)
	}

	if m.width >= 65 && len(pods) > 0 {
		leftW := min(36, max(26, m.width*32/100))
		rightW := max(30, m.width-leftW-5)

		var leftLines []string
		for i := start; i < end; i++ {
			p := pods[i]
			epCount := len(p.episodes)
			doneCount := 0
			for _, e := range p.episodes {
				if e.hasAdsRemoved {
					doneCount++
				}
			}

			nameStr := displayName(p.name)
			statsStr := fmt.Sprintf("(%d/%d)", epCount, doneCount)

			line := fmt.Sprintf("  %s %s", nameStr, statsStr)
			truncLine := truncate(line, leftW-2)
			pad := strings.Repeat(" ", max(0, leftW-len([]rune(truncLine))))
			fullLine := truncLine + pad

			if i == m.podIdx {
				leftLines = append(leftLines, tuiSelectedStyle.Render(fullLine))
			} else {
				leftLines = append(leftLines, fullLine)
			}
		}

		// Build right pane for selected podcast with rich details
		var rightLines []string
		if m.podIdx < len(pods) {
			selPod := pods[m.podIdx]

			// 1. Cover Art (if available and enabled)
			if isKittySupported() && m.showCover {
				coverPath := findCoverImage(selPod.dir)
				if coverPath != "" {
					imgW := min(rightW-4, 24)
					imgH := min(7, max(4, maxVis/3))
					if imgEsc, err := encodeKittyGraphicsFile(coverPath, imgW, imgH); err == nil && imgEsc != "" {
						if isKittyTerminal() {
							rightLines = append(rightLines, imgEsc)
							for h := 1; h < imgH; h++ {
								rightLines = append(rightLines, strings.Repeat(" ", max(0, imgW)))
							}
						} else {
							for _, imgLine := range strings.Split(imgEsc, "\n") {
								if imgLine != "" {
									rightLines = append(rightLines, imgLine)
								}
							}
						}
					}
				}
			}

			// 2. Podcast Title
			title := displayName(selPod.name)
			rightLines = append(rightLines, tuiTitleStyle.Render(truncate(title, rightW-2)))

			// 3. Author
			if author := selPod.displayAuthor(); author != "" {
				rightLines = append(rightLines, tuiSubtitleStyle.Render(truncate("by "+displayName(author), rightW-2)))
			}

			// 4. Detailed Stats & Timeline
			selDone := 0
			var totalDurSec float64
			var newestDate, oldestDate time.Time
			for _, e := range selPod.episodes {
				if e.hasAdsRemoved {
					selDone++
				}
				totalDurSec += e.duration
				d := e.displayDate()
				if !d.IsZero() {
					if newestDate.IsZero() || d.After(newestDate) {
						newestDate = d
					}
					if oldestDate.IsZero() || d.Before(oldestDate) {
						oldestDate = d
					}
				}
			}

			statParts := []string{
				fmt.Sprintf("%d episodes", len(selPod.episodes)),
				fmt.Sprintf("%d ad-free", selDone),
			}
			if queued := len(m.queue[selPod.dir]); queued > 0 {
				statParts = append(statParts, fmt.Sprintf("%d queued", queued))
			}
			if totalDurSec > 0 {
				hrs := int(totalDurSec) / 3600
				mins := (int(totalDurSec) % 3600) / 60
				if hrs > 0 {
					statParts = append(statParts, fmt.Sprintf("~%dh %dm audio", hrs, mins))
				} else {
					statParts = append(statParts, fmt.Sprintf("~%dm audio", mins))
				}
			}
			rightLines = append(rightLines, tuiStatStyle.Render(truncate(strings.Join(statParts, " • "), rightW-2)))

			if !newestDate.IsZero() {
				dateInfo := fmt.Sprintf("Timeline: %s", newestDate.Format("2006-01-02"))
				if !oldestDate.IsZero() && !oldestDate.Equal(newestDate) {
					dateInfo += fmt.Sprintf(" (oldest: %s)", oldestDate.Format("2006-01-02"))
				}
				rightLines = append(rightLines, tuiSubtextStyle.Render(truncate(dateInfo, rightW-2)))
			}

			// 5. Description (rich multi-line wrapped text)
			if desc := selPod.displayDescription(); desc != "" {
				clean := strings.TrimSpace(renderHTML(desc))
				if len(clean) > 0 {
					rightLines = append(rightLines, "")
					prefix := "About: "
					lines := wrapText(prefix+clean, rightW-2)
					if len(lines) > 0 {
						firstLine := lines[0]
						if strings.HasPrefix(firstLine, prefix) {
							rest := strings.TrimPrefix(firstLine, prefix)
							rightLines = append(rightLines, tuiSectionTitle.Render(prefix)+tuiDimStyle.Render(rest))
						} else {
							rightLines = append(rightLines, tuiDimStyle.Render(firstLine))
						}
						for _, l := range lines[1:] {
							if len(rightLines) >= maxVis-5 && len(selPod.episodes) > 0 {
								break
							}
							if len(rightLines) >= maxVis {
								break
							}
							rightLines = append(rightLines, tuiDimStyle.Render(l))
						}
					}
				}
			}

			// 6. Recent Episodes preview
			if len(selPod.episodes) > 0 && len(rightLines) < maxVis-2 {
				rightLines = append(rightLines, "")
				rightLines = append(rightLines, tuiSectionTitle.Render("Recent Episodes:"))
				for epIdx, ep := range selPod.episodes {
					if len(rightLines) >= maxVis {
						break
					}
					if epIdx >= 5 {
						break
					}
					d := ep.displayDate()
					dStr := ""
					if !d.IsZero() {
						dStr = d.Format("2006-01-02") + " "
					}
					chk := " "
					if ep.hasAdsRemoved {
						chk = "✓ "
					}
					epLine := fmt.Sprintf("  %s%s%s", chk, dStr, ep.displayTitle())
					rightLines = append(rightLines, tuiDimStyle.Render(truncate(epLine, rightW-2)))
				}
			}
		}

		// Join columns
		totalLines := max(len(leftLines), len(rightLines))
		for k := 0; k < totalLines; k++ {
			leftPart := ""
			if k < len(leftLines) {
				leftPart = leftLines[k]
			} else {
				leftPart = strings.Repeat(" ", leftW)
			}

			rightPart := ""
			if k < len(rightLines) {
				rightPart = rightLines[k]
			}

			out.WriteString(leftPart + tuiDividerStyle.Render(" │ ") + rightPart + "\n")
		}
	} else {
		for i := start; i < end; i++ {
			p := pods[i]
			epCount := len(p.episodes)
			doneCount := 0
			for _, e := range p.episodes {
				if e.hasAdsRemoved {
					doneCount++
				}
			}

			nameStr := displayName(p.name)
			statsStr := fmt.Sprintf("(%d/%d)", epCount, doneCount)

			authorStr := ""
			if author := p.displayAuthor(); author != "" {
				authorStr = fmt.Sprintf(" [%s]", displayName(author))
			}

			line := fmt.Sprintf("  %s  %s%s", nameStr, statsStr, authorStr)
			line = truncate(line, max(1, m.width-1))

			if i == m.podIdx {
				out.WriteString(tuiSelectedStyle.Render(line))
			} else {
				out.WriteString(line)
			}
			out.WriteByte('\n')
		}
	}

	out.WriteByte('\n')
	helpText := "↑↓ navigate  Enter select  / search  q quit"
	if isKittySupported() {
		helpText += "  I cover"
	}
	if m.showHelp {
		helpText += "  B hide help"
	} else {
		helpText = ""
	}
	if len(pods) > maxVis {
		pct := float64(m.podIdx+1) / float64(len(pods)) * 100
		helpText += fmt.Sprintf("  [%d%%]", int(pct))
	}
	if helpText != "" {
		out.WriteString(tuiHelpStyle.Render("  " + helpText))
		out.WriteByte('\n')
	}

	return out.String()
}

func (m *tuiModel) drawPodcastDetail() string {
	out := &strings.Builder{}

	if m.podIdx >= len(m.podcasts) {
		m.screen = screenPodcasts
		return m.drawPodcastsList()
	}

	pod := m.podcasts[m.podIdx]

	title := tuiTitleStyle.Render("  " + displayName(pod.name))
	out.WriteString(title)
	out.WriteByte('\n')

	eps := m.filteredEpisodes()

	total := len(eps)
	done := 0
	for _, e := range eps {
		if e.hasAdsRemoved {
			done++
		}
	}
	queued := len(m.queue[pod.dir])

	statPill := tuiStatStyle.Render(fmt.Sprintf("  %d episodes, %d ad-free, %d queued", total, done, queued))
	out.WriteString(statPill)
	out.WriteByte('\n')

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	out.WriteByte('\n')

	maxVis := m.visibleLines(3)
	start := m.epScroll
	end := start + maxVis
	if end > len(eps) {
		end = len(eps)
	}

	queueEntries := m.queue[pod.dir]
	isQueued := func(filename string) bool {
		for _, q := range queueEntries {
			if q == filename {
				return true
			}
		}
		return false
	}

	renderEpRow := func(i int) {
		ep := eps[i]
		displayNameStr := ep.displayTitle()
		d := ep.displayDate()
		dateStr := strings.Repeat(" ", 10)
		if !d.IsZero() {
			dateStr = d.Format("2006-01-02")
		}

		availWidth := m.width - 2
		isSelected := (i == m.epIdx)

		if availWidth >= 40 {
			qBadgeWidth := 0
			if isQueued(ep.filename) {
				qBadgeWidth = 4
			}
			titleWidth := availWidth - 16 - qBadgeWidth
			if titleWidth < 10 {
				titleWidth = 10
			}
			truncTitle := truncate(displayName(displayNameStr), titleWidth)
			padLen := max(0, titleWidth-len([]rune(truncTitle)))
			padding := strings.Repeat(" ", padLen)

			if isSelected {
				chk := "  "
				if ep.hasAdsRemoved {
					chk = "✓ "
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " [Q]"
				}
				plainLine := "  " + dateStr + "  " + chk + truncTitle + padding + qBadge
				fullPad := max(0, availWidth-len([]rune(plainLine)))
				fullRow := plainLine + strings.Repeat(" ", fullPad)
				out.WriteString(tuiSelectedStyle.Render(fullRow))
			} else {
				chk := "  "
				if ep.hasAdsRemoved {
					chk = tuiGreenStyle.Render("✓") + " "
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " " + tuiBadgeQueued.Render("[Q]")
				}
				line := "  " + tuiSubtextStyle.Render(dateStr) + "  " + chk + truncTitle + padding + qBadge
				out.WriteString(line)
			}
		} else {
			if isSelected {
				chk := " "
				if ep.hasAdsRemoved {
					chk = "✓"
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " [Q]"
				}
				line := dateStr + " " + chk + " " + displayName(displayNameStr) + qBadge
				truncLine := truncate(line, max(1, m.width-1))
				fullPad := max(0, (m.width-1)-len([]rune(truncLine)))
				out.WriteString(tuiSelectedStyle.Render(truncLine + strings.Repeat(" ", fullPad)))
			} else {
				chk := " "
				if ep.hasAdsRemoved {
					chk = tuiGreenStyle.Render("✓")
				}
				qBadge := ""
				if isQueued(ep.filename) {
					qBadge = " " + tuiBadgeQueued.Render("[Q]")
				}
				line := tuiSubtextStyle.Render(dateStr) + " " + chk + " " + displayName(displayNameStr) + qBadge
				out.WriteString(truncate(line, max(1, m.width-1)))
			}
		}
		out.WriteByte('\n')
	}

	// Clear graphics when viewing podcast episodes
	if isKittyTerminal() {
		out.WriteString(kittyClearGraphics())
	}

	for i := start; i < end; i++ {
		renderEpRow(i)
	}

	out.WriteByte('\n')
	helpText := "↑↓ navigate  Enter/p play/info  Space pause  ←/→ seek  +/- vol  Esc back  q quit"
	if isKittySupported() {
		helpText += "  I cover"
	}
	if m.showHelp {
		helpText += "  B hide"
	} else {
		helpText = ""
	}
	if len(eps) > maxVis {
		helpText += fmt.Sprintf("  [%d/%d]", m.epIdx+1, len(eps))
	}
	if helpText != "" {
		out.WriteString(tuiDimStyle.Render("  " + helpText))
		out.WriteByte('\n')
	}

	return out.String()
}
