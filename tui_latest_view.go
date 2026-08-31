package main

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type tuiLatestItem struct {
	podIdx      int
	epIdx       int
	podcastName string
	podcastDir  string
	podcastID   string
	episode     tuiEpisode
}

func buildLatestEpisodesList(podcasts []tuiPodcast) []tuiLatestItem {
	var list []tuiLatestItem
	for pIdx, pod := range podcasts {
		podID := ""
		if pod.absData != nil {
			podID = pod.absData.ID
		}
		for eIdx, ep := range pod.episodes {
			list = append(list, tuiLatestItem{
				podIdx:      pIdx,
				epIdx:       eIdx,
				podcastName: pod.name,
				podcastDir:  pod.dir,
				podcastID:   podID,
				episode:     ep,
			})
		}
	}

	sort.SliceStable(list, func(i, j int) bool {
		d1 := list[i].episode.displayDate()
		d2 := list[j].episode.displayDate()
		if d1.Equal(d2) {
			return list[i].episode.displayTitle() < list[j].episode.displayTitle()
		}
		return d1.After(d2)
	})

	return list
}

func (m *tuiModel) filteredLatestEpisodes() []tuiLatestItem {
	all := buildLatestEpisodesList(m.podcasts)
	if m.searchQuery == "" {
		return all
	}
	q := strings.ToLower(m.searchQuery)
	var filtered []tuiLatestItem
	for _, item := range all {
		t := strings.ToLower(item.episode.displayTitle())
		p := strings.ToLower(item.podcastName)
		f := strings.ToLower(item.episode.filename)
		if strings.Contains(t, q) || strings.Contains(p, q) || strings.Contains(f, q) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *tuiModel) drawLatestEpisodesScreen() string {
	out := &strings.Builder{}

	titleBanner := tuiHeaderBanner.Render(" LATEST EPISODES (L) ")
	out.WriteString(fmt.Sprintf("  %s\n", titleBanner))

	items := m.filteredLatestEpisodes()
	statPill := tuiStatStyle.Render(fmt.Sprintf("  %d latest episodes across %d podcasts", len(items), len(m.podcasts)))
	out.WriteString(statPill + "\n")

	dividerWidth := max(0, m.width-4)
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")

	overhead := 8
	if globalPlayer.Current != nil {
		overhead = 10
	}
	maxVis := max(3, m.height-overhead)

	if len(items) == 0 {
		out.WriteString("\n  " + tuiDimStyle.Render("No episodes found.") + "\n\n")
		out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")
		out.WriteString(tuiDimStyle.Render("  Esc/q Back │ / Search") + "\n")
		return out.String()
	}

	if m.latestIdx >= len(items) {
		m.latestIdx = len(items) - 1
	}
	if m.latestIdx < 0 {
		m.latestIdx = 0
	}
	if m.latestIdx < m.latestScroll {
		m.latestScroll = m.latestIdx
	}
	if m.latestIdx >= m.latestScroll+maxVis {
		m.latestScroll = m.latestIdx - maxVis + 1
	}
	if m.latestScroll > max(0, len(items)-maxVis) {
		m.latestScroll = max(0, len(items)-maxVis)
	}
	if m.latestScroll < 0 {
		m.latestScroll = 0
	}

	start := m.latestScroll
	end := min(len(items), start+maxVis)

	for i := start; i < end; i++ {
		item := items[i]
		ep := item.episode
		displayNameStr := ep.displayTitle()
		d := ep.displayDate()
		dateStr := strings.Repeat(" ", 10)
		if !d.IsZero() {
			dateStr = d.Format("2006-01-02")
		}

		epGUID := ""
		if ep.absData != nil {
			epGUID = ep.absData.ID
		}
		inQueue := IsEpisodeInDownloadQueue(epGUID, "", ep.displayTitle()) || IsEpisodeInDownloadQueue(epGUID, "", ep.filename)

		podTag := fmt.Sprintf("[%s]", truncate(displayName(item.podcastName), 18))
		availWidth := m.width - 2
		isSelected := (i == m.latestIdx)

		badgeStr := ""
		if inQueue {
			badgeStr = " [⏳ Queued]"
		} else if ep.hasAdsRemoved {
			badgeStr = " [✓ Ad-Free]"
		}

		txStr := ""
		if ep.hasTranscript {
			txStr = " [TX]"
		}

		durStr := ""
		if ep.duration > 0 {
			durStr = " (" + formatPlayerTime(ep.duration) + ")"
		}

		rowContent := fmt.Sprintf("%s %-20s %s%s%s%s", dateStr, podTag, displayNameStr, durStr, txStr, badgeStr)
		truncRow := truncate(rowContent, availWidth-4)

		if isSelected {
			fullPad := max(0, availWidth-visibleRuneCount(truncRow)-2)
			out.WriteString("  " + tuiSelectedStyle.Render(truncRow+strings.Repeat(" ", fullPad)) + "\n")
		} else {
			pTagRender := tuiPurpleStyle.Render(fmt.Sprintf("%-20s", podTag))
			dRender := tuiSubtextStyle.Render(dateStr)
			badgeRender := ""
			if inQueue {
				badgeRender = " " + tuiBadgeQueued.Render("[⏳ Queued]")
			} else if ep.hasAdsRemoved {
				badgeRender = " " + tuiGreenStyle.Render("[✓ Ad-Free]")
			}
			txRender := ""
			if ep.hasTranscript {
				txRender = " " + tuiCyanStyle.Render("[TX]")
			}
			durRender := ""
			if ep.duration > 0 {
				durRender = tuiDimStyle.Render(durStr)
			}
			titleRender := truncate(displayName(displayNameStr), max(10, availWidth-42-visibleRuneCount(durStr)-visibleRuneCount(badgeStr)-visibleRuneCount(txStr)))
			out.WriteString(fmt.Sprintf("  %s %s %s%s%s%s\n", dRender, pTagRender, titleRender, durRender, txRender, badgeRender))
		}
	}

	helpText := "↑/↓ navigate │ Enter details │ D download │ p play │ r ad-queue │ / search │ Esc/q back"
	if m.searchMode {
		helpText = fmt.Sprintf("Search: %s█ (Enter: Apply, Esc: Cancel)", m.searchQuery)
	} else if len(items) > maxVis {
		pct := int(float64(m.latestIdx+1) / float64(len(items)) * 100)
		helpText += fmt.Sprintf(" │ [%d/%d (%d%%)]", m.latestIdx+1, len(items), pct)
	}
	out.WriteString(tuiDividerStyle.Render("  "+strings.Repeat("─", dividerWidth)) + "\n")
	if m.searchMode {
		out.WriteString(tuiSearchStyle.Render("  "+helpText) + "\n")
	} else {
		out.WriteString(tuiDimStyle.Render("  "+helpText) + "\n")
	}

	return out.String()
}

func (m *tuiModel) enqueueDownloadForLatestItem(item tuiLatestItem) {
	ep := item.episode
	epGUID := ""
	pubDate := ""
	var pubAt int64
	if ep.absData != nil {
		epGUID = ep.absData.ID
		pubDate = ep.absData.PubDate
		pubAt = parseABSEpisodePublishedAt(ep.absData)
	}
	if pubAt == 0 && ep.publishedAt > 0 {
		pubAt = ep.publishedAt
	}

	dlItem := DownloadQueueItem{
		PodcastTitle: item.podcastName,
		PodcastDir:   item.podcastDir,
		PodcastID:    item.podcastID,
		EpisodeTitle: ep.displayTitle(),
		GUID:         epGUID,
		PubDate:      pubDate,
		PublishedAt:  pubAt,
		DurationSec:  ep.duration,
	}

	ok, reason := EnqueueDownload(dlItem, m.podcasts)
	if ok {
		m.showToast("Enqueued for download: "+ep.displayTitle(), ToastSuccess)
		var absCli *ABSClient
		if m.podcastsDir != "" {
			cfg := loadConfig()
			if cfg.AudiobookshelfURL != "" {
				absCli = NewABSClient(cfg.AudiobookshelfURL, cfg.AudiobookshelfToken)
			}
		}
		TriggerDownloadQueueWorker(absCli)
	} else if reason == "already_queued" {
		m.showToast("Already in download queue", ToastWarning)
	} else if reason == "already_downloaded" {
		m.showToast("Episode already downloaded", ToastInfo)
	} else {
		m.showToast("Failed to enqueue download", ToastError)
	}
}

func (m *tuiModel) handleLatestViewKey(s string) (tea.Model, tea.Cmd) {
	items := m.filteredLatestEpisodes()
	switch s {
	case "up", "k":
		if m.latestIdx > 0 {
			m.latestIdx--
		}
		if m.latestIdx < m.latestScroll {
			m.latestScroll = m.latestIdx
		}
	case "down", "j":
		if m.latestIdx < len(items)-1 {
			m.latestIdx++
		}
		maxVis := m.visibleLines(4)
		if m.latestIdx >= m.latestScroll+maxVis {
			m.latestScroll = m.latestIdx - maxVis + 1
		}
	case "enter":
		if len(items) > 0 && m.latestIdx < len(items) {
			item := items[m.latestIdx]
			m.podIdx = item.podIdx
			m.epIdx = item.epIdx
			m.prevScreen = screenLatestEpisodes
			m.screen = screenEpisodeDetail
			m.descScroll = 0
		}
	case "d", "D":
		if len(items) > 0 && m.latestIdx < len(items) {
			m.enqueueDownloadForLatestItem(items[m.latestIdx])
		}
	case "p", "P":
		if len(items) > 0 && m.latestIdx < len(items) {
			item := items[m.latestIdx]
			m.podIdx = item.podIdx
			m.epIdx = item.epIdx
			m.playSelectedEpisode()
		}
	case "r", "R":
		if len(items) > 0 && m.latestIdx < len(items) {
			item := items[m.latestIdx]
			m.podIdx = item.podIdx
			m.epIdx = item.epIdx
			m.handleQueueToggle()
		}
	case "esc", "q", "Q", "l", "L":
		if m.prevScreen != screenLatestEpisodes && m.prevScreen != screenDownloadQueue && m.prevScreen != 0 {
			m.screen = m.prevScreen
		} else {
			m.screen = screenPodcasts
		}
	}
	return m, nil
}
