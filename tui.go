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
)

type tuiPodcast struct {
	name     string
	dir      string
	episodes []tuiEpisode
	absData  *absItem
}

type tuiEpisode struct {
	filename      string
	path          string
	hasAdsRemoved bool
	fileSize      int64
	modTime       time.Time
	duration      float64
	durationDone  bool
	absData       *absEpisode
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
	return func() tea.Msg {
		pods, err := m.bk.LoadPodcasts(m.podcastsDir)
		if err != nil {
			return loadedPodcastsMsg{err: err.Error()}
		}
		queue := m.bk.LoadQueues(pods)
		return loadedPodcastsMsg{podcasts: pods, queue: queue}
	}
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
	case "ctrl+c", "ctrl+d":
		m.done = true
		return m, tea.Quit

	case "q":
		if m.screen == screenPodcasts {
			m.done = true
			return m, tea.Quit
		}
		m.handleEscape()
		return m, nil

	case "up", "k":
		m.handleUp()

	case "down", "j":
		m.handleDown()

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
		if m.podIdx > 0 {
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
		if len(pods) > 0 {
			m.epIdx = 0
			m.epScroll = 0
			m.screen = screenPodcastDetail
		}

	case screenPodcastDetail:
		eps := m.filteredEpisodes()
		if m.podIdx < len(m.podcasts) && m.epIdx < len(eps) {
			m.screen = screenEpisodeDetail
			ep := &eps[m.epIdx]
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
		m.queue[pod.dir] = append(entries, ep.filename)
	}
	if m.bk.SaveQueue != nil {
		m.bk.SaveQueue(pod.dir, m.queue[pod.dir])
	}
	m.showPopup("Queue updated")
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

func disableTerminalScroll() {
	fmt.Print("\x1b[?1049h")
}

func enableTerminalScroll() {
	fmt.Print("\x1b[?1049l")
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
		m.setTerminalTitle("mp3_rm_ads - " + m.podcasts[m.podIdx].name)
	} else if m.screen == screenEpisodeDetail && m.podIdx < len(m.podcasts) && m.epIdx < len(m.podcasts[m.podIdx].episodes) {
		m.setTerminalTitle("mp3_rm_ads - " + m.podcasts[m.podIdx].episodes[m.epIdx].filename)
	} else {
		m.setTerminalTitle("mp3_rm_ads")
	}

	var body string
	switch m.screen {
	case screenPodcasts:
		body = m.drawPodcastsList()
	case screenPodcastDetail:
		body = m.drawPodcastDetail()
	case screenEpisodeDetail:
		body = m.drawEpisodeDetail()
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
		if p.absData != nil && p.absData.Media.Metadata.Author != "" {
			authorStr = fmt.Sprintf(" [%s]", p.absData.Media.Metadata.Author)
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

	out.WriteByte('\n')
	helpText := "↑↓ navigate  Enter select  / search  q quit"
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

	// Cover art on the left side
	var coverEsc string
	if isKittySupported() && m.showCover {
		coverPath := findCoverImage(pod.dir)
		if coverPath != "" {
			imgEsc, err := encodeKittyGraphicsFile(coverPath, 30, 0)
			if err == nil && imgEsc != "" {
				coverEsc = imgEsc
			}
		}
	}

	maxVis := m.visibleLines(5)
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

	if coverEsc != "" {
		// Display cover art above the episode list
		out.WriteString(coverEsc)
		out.WriteByte('\n')

		for i := start; i < end; i++ {
			ep := eps[i]
			prefix := "    "
			if ep.hasAdsRemoved {
				prefix = "  " + tuiGreenStyle.Render("\u2713") + " "
			}

			suffix := ""
			if isQueued(ep.filename) {
				suffix = " " + tuiBadgeQueued.Render("[Q]")
			}

			absTitle := absEpisodeTitle(ep.absData)
			displayNameStr := ep.filename
			if absTitle != "" {
				displayNameStr = absTitle
			}

			line := prefix + displayName(displayNameStr) + suffix
			line = truncate(line, max(1, m.width-1))

			if i == m.epIdx {
				out.WriteString(tuiSelectedStyle.Render(line))
			} else {
				out.WriteString(line)
			}
			out.WriteByte('\n')
		}
	} else {
		for i := start; i < end; i++ {
			ep := eps[i]
			prefix := "    "
			if ep.hasAdsRemoved {
				prefix = "  " + tuiGreenStyle.Render("\u2713") + " "
			}

			suffix := ""
			if isQueued(ep.filename) {
				suffix = " " + tuiBadgeQueued.Render("[Q]")
			}

			absTitle := absEpisodeTitle(ep.absData)
			displayNameStr := ep.filename
			if absTitle != "" {
				displayNameStr = absTitle
			}

			line := prefix + displayName(displayNameStr) + suffix
			line = truncate(line, max(1, m.width-1))

			if i == m.epIdx {
				out.WriteString(tuiSelectedStyle.Render(line))
			} else {
				out.WriteString(line)
			}
			out.WriteByte('\n')
		}
	}

	out.WriteByte('\n')
	helpText := "↑↓ navigate  Enter info  R queue  / search  D sort  Esc back"
	if isKittySupported() {
		helpText += "  I cover"
	}
	if m.showHelp {
		helpText += "  B hide"
	} else {
		helpText = ""
	}
	if len(eps) > maxVis {
		pct := float64(m.epIdx+1) / float64(len(eps)) * 100
		helpText += fmt.Sprintf("  [%d%%]", int(pct))
	}
	if helpText != "" {
		out.WriteString(tuiHelpStyle.Render("  " + helpText))
		out.WriteByte('\n')
	}

	return out.String()
}

func (m *tuiModel) drawEpisodeDetail() string {
	out := &strings.Builder{}

	if m.podIdx >= len(m.podcasts) {
		m.screen = screenPodcasts
		return m.drawPodcastsList()
	}

	pod := m.podcasts[m.podIdx]
	if m.epIdx >= len(pod.episodes) {
		m.screen = screenPodcastDetail
		return m.drawPodcastDetail()
	}

	ep := pod.episodes[m.epIdx]
	queueEntries := m.queue[pod.dir]
	inQueue := false
	for _, q := range queueEntries {
		if q == ep.filename {
			inQueue = true
			break
		}
	}

	displayHeader := ep.filename
	if ep.absData != nil && ep.absData.Title != "" {
		displayHeader = ep.absData.Title
	}

	out.WriteString(tuiTitleStyle.Render("  " + displayName(displayHeader)))
	out.WriteByte('\n')

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	out.WriteByte('\n')

	// Status Badges Row
	var badges []string
	if ep.hasAdsRemoved {
		badges = append(badges, tuiBadgeAdFree.Render("✓ Removed"))
	} else {
		badges = append(badges, tuiBadgeHasAds.Render("✗ Not removed"))
	}

	if inQueue {
		badges = append(badges, tuiBadgeQueued.Render("In queue"))
	} else {
		badges = append(badges, tuiDimStyle.Render("Not queued"))
	}

	if ep.absData != nil {
		if ep.absData.EpisodeType != "" {
			badges = append(badges, tuiBadgeType.Render(strings.ToUpper(ep.absData.EpisodeType)))
		}
		if ep.absData.Season != "" && ep.absData.Episode != "" {
			badges = append(badges, tuiBadgeCount.Render(fmt.Sprintf("S%sE%s", ep.absData.Season, ep.absData.Episode)))
		} else if ep.absData.Episode != "" {
			badges = append(badges, tuiBadgeCount.Render(fmt.Sprintf("Ep %s", ep.absData.Episode)))
		}
	}

	out.WriteString("  Status: " + strings.Join(badges, "  ") + "\n\n")

	// 1. Audio & File Details Card
	out.WriteString(tuiSectionTitle.Render("  Audio & File Information"))
	out.WriteByte('\n')
	out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	out.WriteByte('\n')

	out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("File:"), displayName(ep.filename)))
	out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Path:"), displayName(pod.name+"/"+ep.filename)))
	out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Size:"), formatFileSize(ep.fileSize)))

	dur := ep.duration
	if dur <= 0 && m.bk != nil && m.bk.GetDuration != nil {
		dur = m.bk.GetDuration(ep.path)
		ep.duration = dur
		ep.durationDone = true
	}
	if dur > 0 {
		out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Duration:"), formatDurationShort(dur)))
	}

	if ep.absData != nil && ep.absData.AudioFile != nil {
		af := ep.absData.AudioFile
		if af.Codec != "" {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Codec:"), af.Codec))
		}
		if af.BitRate > 0 {
			out.WriteString(fmt.Sprintf("  %s %d kbps\n", tuiLabelStyle.Render("Bitrate:"), af.BitRate/1000))
		}
		if af.ChannelLayout != "" {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Channels:"), af.ChannelLayout))
		}
	}

	// 2. ABS Rich Metadata Section
	if ep.absData != nil {
		out.WriteByte('\n')
		out.WriteString(tuiSectionTitle.Render("  ABS Information"))
		out.WriteByte('\n')
		out.WriteString(tuiDividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
		out.WriteByte('\n')

		if ep.absData.Title != "" {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Title:"), ep.absData.Title))
		}
		if ep.absData.Subtitle != "" {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Subtitle:"), ep.absData.Subtitle))
		}
		if ep.absData.PubDate != "" {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Published:"), formatABSDate(ep.absData.PubDate)))
		} else if ep.absData.PublishedAt > 0 {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Published:"), formatTimestamp(ep.absData.PublishedAt)))
		}
		if ep.absData.Duration > 0 {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("ABS Duration:"), formatDurationShort(ep.absData.Duration)))
		}
		if ep.absData.Size > 0 {
			out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("ABS Size:"), formatFileSize(ep.absData.Size)))
		}

		if pod.absData != nil {
			meta := pod.absData.Media.Metadata
			if meta.Author != "" {
				out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Author:"), meta.Author))
			}
			if len(meta.Genres) > 0 {
				out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Genres:"), strings.Join(meta.Genres, ", ")))
			}
			if meta.FeedURL != "" {
				out.WriteString(fmt.Sprintf("  %s %s\n", tuiLabelStyle.Render("Feed URL:"), meta.FeedURL))
			}
		}

		if ep.absData.Description != "" {
			out.WriteByte('\n')
			out.WriteString(tuiLabelStyle.Render("  Description:\n"))
			desc := stripHTML(ep.absData.Description)
			descLines := splitText(desc, max(20, m.width-8))
			for _, line := range descLines {
				out.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
	} else {
		out.WriteByte('\n')
		out.WriteString(tuiDimStyle.Render("  No ABS data available\n"))
	}

	// 3. Kitty Graphics Protocol Image Display (Cover Art) - Left side
	if isKittySupported() && m.showCover {
		coverPath := findCoverImage(pod.dir)
		if coverPath != "" {
			imgEsc, err := encodeKittyGraphicsFile(coverPath, 30, 0)
			if err == nil && imgEsc != "" {
				out.WriteString(tuiLabelStyle.Render("  Cover Art:\n"))
				out.WriteString(imgEsc + "\n\n")
			}
		}
	}

	out.WriteByte('\n')
	helpText := "R - Toggle queue  Esc - Back"
	if isKittySupported() {
		helpText += "  I - Toggle cover"
	}
	if m.showHelp {
		helpText += "  B hide"
	} else {
		helpText = ""
	}
	if helpText != "" {
		out.WriteString(tuiHelpStyle.Render("  " + helpText))
		out.WriteByte('\n')
	}

	return out.String()
}
