package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadQueuePersistence(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	q := &DownloadQueuePersist{
		Items: []DownloadQueueItem{
			{
				ID:           "item-1",
				PodcastTitle: "Tech News",
				EpisodeTitle: "Episode 101",
				GUID:         "guid-101",
				EnclosureURL: "https://example.com/ep101.mp3",
				Status:       "queued",
				AddedAt:      time.Now().UTC(),
			},
			{
				ID:           "item-2",
				PodcastTitle: "Daily Pod",
				EpisodeTitle: "Morning Edition",
				GUID:         "guid-102",
				EnclosureURL: "https://example.com/ep102.mp3",
				Status:       "queued",
				AddedAt:      time.Now().UTC(),
			},
		},
	}

	if err := saveDownloadQueue(q); err != nil {
		t.Fatalf("saveDownloadQueue failed: %v", err)
	}

	loaded := loadDownloadQueue()
	if len(loaded.Items) != 2 {
		t.Fatalf("expected 2 items in loaded queue, got %d", len(loaded.Items))
	}
	if loaded.Items[0].ID != "item-1" || loaded.Items[0].EpisodeTitle != "Episode 101" {
		t.Errorf("unexpected first item: %+v", loaded.Items[0])
	}
	if loaded.Items[1].ID != "item-2" || loaded.Items[1].EpisodeTitle != "Morning Edition" {
		t.Errorf("unexpected second item: %+v", loaded.Items[1])
	}
}

func TestDownloadQueueStrictDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	pods := []tuiPodcast{
		{
			name: "Existing Show",
			dir:  filepath.Join(tempDir, "show"),
			episodes: []tuiEpisode{
				{
					filename: "existing_ep.mp3",
					title:    "Already Downloaded Episode",
					absData:  &absEpisode{ID: "guid-downloaded-1"},
				},
			},
		},
	}

	item1 := DownloadQueueItem{
		PodcastTitle: "Existing Show",
		EpisodeTitle: "New Unique Episode",
		GUID:         "guid-unique-1",
		EnclosureURL: "https://example.com/unique1.mp3",
	}
	ok, reason := EnqueueDownload(item1, pods)
	if !ok || reason != "queued" {
		t.Errorf("expected item1 to be queued, got ok=%v, reason=%s", ok, reason)
	}

	itemDupGUID := DownloadQueueItem{
		PodcastTitle: "Existing Show",
		EpisodeTitle: "Different Title",
		GUID:         "guid-unique-1",
		EnclosureURL: "https://example.com/different.mp3",
	}
	ok, reason = EnqueueDownload(itemDupGUID, pods)
	if ok || reason != "already_queued" {
		t.Errorf("expected duplicate GUID to fail enqueue, got ok=%v, reason=%s", ok, reason)
	}

	itemDupURL := DownloadQueueItem{
		PodcastTitle: "Existing Show",
		EpisodeTitle: "Different Title Again",
		GUID:         "guid-unique-2",
		EnclosureURL: "https://example.com/unique1.mp3",
	}
	ok, reason = EnqueueDownload(itemDupURL, pods)
	if ok || reason != "already_queued" {
		t.Errorf("expected duplicate EnclosureURL to fail enqueue, got ok=%v, reason=%s", ok, reason)
	}

	itemDupTitle := DownloadQueueItem{
		PodcastTitle: "Existing Show",
		EpisodeTitle: "New Unique Episode",
		GUID:         "guid-unique-3",
		EnclosureURL: "https://example.com/unique3.mp3",
	}
	ok, reason = EnqueueDownload(itemDupTitle, pods)
	if ok || reason != "already_queued" {
		t.Errorf("expected duplicate Title to fail enqueue, got ok=%v, reason=%s", ok, reason)
	}

	itemDownloaded := DownloadQueueItem{
		PodcastTitle: "Existing Show",
		EpisodeTitle: "Already Downloaded Episode",
		GUID:         "guid-downloaded-1",
		EnclosureURL: "https://example.com/down.mp3",
	}
	if !isItemDownloaded(itemDownloaded, pods) {
		t.Errorf("expected isItemDownloaded to return true for existing episode")
	}
}

