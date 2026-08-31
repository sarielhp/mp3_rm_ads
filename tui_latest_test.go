package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLatestEpisodesSortingAndBuilding(t *testing.T) {
	podcasts := []tuiPodcast{
		{
			name: "Show A",
			dir:  "/tmp/show_a",
			episodes: []tuiEpisode{
				{
					filename:    "a1.mp3",
					title:       "Episode A1",
					publishedAt: 1000000,
					duration:    1200,
				},
				{
					filename:    "a2.mp3",
					title:       "Episode A2",
					publishedAt: 3000000,
					duration:    1500,
				},
			},
		},
		{
			name: "Show B",
			dir:  "/tmp/show_b",
			episodes: []tuiEpisode{
				{
					filename:    "b1.mp3",
					title:       "Episode B1",
					publishedAt: 2000000,
					duration:    900,
				},
				{
					filename:    "b2.mp3",
					title:       "Episode B2",
					publishedAt: 4000000,
					duration:    1800,
				},
			},
		},
	}

	latest := buildLatestEpisodesList(podcasts)
	if len(latest) != 4 {
		t.Fatalf("expected 4 episodes in latest list, got %d", len(latest))
	}

	if latest[0].episode.title != "Episode B2" {
		t.Errorf("expected newest episode to be Episode B2 (4000000), got %s", latest[0].episode.title)
	}
	if latest[1].episode.title != "Episode A2" {
		t.Errorf("expected 2nd episode to be Episode A2 (3000000), got %s", latest[1].episode.title)
	}
	if latest[2].episode.title != "Episode B1" {
		t.Errorf("expected 3rd episode to be Episode B1 (2000000), got %s", latest[2].episode.title)
	}
	if latest[3].episode.title != "Episode A1" {
		t.Errorf("expected oldest episode to be Episode A1 (1000000), got %s", latest[3].episode.title)
	}
}

func TestLatestEpisodesNavigationAndDKey(t *testing.T) {
	tempDir := t.TempDir()
	testDownloadQueuePath = filepath.Join(tempDir, "download_queue.json")
	defer func() { testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	m := makeTestModel()
	m.width = 100
	m.height = 30
	m.screen = screenPodcasts

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.screen != screenLatestEpisodes {
		t.Fatalf("expected screen to switch to screenLatestEpisodes after pressing 'l', got %v", m.screen)
	}

	view := m.View()
	if !strings.Contains(view, "LATEST EPISODES") {
		t.Errorf("expected view to contain LATEST EPISODES banner, got:\n%s", view)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.toast == nil || !strings.Contains(m.toast.Message, "Enqueued for download") {
		t.Errorf("expected success toast after pressing 'd' in latest view, got %+v", m.toast)
	}

	queuedItems := GetDownloadQueueItems()
	if len(queuedItems) != 1 {
		t.Fatalf("expected 1 item in download queue, got %d", len(queuedItems))
	}

	viewAfterQueue := m.View()
	if !strings.Contains(viewAfterQueue, "[⏳ Queued]") {
		t.Errorf("expected [⏳ Queued] badge in latest view after enqueuing, got:\n%s", viewAfterQueue)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenEpisodeDetail {
		t.Fatalf("expected Enter to open screenEpisodeDetail, got %v", m.screen)
	}

	detailView := m.View()
	if !strings.Contains(detailView, "[⏳ Queued]") {
		t.Errorf("expected [⏳ Queued] badge in episode detail view, got:\n%s", detailView)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.screen != screenLatestEpisodes {
		t.Errorf("expected Esc from detail to return to screenLatestEpisodes, got %v", m.screen)
	}
}

func TestDKeyInEpisodeViews(t *testing.T) {
	tempDir := t.TempDir()
	testDownloadQueuePath = filepath.Join(tempDir, "download_queue.json")
	defer func() { testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	m := makeTestModel()
	m.width = 100
	m.height = 30

	m.screen = screenPodcastDetail
	m.podIdx = 0
	m.epIdx = 1

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if m.toast == nil || !strings.Contains(m.toast.Message, "Enqueued for download") {
		t.Errorf("expected toast feedback in screenPodcastDetail, got %+v", m.toast)
	}

	detailView := m.View()
	if !strings.Contains(detailView, "[⏳ Queued]") {
		t.Errorf("expected queued badge in episode list view, got:\n%s", detailView)
	}

	m.screen = screenEpisodeDetail
	m.epIdx = 0
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if m.toast == nil {
		t.Errorf("expected toast feedback in screenEpisodeDetail")
	}
}

func TestDownloadQueueViewAndKeyHandling(t *testing.T) {
	tempDir := t.TempDir()
	testDownloadQueuePath = filepath.Join(tempDir, "download_queue.json")
	defer func() { testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	EnqueueDownload(DownloadQueueItem{
		ID:           "test-ep-1",
		PodcastTitle: "Science Show",
		EpisodeTitle: "Mars Exploration",
		Status:       "queued",
		AddedAt:      time.Now().UTC(),
	}, nil)

	EnqueueDownload(DownloadQueueItem{
		ID:           "test-ep-2",
		PodcastTitle: "History Show",
		EpisodeTitle: "Ancient Rome",
		Status:       "queued",
		AddedAt:      time.Now().UTC(),
	}, nil)

	m := makeTestModel()
	m.width = 100
	m.height = 30
	m.screen = screenDownloadQueue

	view := m.View()
	if !strings.Contains(view, "DOWNLOAD QUEUE") || !strings.Contains(view, "Mars Exploration") || !strings.Contains(view, "Ancient Rome") {
		t.Errorf("expected download queue view with items, got:\n%s", view)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.toast == nil || !strings.Contains(m.toast.Message, "Removed from download queue") {
		t.Errorf("expected removal toast, got %+v", m.toast)
	}

	if len(GetDownloadQueueItems()) != 1 {
		t.Fatalf("expected 1 item left in queue, got %d", len(GetDownloadQueueItems()))
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if len(GetDownloadQueueItems()) != 0 {
		t.Errorf("expected 0 items after clear, got %d", len(GetDownloadQueueItems()))
	}
}
