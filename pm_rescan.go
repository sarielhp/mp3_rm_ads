package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tcolgate/mp3"
)

func getMP3DiskDurationNative(path string) float64 {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	d := mp3.NewDecoder(file)
	var duration float64
	var frame mp3.Frame
	var skipped int

	for {
		if err := d.Decode(&frame, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		duration += frame.Duration().Seconds()
	}

	return duration
}

func getMP3DiskDuration(path string) float64 {
	if path == "" || !fileExists(path) {
		return 0
	}

	if dur := getMP3DiskDurationNative(path); dur > 0 {
		return dur
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		s := strings.TrimSpace(out.String())
		if dur, err := strconv.ParseFloat(s, 64); err == nil && dur > 0 {
			return dur
		}
	}

	ctxMI, cancelMI := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelMI()

	cmdMI := exec.CommandContext(ctxMI, "mediainfo", "--Inform=General;%Duration%", path)
	var outMI bytes.Buffer
	cmdMI.Stdout = &outMI
	if err := cmdMI.Run(); err == nil {
		s := strings.TrimSpace(outMI.String())
		if durMS, err := strconv.ParseFloat(s, 64); err == nil && durMS > 0 {
			return durMS / 1000.0
		}
	}

	return 0
}

func updateEpisodeInTx(tx *sql.Tx, episodeID string, diskDuration float64, hostPath string) bool {
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
			if fileExists(hostPath) {
				if fi, err := os.Stat(hostPath); err == nil {
					metaMap["size"] = fi.Size()
					metaMap["mtimeMs"] = fi.ModTime().UnixNano() / 1e6
				}
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
		fmt.Fprintf(os.Stderr, "[-] Failed to update database transaction for episode %s: %v\n", episodeID, err)
		return false
	}

	return true
}

type durationJob struct {
	epIndex  int
	hostPath string
}

type durationResult struct {
	epIndex      int
	diskDuration float64
}

func rescanPodcastEpisodes(client *ABSClient, item PodcastItem, dryRun bool, db *sql.DB, podcastsDir string, verbose bool, silent bool) (int, int) {
	podcastTitle := item.Media.Metadata.Title
	if podcastTitle == "" {
		podcastTitle = "Untitled Podcast"
	}
	itemID := item.ID
	episodes := item.Media.Episodes

	rescanCount := 0
	checkedCount := 0

	numWorkers := 4
	if len(episodes) < numWorkers {
		numWorkers = len(episodes)
	}
	if numWorkers <= 0 {
		return 0, 0
	}

	jobs := make(chan durationJob, len(episodes))
	results := make(chan durationResult, len(episodes))

	for w := 1; w <= numWorkers; w++ {
		go func() {
			for job := range jobs {
				dur := getMP3DiskDuration(job.hostPath)
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
		hostPath := resolveHostPath(path, podcastsDir)
		if hostPath == "" || !fileExists(hostPath) {
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

	episodesToUpdate := make([]PodcastEpisode, 0)
	episodeDurations := make(map[string]float64)
	episodeHostPaths := make(map[string]string)

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
			episodeHostPaths[ep.ID] = resolveHostPath(path, podcastsDir)
		}
	}

	if len(episodesToUpdate) > 0 {
		if !silent {
			fmt.Print("\r\x1b[K")
			fmt.Printf("=== Podcast: %s (%d downloaded episode(s)) ===\n", podcastTitle, len(episodes))
		}

		for _, ep := range episodesToUpdate {
			rescanCount++
			epTitle := ep.Title
			if epTitle == "" {
				epTitle = "Untitled Episode"
			}
			diskDuration := episodeDurations[ep.ID]
			dbDuration := ep.AudioFile.Duration
			if dbDuration <= 0 {
				dbDuration = ep.Duration
			}
			if !silent {
				if verbose {
					fmt.Printf("  Episode: %s (ID: %s)\n", epTitle, ep.ID)
				} else {
					fmt.Printf("  Episode: %s\n", epTitle)
				}
				fmt.Printf("      Disk Duration: %.2fs | DB Duration: %.2fs (Shorter by %.2fs)\n", diskDuration, dbDuration, dbDuration-diskDuration)
			}

			if dryRun {
				if !silent {
					fmt.Println("      Dry run: Would rescan episode and update database.")
				}
			} else if tx != nil {
				if !silent && verbose {
					fmt.Println("      Queueing SQLite transaction update...")
				}
				updateEpisodeInTx(tx, ep.ID, diskDuration, episodeHostPaths[ep.ID])
			}
		}
	}
	if tx != nil {
		tx.Commit()
	}

	if rescanCount > 0 && !dryRun {
		if !silent {
			if verbose {
				fmt.Printf("      Triggering Audiobookshelf item scan for item ID: %s...\n", itemID)
			} else {
				fmt.Println("      Triggering Audiobookshelf item scan...")
			}
		}
		_, err := client.Request(fmt.Sprintf("/api/items/%s/scan", itemID), "POST", nil)
		if err != nil {
			printError(fmt.Sprintf("      Audiobookshelf scan API request failed: %v", err))
		} else {
			if !silent {
				fmt.Println("      Audiobookshelf scan triggered successfully.")
			}
		}
	}

	if rescanCount > 0 && !silent {
		desc := "rescanned and updated"
		if dryRun {
			desc = "would be rescanned/updated"
		}
		fmt.Printf("Total %d episode(s) in %s %s.\n", rescanCount, podcastTitle, desc)
	}

	return rescanCount, checkedCount
}
