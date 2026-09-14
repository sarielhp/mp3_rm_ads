package podcast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"pod/pkg/util"
)

func (q *DownloadQueue) FlushPodcast(id, dir string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	lock, err := util.AcquireFileLock(q.filePath)
	if err != nil {
		return err
	}
	if lock == nil {
		return fmt.Errorf("download queue is busy")
	}
	defer lock.Release()
	data, err := os.ReadFile(q.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var persist DownloadQueuePersist
	if err := json.Unmarshal(data, &persist); err != nil {
		return err
	}
	var remaining []DownloadQueueItem
	for _, item := range persist.Items {
		matches := item.PodcastID == id || (item.PodcastDir != "" && filepath.Clean(item.PodcastDir) == filepath.Clean(dir))
		if matches && item.Status == "downloading" {
			return fmt.Errorf("podcast has an active download queue item; retry flush after it finishes")
		}
		if !matches {
			remaining = append(remaining, item)
		}
	}
	persist.Items = remaining
	return q.Save(&persist)
}
