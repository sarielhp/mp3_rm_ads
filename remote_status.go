package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	verOut, _ := transport.Exec(targetHost, "~/.local/bin/abs help 2>/dev/null || ~/abs_remote/bin/abs help 2>/dev/null || abs help 2>/dev/null")
	if verOut != "" {
		lines := splitLines(verOut)
		if len(lines) > 0 {
			status.BinaryVersion = strings.TrimSpace(lines[0])
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

	if status.WorkerRunning {
		activeOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l -E '\"(transcribing_remotely|cutting_remotely)\"' %s/*/*.mp3.json 2>/dev/null | head -n 1", remoteWorkDir))
		if activePath := strings.TrimSpace(activeOut); activePath != "" {
			rel := strings.TrimSuffix(strings.TrimPrefix(activePath, remoteWorkDir+"/"), ".json")
			status.ActiveTask = rel
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
		fmt.Printf("  - Binary:          %s\n", status.BinaryVersion)
	}
	workerStatusStr := bold("Idle (Not running)")
	if status.WorkerRunning {
		if status.ActiveTask != "" {
			workerStatusStr = boldGreen(fmt.Sprintf("Running (Processing: %s)", status.ActiveTask))
		} else {
			workerStatusStr = boldGreen("Running (Scanning mirror)")
		}
	}
	fmt.Printf("  - Worker Process:  %s\n", workerStatusStr)
	fmt.Printf("  - Ready to Pull:   %s\n", boldGreen(fmt.Sprintf("%d episode(s)", len(readyEpisodes))))
	fmt.Printf("  - Remote Archive:  %d episode(s)\n", archiveCount)
	if len(status.ActiveBatches) > 0 {
		fmt.Printf("  - Staged Batches:  %d\n", len(status.ActiveBatches))
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
