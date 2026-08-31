package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func findAudioFilesForRemote(paths []string, defaultDir string) []string {
	var targetPaths []string
	if len(paths) > 0 {
		targetPaths = paths
	} else if defaultDir != "" {
		targetPaths = []string{defaultDir}
	} else {
		return nil
	}

	var results []string
	seen := make(map[string]bool)

	for _, p := range targetPaths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			mp3s := findMP3Files(p)
			for _, m := range mp3s {
				absM, err := filepath.Abs(m)
				if err == nil && !seen[absM] && !strings.Contains(absM, "/.work/") && !strings.HasSuffix(absM, ".precut") {
					seen[absM] = true
					results = append(results, absM)
				}
			}
		} else {
			if strings.HasSuffix(strings.ToLower(p), ".mp3") {
				absP, err := filepath.Abs(p)
				if err == nil && !seen[absP] {
					seen[absP] = true
					results = append(results, absP)
				}
			}
		}
	}
	return results
}

func runRemotePush(cfg *Config, args []string, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote push requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
	}

	defaultDir := ""
	if cfg != nil {
		defaultDir = cfg.PodcastsDir
	}
	files := findAudioFilesForRemote(args, defaultDir)
	if len(files) == 0 {
		return fmt.Errorf("no audio (.mp3) files found to push for batch processing")
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	var toPush []string
	for _, f := range files {
		if !isEpisodeCompleted(f) {
			toPush = append(toPush, f)
		}
	}
	if len(toPush) == 0 {
		toPush = files
	}

	if !quiet {
		fmt.Printf("Pushing %d audio file(s) to mirror directory on %s:%s...\n", len(toPush), targetHost, remoteWorkDir)
	}

	pushedCount := 0
	for _, f := range toPush {
		relPath, _ := computeRelativeMediaDir(defaultDir, f)
		remoteDstDir := fmt.Sprintf("%s/%s", remoteWorkDir, filepath.Dir(relPath))
		remoteDstFile := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
		remoteDstStatus := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)

		mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDstDir)
		_, _ = transport.Exec(targetHost, mkdirCmd)

		localStat := getOrCreateEpisodeStatus(f)
		localStat.Status = StateQueuedRemote
		_ = saveEpisodeStatus(statusPathFor(f), localStat)

		remoteStat := *localStat
		remoteStat.Status = StateAwaitingTranscription
		remoteStat.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		tmpStatPath := filepath.Join(os.TempDir(), fmt.Sprintf("rem_stat_%d.json", time.Now().UnixNano()))
		_ = saveEpisodeStatus(tmpStatPath, &remoteStat)

		if err := transport.Upload(targetHost, f, remoteDstFile); err != nil {
			_ = os.Remove(tmpStatPath)
			return fmt.Errorf("failed to upload audio %s to %s: %w", f, targetHost, err)
		}
		if err := transport.Upload(targetHost, tmpStatPath, remoteDstStatus); err != nil {
			_ = os.Remove(tmpStatPath)
			return fmt.Errorf("failed to upload status file for %s to %s: %w", f, targetHost, err)
		}
		_ = os.Remove(tmpStatPath)
		pushedCount++
	}

	triggerFile := fmt.Sprintf("%s/.scan_trigger", remoteWorkDir)
	_, _ = transport.Exec(targetHost, fmt.Sprintf("touch %s", triggerFile))

	if !quiet {
		fmt.Printf("Triggering remote worker/scan on %s...\n", targetHost)
	}

	workerCmd := fmt.Sprintf("nohup ~/.local/bin/abs remote scan %s > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, workerCmd); err != nil {
		altCmd := fmt.Sprintf("nohup abs remote scan %s > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
		_, _ = transport.Exec(targetHost, altCmd)
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Successfully pushed %d episode(s) to %s:%s.\n", pushedCount, targetHost, remoteWorkDir)
		fmt.Printf("  - Check status: abs remote status %s\n", targetHost)
		fmt.Printf("  - Pull results: abs remote pull %s\n", targetHost)
	}

	return nil
}
