package main

import (
	"fmt"
	"time"
)

func printRemoteStatus(targetHost string, status RemoteServerStatus, readyEpisodes []RemoteDoneItem, archiveCount int, cfg *Config, quiet, verbose bool) error {
	if quiet {
		return nil
	}

	printRemoteServerSummary(targetHost, status, readyEpisodes, archiveCount)

	if len(status.QueuedTasks) > 0 {
		printRemoteQueuedTasks(status.QueuedTasks, cfg)
	}

	if len(readyEpisodes) > 0 {
		fmt.Println()
		fmt.Println(bold("Episodes ready for copy back (abs remote pull):"))
		for _, ep := range readyEpisodes {
			fmt.Printf("  [✓] %s (ad saved: %.1fs)\n", bold(ep.RelPath), ep.CutDurationSec)
		}
	}

	if len(status.ActiveBatches) > 0 {
		printRemoteBatches(status.ActiveBatches, verbose)
	}
	fmt.Println()

	return nil
}

func printRemoteServerSummary(targetHost string, status RemoteServerStatus, readyEpisodes []RemoteDoneItem, archiveCount int) {
	fmt.Printf("\n=== Remote Server Status: %s ===\n", bold(targetHost))
	fmt.Printf("  - Reachable:       %s\n", boldGreen("Yes"))
	if status.BinaryVersion != "" {
		fmt.Printf("  - Version:         %s\n", status.BinaryVersion)
	}
	workerStatusStr := bold("Not running (idle)")
	if status.WorkerRunning {
		if status.ActiveTask != "" {
			workerStatusStr = boldGreen("Running")
		} else {
			workerStatusStr = boldGreen("Running (Scanning mirror)")
		}
	}
	fmt.Printf("  - Worker Process:  %s\n", workerStatusStr)
	if status.WorkerRunning && status.ActiveTask != "" {
		fmt.Printf("      • Task:        %s\n", bold(status.ActiveTask))
		if status.ActiveDuration != "" {
			fmt.Printf("      • Length:      %s\n", blue(status.ActiveDuration))
		}
		if status.ActiveStage != "" {
			fmt.Printf("      • Stage:       %s\n", status.ActiveStage)
		}
		if status.ActiveElapsed != "" {
			progressInfo := fmt.Sprintf("Elapsed: %s", status.ActiveElapsed)
			if status.ActiveETA != "" {
				progressInfo += fmt.Sprintf(" | %s", status.ActiveETA)
			}
			fmt.Printf("      • Progress:    %s\n", boldCyan(progressInfo))
		}
	}
	if len(status.QueuedTasks) > 0 {
		fmt.Printf("  - Remote Queue:    %s\n", boldYellow(fmt.Sprintf("%d job(s) scheduled", len(status.QueuedTasks))))
	} else {
		fmt.Printf("  - Remote Queue:    0 jobs\n")
	}
	fmt.Printf("  - Ready to Pull:   %s\n", boldGreen(fmt.Sprintf("%d episode(s)", len(readyEpisodes))))
	fmt.Printf("  - Remote Archive:  %d episode(s)\n", archiveCount)
	if len(status.ActiveBatches) > 0 {
		fmt.Printf("  - Staged Batches:  %d\n", len(status.ActiveBatches))
	}
	if status.Message != "" {
		fmt.Printf("  - Notice:          %s\n", boldYellow(status.Message))
	}
}

func printRemoteQueuedTasks(queuedTasks []string, cfg *Config) {
	fmt.Println()
	fmt.Println(bold("Remote Queue (Scheduled Jobs):"))
	now := time.Now()
	hasPrintedSeparator := false
	for idx, task := range queuedTasks {
		var dur float64
		var pri int
		var isRec bool
		var pt time.Time
		if cfg != nil && cfg.PodcastsDir != "" {
			localAudio := resolveLocalAudioPath(cfg.PodcastsDir, task)
			dur = getEpisodeDurationForQueue(localAudio)
			pri = getEpisodePriorityForQueue(localAudio)
			isRec, pt = isEpisodeRecent24h(localAudio, now)
		}
		durStr := "--:--"
		if dur > 0 {
			durStr = formatClock(dur)
		}

		if idx > 0 && !hasPrintedSeparator {
			prevTask := queuedTasks[idx-1]
			prevRec := false
			if cfg != nil && cfg.PodcastsDir != "" {
				prevAudio := resolveLocalAudioPath(cfg.PodcastsDir, prevTask)
				prevRec, _ = isEpisodeRecent24h(prevAudio, now)
			}
			if prevRec && !isRec {
				fmt.Printf("  %s\n", repeatStr("─", 76))
				hasPrintedSeparator = true
			}
		}

		timeTag := ""
		if isRec && !pt.IsZero() {
			ago := now.Sub(pt)
			if ago < time.Hour {
				timeTag = fmt.Sprintf(" (%dm ago)", int(ago.Minutes()))
			} else {
				timeTag = fmt.Sprintf(" (%dh ago)", int(ago.Hours()))
			}
		}

		if pri > 0 {
			fmt.Printf("  [%d] %s  %s%s (priority: %d)\n", idx+1, blue(fmt.Sprintf("[%s]", durStr)), task, timeTag, pri)
		} else {
			fmt.Printf("  [%d] %s  %s%s\n", idx+1, blue(fmt.Sprintf("[%s]", durStr)), task, timeTag)
		}
	}
}

func printRemoteBatches(batches []RemoteBatchManifest, verbose bool) {
	fmt.Println()
	fmt.Println(bold("Batches on remote server:"))
	for _, b := range batches {
		statusColor := boldYellow(string(b.Status))
		if b.Status == BatchStatusCompleted {
			statusColor = boldGreen(string(b.Status))
		} else if b.Status == BatchStatusFailed {
			statusColor = boldRed(string(b.Status))
		}

		fmt.Printf("  [%s] Status: %s | Progress: %d/%d completed | Created: %s\n",
			bold(b.BatchID), statusColor, b.CompletedItems, b.TotalItems, b.CreatedAt)

		if verbose {
			for _, it := range b.Items {
				itemStatus := it.Status
				errNote := ""
				if it.Error != "" {
					errNote = fmt.Sprintf(" (Error: %s)", it.Error)
				}
				fmt.Printf("      - %s: %s [%s]%s\n", it.ID, it.AudioFileName, itemStatus, errNote)
			}
		}
	}
}
