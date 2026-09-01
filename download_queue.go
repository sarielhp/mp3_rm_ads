package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadQueueItem struct {
	ID           string       `json:"id"`
	PodcastTitle string       `json:"podcast_title"`
	PodcastDir   string       `json:"podcast_dir,omitempty"`
	PodcastID    string       `json:"podcast_id,omitempty"`
	EpisodeTitle string       `json:"episode_title"`
	GUID         string       `json:"guid,omitempty"`
	EnclosureURL string       `json:"enclosure_url,omitempty"`
	PubDate      string       `json:"pub_date,omitempty"`
	PublishedAt  int64        `json:"published_at,omitempty"`
	DurationSec  float64      `json:"duration_sec,omitempty"`
	Status       string       `json:"status"`
	Error        string       `json:"error,omitempty"`
	AddedAt      time.Time    `json:"added_at"`
	EpisodeObj   *FeedEpisode `json:"episode_obj,omitempty"`
}

type DownloadQueuePersist struct {
	Items     []DownloadQueueItem `json:"items"`
	UpdatedAt time.Time           `json:"updated_at,omitempty"`
}

var (
	testDownloadQueuePath string
	downloadQueueMutex    syncMutex
	downloadWorkerRunning bool
	downloadWorkerMu      syncMutex
)

func getDownloadQueueFilePath() string {
	if testDownloadQueuePath != "" {
		return testDownloadQueuePath
	}
	return filepath.Join(configDir(), "download_queue.json")
}

func loadDownloadQueue() *DownloadQueuePersist {
	path := getDownloadQueueFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &DownloadQueuePersist{Items: []DownloadQueueItem{}}
	}
	var q DownloadQueuePersist
	if err := json.Unmarshal(data, &q); err != nil {
		return &DownloadQueuePersist{Items: []DownloadQueueItem{}}
	}
	if q.Items == nil {
		q.Items = []DownloadQueueItem{}
	}
	return &q
}

func saveDownloadQueue(q *DownloadQueuePersist) error {
	path := getDownloadQueueFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	q.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0644)
}

func matchEpisodeDeduplication(guid1, enc1, title1, guid2, enc2, title2 string) bool {
	if guid1 != "" && guid2 != "" && strings.EqualFold(strings.TrimSpace(guid1), strings.TrimSpace(guid2)) {
		return true
	}
	if enc1 != "" && enc2 != "" && strings.EqualFold(strings.TrimSpace(enc1), strings.TrimSpace(enc2)) {
		return true
	}
	t1 := normalizeEpisodeTitle(title1)
	t2 := normalizeEpisodeTitle(title2)
	if t1 != "" && t2 != "" && t1 == t2 {
		return true
	}
	return false
}

func isItemInDownloadQueue(item DownloadQueueItem, q *DownloadQueuePersist) bool {
	for _, existing := range q.Items {
		if existing.Status == "completed" {
			continue
		}
		if matchEpisodeDeduplication(item.GUID, item.EnclosureURL, item.EpisodeTitle, existing.GUID, existing.EnclosureURL, existing.EpisodeTitle) {
			return true
		}
	}
	return false
}

func isItemDownloaded(item DownloadQueueItem, pods []tuiPodcast) bool {
	for _, p := range pods {
		if item.PodcastTitle != "" && p.name != "" && !strings.EqualFold(strings.TrimSpace(item.PodcastTitle), strings.TrimSpace(p.name)) {
			if item.PodcastDir != "" && p.dir != "" && item.PodcastDir != p.dir {
				continue
			}
		}
		for _, ep := range p.episodes {
			epGUID := ""
			if ep.absData != nil {
				epGUID = ep.absData.ID
			}
			if matchEpisodeDeduplication(item.GUID, item.EnclosureURL, item.EpisodeTitle, epGUID, "", ep.displayTitle()) {
				return true
			}
			if matchEpisodeDeduplication(item.GUID, item.EnclosureURL, item.EpisodeTitle, "", "", ep.filename) {
				return true
			}
		}
	}
	return false
}

func IsEpisodeInDownloadQueue(guid, encURL, title string) bool {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()
	q := loadDownloadQueue()
	for _, existing := range q.Items {
		if existing.Status == "completed" {
			continue
		}
		if matchEpisodeDeduplication(guid, encURL, title, existing.GUID, existing.EnclosureURL, existing.EpisodeTitle) {
			return true
		}
	}
	return false
}

