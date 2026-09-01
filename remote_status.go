package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runRemoteStatus(cfg *Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote status requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
	}

	status := RemoteServerStatus{
		Host:      targetHost,
		Reachable: isRemoteHostReachable(targetHost, transport),
	}

	if !status.Reachable {
		fmt.Printf("Remote Host: %s [UNREACHABLE]\n", targetHost)
		return nil
	}

	verOut, _ := transport.Exec(targetHost, "~/.local/bin/abs --version 2>/dev/null || ~/abs_remote/bin/abs --version 2>/dev/null || abs --version 2>/dev/null || cat ~/abs_remote/VERSION 2>/dev/null || cat ~/.config/abs/VERSION 2>/dev/null || ~/.local/bin/abs help 2>/dev/null || ~/abs_remote/bin/abs help 2>/dev/null || abs help 2>/dev/null")
	if verOut != "" {
		lines := splitLines(verOut)
		if len(lines) > 0 {
			v := strings.TrimSpace(lines[0])
			v = strings.TrimPrefix(v, "abs version ")
			v = strings.TrimPrefix(v, "abs ")
			status.BinaryVersion = strings.TrimSpace(v)
		}
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	lockData, _ := transport.Exec(targetHost, fmt.Sprintf("cat %s/.worker.lock 2>/dev/null", remoteWorkDir))
	var lockPID int
	if strings.TrimSpace(lockData) != "" {
		lines := splitLines(strings.TrimSpace(lockData))
		if len(lines) > 0 {
			_, _ = fmt.Sscanf(lines[0], "%d", &lockPID)
		}
	}

	if lockPID > 0 {
		aliveOut, _ := transport.Exec(targetHost, fmt.Sprintf("kill -0 %d 2>/dev/null && echo alive", lockPID))
		if strings.TrimSpace(aliveOut) == "alive" {
			status.WorkerRunning = true
		}
	} else {
		psOut, _ := transport.Exec(targetHost, "pgrep -x abs 2>/dev/null")
		status.WorkerRunning = strings.TrimSpace(psOut) != ""
	}

	activeOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l -E '\"status\": \"(transcribing_remotely|cutting_remotely)\"' %s/*/*.mp3.json 2>/dev/null | head -n 1", remoteWorkDir))
	if activePath := strings.TrimSpace(activeOut); activePath != "" {
		status.ActiveTask = cleanRemoteRelPath(activePath, remoteWorkDir)
		statContent, _ := transport.Exec(targetHost, fmt.Sprintf("cat %q 2>/dev/null", activePath))
		if strings.TrimSpace(statContent) != "" {
			var activeSt EpisodeStatusFile
			if json.Unmarshal([]byte(statContent), &activeSt) == nil {
				if activeSt.Original.DurationSec > 0 {
					status.ActiveDuration = formatClock(activeSt.Original.DurationSec)
				}
				status.ActiveStage = activeSt.CurrentStep
				if status.ActiveStage == "" {
					if activeSt.Status == StateTranscribingRemotely {
						status.ActiveStage = "Step 1/3: Whisper Transcription"
					} else if activeSt.Status == StateCuttingRemotely {
						status.ActiveStage = "Step 3/3: FFmpeg Audio Cutting"
					}
				}
				startTimeStr := activeSt.StepStartedAt
				if startTimeStr == "" {
					startTimeStr = activeSt.UpdatedAt
				}
				if startTimeStr != "" {
					if tStart, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
						elapsed := time.Since(tStart)
						status.ActiveElapsed = formatClock(elapsed.Seconds())
						if activeSt.Status == StateTranscribingRemotely && activeSt.Original.DurationSec > 0 {
							estTotalSec := activeSt.Original.DurationSec / 4.5
							if estTotalSec > 0 {
								pct := int((elapsed.Seconds() / estTotalSec) * 100)
								if pct > 99 {
									pct = 99
								}
								remSec := estTotalSec - elapsed.Seconds()
								if remSec < 0 {
									remSec = 5
								}
								status.ActiveETA = fmt.Sprintf("Est. Remaining: ~%s (%d%%)", formatClock(remSec), pct)
							}
						}
					}
				}
			}
		}
		if status.ActiveDuration == "" && cfg != nil && cfg.PodcastsDir != "" {
			localAudio := filepath.Join(cfg.PodcastsDir, status.ActiveTask)
			if dur := getEpisodeDurationForQueue(localAudio); dur > 0 {
				status.ActiveDuration = formatClock(dur)
			}
		}
	}

	queuedOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l '\"status\": \"awaiting_transcription\"' %s/*/*.mp3.json 2>/dev/null", remoteWorkDir))
	if strings.TrimSpace(queuedOut) != "" {
		for _, qPath := range splitLines(strings.TrimSpace(queuedOut)) {
			qPath = strings.TrimSpace(qPath)
			if qPath == "" {
				continue
			}
			status.QueuedTasks = append(status.QueuedTasks, cleanRemoteRelPath(qPath, remoteWorkDir))
		}
		if len(status.QueuedTasks) > 1 {
			durMap := make(map[string]float64, len(status.QueuedTasks))
			priMap := make(map[string]int, len(status.QueuedTasks))
			for _, qTask := range status.QueuedTasks {
				var dur float64
				var pri int
				if cfg != nil && cfg.PodcastsDir != "" {
					localAudio := filepath.Join(cfg.PodcastsDir, qTask)
					dur = getEpisodeDurationForQueue(localAudio)
					pri = getEpisodePriorityForQueue(localAudio)
				}
				durMap[qTask] = dur
				priMap[qTask] = pri
			}
			sort.SliceStable(status.QueuedTasks, func(i, j int) bool {
				pi := priMap[status.QueuedTasks[i]]
				pj := priMap[status.QueuedTasks[j]]
				if pi != pj {
					return pi > pj
				}
				di := durMap[status.QueuedTasks[i]]
				dj := durMap[status.QueuedTasks[j]]
				if di != dj {
					return di < dj
				}
				return status.QueuedTasks[i] < status.QueuedTasks[j]
			})
		}
	}

	logOut, _ := transport.Exec(targetHost, fmt.Sprintf("tail -n 25 %s/worker.log 2>/dev/null", remoteWorkDir))
	workerFailed := false
	var failureReason string
	if logOut != "" {
		if strings.Contains(logOut, "context deadline exceeded") {
			workerFailed = true
			failureReason = "context deadline exceeded (client timeout)"
		} else if strings.Contains(logOut, "panic:") || strings.Contains(logOut, "fatal error:") {
			workerFailed = true
			failureReason = "runtime panic/fatal error"
		} else if strings.Contains(logOut, "failed to connect to Whisper GPU server") || strings.Contains(logOut, "failed to connect to Whisper") {
			workerFailed = true
			failureReason = "whisper server unreachable"
		}
	}

	if workerFailed {
		abortCmd := fmt.Sprintf("pkill -f 'abs.*(scan|worker)' 2>/dev/null || true; rm -f %s/.worker.lock; docker restart $(docker ps -q --filter 'ancestor=fedirz/faster-whisper-server' 2>/dev/null || docker ps -q 2>/dev/null) 2>/dev/null || true", remoteWorkDir)
		_, _ = transport.Exec(targetHost, abortCmd)

		relaunchCmd := fmt.Sprintf("touch %s/.scan_trigger && nohup ~/.local/bin/abs remote scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir, remoteWorkDir)
		_, _ = transport.Exec(targetHost, relaunchCmd)
		status.WorkerRunning = true
		status.Message = fmt.Sprintf("Auto-recovered: detected '%s' in worker.log, restarted worker with updated binary", failureReason)
	} else if !status.WorkerRunning && len(status.QueuedTasks) > 0 {
		wakeWorkerCmd := fmt.Sprintf("touch %s/.scan_trigger && nohup ~/.local/bin/abs remote scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir, remoteWorkDir)
		if _, err := transport.Exec(targetHost, wakeWorkerCmd); err != nil {
			altCmd := fmt.Sprintf("touch %s/.scan_trigger && nohup abs remote scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir, remoteWorkDir)
			_, _ = transport.Exec(targetHost, altCmd)
		}
		status.WorkerRunning = true
		if status.ActiveTask == "" && len(status.QueuedTasks) > 0 {
			status.ActiveTask = status.QueuedTasks[0]
		}
	}

	var readyEpisodes []RemoteDoneItem
	var archiveCount int

	tempDonePath := filepath.Join(os.TempDir(), fmt.Sprintf("status_done_%d.json", time.Now().UnixNano()))
	if err := transport.Download(targetHost, fmt.Sprintf("%s/done.json", remoteWorkDir), tempDonePath); err == nil {
		if doneM, err := loadDoneManifest(tempDonePath); err == nil && doneM != nil {
			for _, it := range doneM.Episodes {
				if it.Status == StateReadyForCopyBack {
					readyEpisodes = append(readyEpisodes, it)
				}
			}
		}
		_ = os.Remove(tempDonePath)
	}

	tempArchPath := filepath.Join(os.TempDir(), fmt.Sprintf("status_arch_%d.json", time.Now().UnixNano()))
	if err := transport.Download(targetHost, fmt.Sprintf("%s/archive.json", remoteWorkDir), tempArchPath); err == nil {
		if archM, err := loadDoneManifest(tempArchPath); err == nil && archM != nil {
			archiveCount = len(archM.Episodes)
		}
		_ = os.Remove(tempArchPath)
	}

	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)
	out, _ := transport.Exec(targetHost, fmt.Sprintf("ls -1 %s 2>/dev/null", remoteStagingDir))
	if strings.TrimSpace(out) != "" {
		batchIDs := splitLines(strings.TrimSpace(out))
		for _, bid := range batchIDs {
			bid = strings.TrimSpace(bid)
			if bid == "" {
				continue
			}

			tempDir := filepath.Join(os.TempDir(), "abs_status", bid)
			_ = os.MkdirAll(tempDir, 0755)
			manPath := filepath.Join(tempDir, "manifest.json")
			remoteMan := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, bid)

			if err := transport.Download(targetHost, remoteMan, manPath); err == nil {
				if m, err := loadManifest(manPath); err == nil {
					status.ActiveBatches = append(status.ActiveBatches, *m)
				}
			}
			_ = os.RemoveAll(tempDir)
		}
	}

	if quiet {
		return nil
	}

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

	if len(status.QueuedTasks) > 0 {
		fmt.Println()
		fmt.Println(bold("Remote Queue (Scheduled Jobs):"))
		for idx, task := range status.QueuedTasks {
			var dur float64
			var pri int
			if cfg != nil && cfg.PodcastsDir != "" {
				localAudio := filepath.Join(cfg.PodcastsDir, task)
				dur = getEpisodeDurationForQueue(localAudio)
				pri = getEpisodePriorityForQueue(localAudio)
			}
			durStr := "--:--"
			if dur > 0 {
				durStr = formatClock(dur)
			}
			if pri > 0 {
				fmt.Printf("  [%d] %s  %s (priority: %d)\n", idx+1, blue(fmt.Sprintf("[%s]", durStr)), task, pri)
			} else {
				fmt.Printf("  [%d] %s  %s\n", idx+1, blue(fmt.Sprintf("[%s]", durStr)), task)
			}
		}
	}

	if len(readyEpisodes) > 0 {
		fmt.Println()
		fmt.Println(bold("Episodes ready for copy back (abs remote pull):"))
		for _, ep := range readyEpisodes {
			fmt.Printf("  [✓] %s (ad saved: %.1fs)\n", bold(ep.RelPath), ep.CutDurationSec)
		}
	}

	if len(status.ActiveBatches) > 0 {
		fmt.Println()
		fmt.Println(bold("Batches on remote server:"))
		for _, b := range status.ActiveBatches {
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
	fmt.Println()

	return nil
}

func runRemoteCancel(cfg *Config, host, batchID string, transport RemoteTransport, quiet bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote cancel requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}
	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)

	if batchID != "" {
		tempDir := filepath.Join(os.TempDir(), "abs_cancel", batchID)
		_ = os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		manPath := filepath.Join(tempDir, "manifest.json")
		remoteMan := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)

		if err := transport.Download(targetHost, remoteMan, manPath); err == nil {
			if m, err := loadManifest(manPath); err == nil {
				m.Status = BatchStatusCancelled
				m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				_ = saveManifest(manPath, m)
				_ = transport.Upload(targetHost, manPath, remoteMan)
			}
		}

		killCmd := fmt.Sprintf("pkill -f 'batch-worker.*%s' 2>/dev/null || true", batchID)
		_, _ = transport.Exec(targetHost, killCmd)

		if !quiet {
			fmt.Printf("Cancelled batch %s on %s.\n", batchID, targetHost)
		}
		return nil
	}

	killAllCmd := "pkill -f 'abs.*(scan|worker|batch-worker)' 2>/dev/null || true"
	_, _ = transport.Exec(targetHost, killAllCmd)

	if !quiet {
		fmt.Printf("Cancelled all active batch/worker processes on %s.\n", targetHost)
	}
	return nil
}

func cleanRemoteRelPath(rawPath, remoteWorkDir string) string {
	rawPath = strings.TrimSuffix(strings.TrimSpace(rawPath), ".json")
	if idx := strings.Index(rawPath, "abs_remote/"); idx != -1 {
		return rawPath[idx+len("abs_remote/"):]
	}
	return filepath.Base(rawPath)
}