func setupMockABSDownloadServer(downloadedItems *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/podcasts" && r.Method == "GET" {
			res := struct {
				Podcasts []PodcastItem `json:"podcasts"`
			}{
				Podcasts: []PodcastItem{
					{
						ID:    "pod-123",
						Media: PodcastMedia{Metadata: PodcastMetadata{Title: "Test Podcast"}},
					},
				},
			}
			data, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
		if r.URL.Path == "/api/podcasts/pod-123/download-episodes" && r.Method == "POST" {
			var eps []FeedEpisode
			_ = json.NewDecoder(r.Body).Decode(&eps)
			for _, ep := range eps {
				*downloadedItems = append(*downloadedItems, ep.Title)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestDownloadQueueFIFOOrderAndWorker(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	var downloadedItems []string
	server := setupMockABSDownloadServer(&downloadedItems)
	defer server.Close()

	client := NewABSClient(server.URL, "test-token")

	titles := []string{"First Enqueued Episode", "Second Enqueued Episode", "Third Enqueued Episode"}
	guids := []string{"guid-first", "guid-second", "guid-third"}
	for i := range titles {
		EnqueueDownload(DownloadQueueItem{
			PodcastTitle: "Test Podcast",
			PodcastID:    "pod-123",
			EpisodeTitle: titles[i],
			GUID:         guids[i],
		}, nil)
	}

	items := GetDownloadQueueItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 items in queue, got %d", len(items))
	}
	for i, title := range titles {
		if items[i].EpisodeTitle != title {
			t.Errorf("items not in FIFO order: %+v", items)
		}
	}

	for i, title := range titles {
		processed, err := ProcessNextDownloadQueueItem(client)
		if !processed || err != nil {
			t.Errorf("ProcessNextDownloadQueueItem failed for item %d: %v", i, err)
		}
		if len(downloadedItems) != i+1 || downloadedItems[i] != title {
			t.Errorf("expected item %d downloaded in order, got: %v", i, downloadedItems)
		}
	}

	processed, err := ProcessNextDownloadQueueItem(client)
	if processed || err != nil {
		t.Errorf("expected no more items to process, got processed=%v, err=%v", processed, err)
	}
}

func TestDownloadQueueRemoveAndClear(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()

	itemA := DownloadQueueItem{ID: "id-a", EpisodeTitle: "Ep A"}
	itemB := DownloadQueueItem{ID: "id-b", EpisodeTitle: "Ep B"}
	itemC := DownloadQueueItem{ID: "id-c", EpisodeTitle: "Ep C"}

	EnqueueDownload(itemA, nil)
	EnqueueDownload(itemB, nil)
	EnqueueDownload(itemC, nil)

	if len(GetDownloadQueueItems()) != 3 {
		t.Fatalf("expected 3 items, got %d", len(GetDownloadQueueItems()))
	}

	if !RemoveDownloadQueueItem("id-b") {
		t.Errorf("expected RemoveDownloadQueueItem(id-b) to succeed")
	}

	items := GetDownloadQueueItems()
	if len(items) != 2 || items[0].ID != "id-a" || items[1].ID != "id-c" {
		t.Errorf("unexpected items after remove: %+v", items)
	}

	if !RemoveDownloadQueueItemAt(0) {
		t.Errorf("expected RemoveDownloadQueueItemAt(0) to succeed")
	}

	items = GetDownloadQueueItems()
	if len(items) != 1 || items[0].ID != "id-c" {
		t.Errorf("unexpected items after remove at 0: %+v", items)
	}

	ClearDownloadQueue()
	if len(GetDownloadQueueItems()) != 0 {
		t.Errorf("expected 0 items after ClearDownloadQueue, got %d", len(GetDownloadQueueItems()))
	}
}

func TestDownloadQueue_CrossProcessClaimingAndLease(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()
	EnqueueDownload(DownloadQueueItem{
		ID:           "claim-1",
		PodcastTitle: "Show",
		EpisodeTitle: "Ep 1",
	}, nil)

	item, claimed, err := claimDownloadQueueItem()
	if err != nil || !claimed {
		t.Fatalf("expected item claimed, got claimed=%v, err=%v", claimed, err)
	}
	if item.OwnerPID != os.Getpid() || item.Status != "downloading" {
		t.Errorf("expected OwnerPID=%d, status=downloading, got %+v", os.Getpid(), item)
	}

	resetReconcileDownloadQueueOnceForTest()
	q := loadDownloadQueue()
	if len(q.Items) != 1 || q.Items[0].Status != "downloading" {
		t.Errorf("expected active owner item to stay downloading, got %+v", q.Items)
	}

	q.Items[0].OwnerPID = 999999
	_ = saveDownloadQueue(q)
	resetReconcileDownloadQueueOnceForTest()
	q = loadDownloadQueue()
	if len(q.Items) != 1 || q.Items[0].Status != "queued" {
		t.Errorf("expected dead PID item to be reset to queued, got %+v", q.Items)
	}
}

func TestDownloadQueue_WorkerPendingWakeup(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()
	var downloadedItems []string
	server := setupMockABSDownloadServer(&downloadedItems)
	defer server.Close()

	client := NewABSClient(server.URL, "test-token")

	downloadWorkerMu.Lock()
	downloadWorkerRunning = true
	downloadWorkerMu.Unlock()

	EnqueueDownload(DownloadQueueItem{
		ID:           "wake-1",
		PodcastTitle: "Test Podcast",
		PodcastID:    "pod-123",
		EpisodeTitle: "Wakeup Episode",
	}, nil)

	TriggerDownloadQueueWorker(client)

	downloadWorkerMu.Lock()
	if !downloadWorkerPending {
		t.Errorf("expected downloadWorkerPending to be true when triggered during running")
	}
	downloadWorkerMu.Unlock()

	go runDownloadQueueWorkerLoop(client)
	WaitDownloadWorkerForTest()

	items := GetDownloadQueueItems()
	if len(items) != 1 || items[0].Status != "completed" {
		t.Errorf("expected item to be processed and completed, got %+v", items)
	}
}

func TestProcessNextDownloadQueueItem_NilClientSafe(t *testing.T) {
	tempDir := t.TempDir()
	queueFile := filepath.Join(tempDir, "download_queue.json")
	testDownloadQueuePath = queueFile
	defer func() { WaitDownloadWorkerForTest(); testDownloadQueuePath = "" }()

	ClearDownloadQueue()
	origCfgPath := testConfigPath
	testConfigPath = filepath.Join(t.TempDir(), "config.json")
	defer func() { testConfigPath = origCfgPath }()

	EnqueueDownload(DownloadQueueItem{
		ID:           "nil-cli-1",
		PodcastTitle: "Any Show",
		PodcastID:    "pod-id-exists",
		EpisodeTitle: "Any Ep",
	}, nil)

	processed, err := ProcessNextDownloadQueueItem(nil)
	if !processed {
		t.Fatalf("expected processed=true")
	}
	if err == nil {
		t.Fatalf("expected error for nil client, got nil")
	}

	items := GetDownloadQueueItems()
	if len(items) != 1 || items[0].Status != "failed" {
		t.Errorf("expected item status=failed, got %+v", items)
	}
}