func EnqueueDownload(item DownloadQueueItem, pods []tuiPodcast) (bool, string) {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()

	q := loadDownloadQueue()
	if isItemInDownloadQueue(item, q) {
		return false, "already_queued"
	}

	if item.ID == "" {
		item.ID = fmt.Sprintf("dl-%d", time.Now().UnixNano())
	}
	item.Status = "queued"
	item.AddedAt = time.Now().UTC()

	q.Items = append(q.Items, item)
	if err := saveDownloadQueue(q); err != nil {
		return false, "save_error"
	}
	return true, "queued"
}

func RemoveDownloadQueueItem(id string) bool {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()

	q := loadDownloadQueue()
	var updated []DownloadQueueItem
	found := false
	for _, item := range q.Items {
		if item.ID == id {
			found = true
			continue
		}
		updated = append(updated, item)
	}
	if found {
		q.Items = updated
		_ = saveDownloadQueue(q)
	}
	return found
}

func RemoveDownloadQueueItemAt(index int) bool {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()

	q := loadDownloadQueue()
	if index < 0 || index >= len(q.Items) {
		return false
	}
	q.Items = append(q.Items[:index], q.Items[index+1:]...)
	_ = saveDownloadQueue(q)
	return true
}

func ClearDownloadQueue() {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()

	q := &DownloadQueuePersist{Items: []DownloadQueueItem{}}
	_ = saveDownloadQueue(q)
}

func GetDownloadQueueItems() []DownloadQueueItem {
	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()

	q := loadDownloadQueue()
	return q.Items
}

func ProcessNextDownloadQueueItem(client *ABSClient) (bool, error) {
	downloadQueueMutex.Lock()
	q := loadDownloadQueue()
	targetIdx := -1
	for i, item := range q.Items {
		if item.Status == "queued" {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		downloadQueueMutex.Unlock()
		return false, nil
	}

	item := &q.Items[targetIdx]
	item.Status = "downloading"
	_ = saveDownloadQueue(q)
	downloadQueueMutex.Unlock()

	if client == nil {
		cfg := loadConfig()
		client = NewABSClient(cfg.AudiobookshelfURL, cfg.AudiobookshelfToken)
	}

	podcastID := item.PodcastID
	if podcastID == "" && client != nil {
		if pods, err := client.PodcastItems(); err == nil {
			for _, p := range pods {
				if strings.EqualFold(strings.TrimSpace(p.Media.Metadata.Title), strings.TrimSpace(item.PodcastTitle)) {
					podcastID = p.ID
					break
				}
			}
		}
	}

	var dlErr error
	if podcastID == "" {
		dlErr = fmt.Errorf("podcast ID not found for '%s'", item.PodcastTitle)
	} else {
		feedEp := FeedEpisode{
			Title:           item.EpisodeTitle,
			GUID:            item.GUID,
			EnclosureURL:    item.EnclosureURL,
			PubDate:         item.PubDate,
			PublishedAt:     item.PublishedAt,
			DurationSeconds: item.DurationSec,
		}
		if item.EnclosureURL != "" {
			feedEp.Enclosure = &FeedEnclosure{URL: item.EnclosureURL}
		}
		if item.EpisodeObj != nil {
			feedEp = *item.EpisodeObj
		}
		dlErr = client.DownloadEpisodes(podcastID, []FeedEpisode{feedEp})
	}

	downloadQueueMutex.Lock()
	defer downloadQueueMutex.Unlock()
	q = loadDownloadQueue()
	for i := range q.Items {
		if q.Items[i].ID == item.ID {
			if dlErr != nil {
				q.Items[i].Status = "failed"
				q.Items[i].Error = dlErr.Error()
			} else {
				q.Items[i].Status = "completed"
			}
			break
		}
	}
	_ = saveDownloadQueue(q)
	return true, dlErr
}

func TriggerDownloadQueueWorker(client *ABSClient) {
	downloadWorkerMu.Lock()
	if downloadWorkerRunning {
		downloadWorkerMu.Unlock()
		return
	}
	downloadWorkerRunning = true
	downloadWorkerMu.Unlock()

	go func() {
		defer func() {
			downloadWorkerMu.Lock()
			downloadWorkerRunning = false
			downloadWorkerMu.Unlock()
		}()

		for {
			processed, _ := ProcessNextDownloadQueueItem(client)
			if !processed {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

// WaitDownloadWorkerForTest blocks until the download queue worker finishes.
func WaitDownloadWorkerForTest() {
	for {
		downloadWorkerMu.Lock()
		running := downloadWorkerRunning
		downloadWorkerMu.Unlock()
		if !running {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
}
