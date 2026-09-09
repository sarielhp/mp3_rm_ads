package remote

import (
	"fmt"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/format"
	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

func PrintRemoteStatus(targetHost string, status types.RemoteServerStatus, readyEpisodes []RemoteDoneItem, archiveCount int, cfg *types.Config, quiet, verbose bool) error {
	if quiet {
		return nil
	}

	PrintRemoteServerSummary(targetHost, status, readyEpisodes, archiveCount)

	if len(status.QueuedTasks) > 0 {
		PrintRemoteQueuedTasks(status.QueuedTasks, cfg)
	}

	if len(readyEpisodes) > 0 {
		fmt.Println()
		fmt.Println(util.Bold("Episodes ready for copy back (abs offload pull):"))
		for _, ep := range readyEpisodes {
			fmt.Printf("  [✓] %s (ad saved: %.1fs)\n", util.Bold(ep.RelPath), ep.CutDurationSec)
		}
	}

	if len(status.ActiveBatches) > 0 {
		PrintRemoteBatches(status.ActiveBatches, verbose)
	}
	fmt.Println()

	return nil
}

func PrintRemoteServerSummary(targetHost string, status types.RemoteServerStatus, readyEpisodes []RemoteDoneItem, archiveCount int) {
	fmt.Printf("\n=== Remote Server Status: %s ===\n", util.Bold(targetHost))
	fmt.Printf("  - Reachable:       %s\n", util.BoldGreen("Yes"))
	if status.BinaryVersion != "" {
		fmt.Printf("  - Version:         %s\n", status.BinaryVersion)
	}
	workerStatusStr := util.Bold("Not running (idle)")
	if status.WorkerRunning {
		if status.ActiveTask != "" {
			workerStatusStr = util.BoldGreen("Running")
		} else {
			workerStatusStr = util.BoldGreen("Running (Scanning mirror)")
		}
	}
	fmt.Printf("  - Worker Process:  %s\n", workerStatusStr)
	if status.WorkerRunning && status.ActiveTask != "" {
		fmt.Printf("      • Task:        %s\n", util.Bold(status.ActiveTask))
		if status.ActiveDuration != "" {
			fmt.Printf("      • Length:      %s\n", util.Blue(status.ActiveDuration))
		}
		if status.ActiveStage != "" {
			fmt.Printf("      • Stage:       %s\n", status.ActiveStage)
		}
		if status.ActiveElapsed != "" {
			progressInfo := fmt.Sprintf("Elapsed: %s", status.ActiveElapsed)
			if status.ActiveETA != "" {
				progressInfo += fmt.Sprintf(" | %s", status.ActiveETA)
			}
			fmt.Printf("      • Progress:    %s\n", util.BoldCyan(progressInfo))
		}
	}
	if len(status.QueuedTasks) > 0 {
		fmt.Printf("  - Remote Queue:    %s\n", util.BoldYellow(fmt.Sprintf("%d job(s) scheduled", len(status.QueuedTasks))))
	} else {
		fmt.Printf("  - Remote Queue:    0 jobs\n")
	}
	fmt.Printf("  - Ready to Pull:   %s\n", util.BoldGreen(fmt.Sprintf("%d episode(s)", len(readyEpisodes))))
	fmt.Printf("  - Remote Archive:  %d episode(s)\n", archiveCount)
	if len(status.ActiveBatches) > 0 {
		fmt.Printf("  - Staged Batches:  %d\n", len(status.ActiveBatches))
	}
	if status.Message != "" {
		fmt.Printf("  - Notice:          %s\n", util.BoldYellow(status.Message))
	}
}

func PrintRemoteQueuedTasks(queuedTasks []string, cfg *types.Config) {
	fmt.Println()
	fmt.Println(util.Bold("Remote Queue (Scheduled Jobs):"))
	now := time.Now()
	hasPrintedSeparator := false
	for idx, task := range queuedTasks {
		var dur float64
		var pri int
		var isRec bool
		var pt time.Time
		if cfg != nil && cfg.PodcastsDir != "" {
			localAudio := ResolveLocalAudioPath(cfg.PodcastsDir, task)
			dur = GetEpisodeDurationForQueue(localAudio)
			pri = GetEpisodePriorityForQueue(localAudio)
			isRec, pt = IsEpisodeRecent24h(localAudio, now)
		}
		durStr := "--:--"
		if dur > 0 {
			durStr = format.FormatClock(dur)
		}

		if idx > 0 && !hasPrintedSeparator {
			prevTask := queuedTasks[idx-1]
			prevRec := false
			if cfg != nil && cfg.PodcastsDir != "" {
				prevAudio := ResolveLocalAudioPath(cfg.PodcastsDir, prevTask)
				prevRec, _ = IsEpisodeRecent24h(prevAudio, now)
			}
			if prevRec && !isRec {
				fmt.Printf("  %s\n", strings.Repeat("─", 76))
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
			fmt.Printf("  [%d] %s  %s%s (priority: %d)\n", idx+1, util.Blue(fmt.Sprintf("[%s]", durStr)), task, timeTag, pri)
		} else {
			fmt.Printf("  [%d] %s  %s%s\n", idx+1, util.Blue(fmt.Sprintf("[%s]", durStr)), task, timeTag)
		}
	}
}

func PrintRemoteBatches(batches []types.RemoteBatchManifest, verbose bool) {
	fmt.Println()
	fmt.Println(util.Bold("Batches on remote server:"))
	for _, b := range batches {
		statusColor := util.BoldYellow(string(b.Status))
		if b.Status == types.BatchStatusCompleted {
			statusColor = util.BoldGreen(string(b.Status))
		} else if b.Status == types.BatchStatusFailed {
			statusColor = util.BoldRed(string(b.Status))
		}

		fmt.Printf("  [%s] Status: %s | Progress: %d/%d completed | Created: %s\n",
			util.Bold(b.BatchID), statusColor, b.CompletedItems, b.TotalItems, b.CreatedAt)

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
