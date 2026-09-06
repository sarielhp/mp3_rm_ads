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

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	fetchRemoteVersionAndWorkerStatus(targetHost, remoteWorkDir, transport, &status)
	fetchRemoteActiveTask(targetHost, remoteWorkDir, transport, cfg, &status)
	fetchAndSortRemoteQueuedTasks(targetHost, remoteWorkDir, transport, cfg, &status)
	checkAndRecoverRemoteWorker(targetHost, remoteWorkDir, transport, &status)

	readyEpisodes, archiveCount, batches := fetchRemoteReadyEpisodesAndBatches(targetHost, remoteWorkDir, transport)
	status.ActiveBatches = batches

	return printRemoteStatus(targetHost, status, readyEpisodes, archiveCount, cfg, quiet, verbose)
}

func fetchRemoteVersionAndWorkerStatus(targetHost, remoteWorkDir string, transport RemoteTransport, status *RemoteServerStatus) {
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
}

func fetchRemoteActiveTask(targetHost, remoteWorkDir string, transport RemoteTransport, cfg *Config, status *RemoteServerStatus) {
	activeOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l -E '\"status\": \"(transcribing_remotely|cutting_remotely)\"' %s/*/*.mp3.json 2>/dev/null | head -n 1", remoteWorkDir))
	activePath := strings.TrimSpace(activeOut)
	if activePath == "" {
		return
	}

	status.ActiveTask = cleanRemoteRelPath(activePath, remoteWorkDir)
	statContent, _ := transport.Exec(targetHost, fmt.Sprintf("cat %s 2>/dev/null", shellQuoteHomePath(activePath)))
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

func fetchAndSortRemoteQueuedTasks(targetHost, remoteWorkDir string, transport RemoteTransport, cfg *Config, status *RemoteServerStatus) {
	queuedOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l '\"status\": \"awaiting_transcription\"' %s/*/*.json 2>/dev/null", remoteWorkDir))
	if strings.TrimSpace(queuedOut) == "" {
		return
	}

	for _, qPath := range splitLines(strings.TrimSpace(queuedOut)) {
		qPath = strings.TrimSpace(qPath)
		if qPath != "" {
			status.QueuedTasks = append(status.QueuedTasks, cleanRemoteRelPath(qPath, remoteWorkDir))
		}
	}

	if len(status.QueuedTasks) <= 1 {
		return
	}

	now := time.Now()
	durMap := make(map[string]float64, len(status.QueuedTasks))
	priMap := make(map[string]int, len(status.QueuedTasks))
	pubMap := make(map[string]time.Time, len(status.QueuedTasks))
	recMap := make(map[string]bool, len(status.QueuedTasks))
	for _, qTask := range status.QueuedTasks {
		var dur float64
		var pri int
		var isRec bool
		var pt time.Time
		if cfg != nil && cfg.PodcastsDir != "" {
			localAudio := resolveLocalAudioPath(cfg.PodcastsDir, qTask)
			dur = getEpisodeDurationForQueue(localAudio)
			pri = getEpisodePriorityForQueue(localAudio)
			isRec, pt = isEpisodeRecent24h(localAudio, now)
		}
		durMap[qTask] = dur
		priMap[qTask] = pri
		pubMap[qTask] = pt
		recMap[qTask] = isRec
	}
	sort.SliceStable(status.QueuedTasks, func(i, j int) bool {
		ti := status.QueuedTasks[i]
		tj := status.QueuedTasks[j]
		pi := priMap[ti]
		pj := priMap[tj]
		if pi != pj {
			return pi > pj
		}
		recI := recMap[ti]
		recJ := recMap[tj]
		if recI != recJ {
			return recI && !recJ
		}
		if recI {
			pubI := pubMap[ti]
			pubJ := pubMap[tj]
			if !pubI.Equal(pubJ) {
				return pubI.After(pubJ)
			}
			di := durMap[ti]
			dj := durMap[tj]
			if di != dj {
				return di < dj
			}
			return ti < tj
		}
		di := durMap[ti]
		dj := durMap[tj]
		if di != dj {
			return di < dj
		}
		pubI := pubMap[ti]
		pubJ := pubMap[tj]
		if !pubI.Equal(pubJ) {
			return pubI.After(pubJ)
		}
		return ti < tj
	})
}

func checkAndRecoverRemoteWorker(targetHost, remoteWorkDir string, transport RemoteTransport, status *RemoteServerStatus) {
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
}

func fetchRemoteReadyEpisodesAndBatches(targetHost, remoteWorkDir string, transport RemoteTransport) ([]RemoteDoneItem, int, []RemoteBatchManifest) {
	var readyEpisodes []RemoteDoneItem
	var archiveCount int
	var batches []RemoteBatchManifest

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
			if bid == "" || !validateBatchID(bid) {
				continue
			}

			tempDir := filepath.Join(os.TempDir(), "abs_status", bid)
			_ = os.MkdirAll(tempDir, 0755)
			manPath := filepath.Join(tempDir, "manifest.json")
			remoteMan := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, bid)

			if err := transport.Download(targetHost, remoteMan, manPath); err == nil {
				if m, err := loadManifest(manPath); err == nil {
					batches = append(batches, *m)
				}
			}
			_ = os.RemoveAll(tempDir)
		}
	}
	return readyEpisodes, archiveCount, batches
}
