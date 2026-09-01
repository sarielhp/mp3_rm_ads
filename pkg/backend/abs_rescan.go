package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func UpdateEpisodeInTx(tx *sql.Tx, episodeID string, diskDuration float64, hostPath string) bool {
	if tx == nil {
		return false
	}

	var audioFileStr string
	query := "SELECT audioFile FROM podcastEpisodes WHERE id = ?;"
	if err := tx.QueryRow(query, episodeID).Scan(&audioFileStr); err != nil {
		return false
	}

	var audioFileMap map[string]interface{}
	if err := json.Unmarshal([]byte(audioFileStr), &audioFileMap); err != nil {
		return false
	}

	audioFileMap["duration"] = diskDuration
	if metaRaw, ok := audioFileMap["metadata"]; ok && metaRaw != nil {
		if metaMap, ok := metaRaw.(map[string]interface{}); ok {
			if fi, err := os.Stat(hostPath); err == nil {
				metaMap["size"] = fi.Size()
				metaMap["mtimeMs"] = fi.ModTime().UnixNano() / 1e6
			}
		}
	}

	newJSON, err := json.Marshal(audioFileMap)
	if err != nil {
		return false
	}

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05.000 +00:00")
	updateQuery := "UPDATE podcastEpisodes SET audioFile = ?, updatedAt = ? WHERE id = ?;"
	_, err = tx.Exec(updateQuery, string(newJSON), nowStr, episodeID)
	if err != nil {
		return false
	}

	return true
}

func (c *AudiobookshelfBackend) Rescan(opts RescanOptions) (RescanResult, error) {
	podcastsDir := opts.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = c.PodcastsDir
	}

	podcasts, err := c.Podcasts()
	if err != nil {
		return RescanResult{}, fmt.Errorf("failed to fetch podcasts: %w", err)
	}

	var db *sql.DB
	if !opts.DryRun && c.DBPath != "" {
		if fi, err := os.Stat(c.DBPath); err == nil && !fi.IsDir() {
			db, _ = sql.Open("sqlite3", c.DBPath+"?_busy_timeout=5000")
			if db != nil {
				defer db.Close()
			}
		}
	}

	var totalRescanned, totalChecked int
	for _, p := range podcasts {
		if opts.PodcastID != "" && p.ID != opts.PodcastID && !strings.EqualFold(p.Media.Metadata.Title, opts.PodcastID) {
			continue
		}
		rCount, cCount := c.RescanPodcastEpisodes(p, opts.DryRun, db, podcastsDir, opts.Verbose, opts.Quiet)
		totalRescanned += rCount
		totalChecked += cCount
	}

	return RescanResult{
		RescanCount:  totalRescanned,
		CheckedCount: totalChecked,
	}, nil
}

func (c *AudiobookshelfBackend) RescanPodcastEpisodes(item Podcast, dryRun bool, db *sql.DB, podcastsDir string, verbose bool, quiet bool) (int, int) {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}
	itemID := item.ID
	episodes := item.Media.Episodes

	if len(episodes) == 0 {
		return 0, 0
	}

	numWorkers := 4
	if len(episodes) < numWorkers {
		numWorkers = len(episodes)
	}

	type durationJob struct {
		epIndex  int
		hostPath string
	}
	type durationResult struct {
		epIndex      int
		diskDuration float64
	}

	jobs := make(chan durationJob, len(episodes))
	results := make(chan durationResult, len(episodes))

	for w := 1; w <= numWorkers; w++ {
		go func() {
			for job := range jobs {
				dur := GetMP3DiskDuration(job.hostPath)
				results <- durationResult{
					epIndex:      job.epIndex,
					diskDuration: dur,
				}
			}
		}()
	}

	activeJobs := 0
	for idx, ep := range episodes {
		if ep.AudioFile == nil || ep.AudioFile.Metadata == nil {
			continue
		}

		path := ep.AudioFile.Metadata.Path
		hostPath := resolveHostPathLocal(path, podcastsDir)
		if hostPath == "" {
			continue
		}
		if fi, err := os.Stat(hostPath); err != nil || fi.IsDir() {
			continue
		}

		jobs <- durationJob{epIndex: idx, hostPath: hostPath}
		activeJobs++
	}
	close(jobs)

	diskDurations := make(map[int]float64)
	for i := 0; i < activeJobs; i++ {
		res := <-results
		diskDurations[res.epIndex] = res.diskDuration
	}
	close(results)

	var tx *sql.Tx
	var err error
	if db != nil && !dryRun {
		tx, err = db.Begin()
		if err == nil {
			defer tx.Rollback()
		} else {
			tx = nil
		}
	}

	var episodesToUpdate []Episode
	episodeDurations := make(map[string]float64)
	episodeHostPaths := make(map[string]string)
	checkedCount := 0
	rescanCount := 0

	for idx, ep := range episodes {
		if ep.AudioFile == nil || ep.AudioFile.Metadata == nil {
			continue
		}

		checkedCount++
		diskDuration, hasDur := diskDurations[idx]
		if !hasDur || diskDuration <= 0 {
			continue
		}

		dbDuration := ep.AudioFile.Duration
		if dbDuration <= 0 {
			dbDuration = ep.Duration
		}
		if dbDuration <= 0 {
			continue
		}

		if diskDuration < dbDuration-0.5 {
			episodesToUpdate = append(episodesToUpdate, ep)
			episodeDurations[ep.ID] = diskDuration
			path := ep.AudioFile.Metadata.Path
			episodeHostPaths[ep.ID] = resolveHostPathLocal(path, podcastsDir)
		}
	}

	if len(episodesToUpdate) > 0 {
		for _, ep := range episodesToUpdate {
			rescanCount++
			diskDuration := episodeDurations[ep.ID]
			if tx != nil {
				UpdateEpisodeInTx(tx, ep.ID, diskDuration, episodeHostPaths[ep.ID])
			}
		}
	}
	if tx != nil {
		tx.Commit()
	}

	if rescanCount > 0 && !dryRun {
		_, _ = c.Request(fmt.Sprintf("/api/items/%s/scan", itemID), "POST", nil)
	}

	return rescanCount, checkedCount
}

func (c *AudiobookshelfBackend) SyncDuration(filePath string, duration float64) error {
	if c.Host == "" {
		return nil
	}
	items, err := c.Podcasts()
	if err != nil {
		return err
	}

	parentDir := filepath.Base(filepath.Dir(filePath))
	for _, item := range items {
		title := item.Media.Metadata.Title
		if strings.EqualFold(title, parentDir) || strings.EqualFold(item.ID, parentDir) {
			_, err := c.Request(fmt.Sprintf("/api/items/%s/scan", item.ID), "POST", nil)
			return err
		}
	}
	return nil
}
