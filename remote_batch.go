package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runRemotePush(cfg *Config, args []string, host string, transport RemoteTransport, priority int, quiet, verbose bool) error {
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

	toPush, err := resolveFilesToPush(args, defaultDir, quiet)
	if err != nil || len(toPush) == 0 {
		return err
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	if priority > 0 {
		for _, f := range toPush {
			st := getOrCreateEpisodeStatus(f)
			st.Priority = priority
			_ = saveEpisodeStatus(statusPathFor(f), st)
		}
	}

	sortAudioFilesByDuration(toPush)

	if !quiet {
		if priority > 0 {
			fmt.Printf("Pushing %d audio file(s) [priority: %d] to mirror directory on %s:%s...\n", len(toPush), priority, targetHost, remoteWorkDir)
		} else {
			fmt.Printf("Pushing %d audio file(s) to mirror directory on %s:%s...\n", len(toPush), targetHost, remoteWorkDir)
		}
	}

	pushedCount := 0
	for _, f := range toPush {
		if err := pushSingleAudioFile(f, defaultDir, remoteWorkDir, targetHost, priority, transport); err != nil {
			return err
		}
		pushedCount++
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Successfully pushed %d episode(s) to %s:%s.\n", pushedCount, targetHost, remoteWorkDir)
		fmt.Printf("  - Check status: abs remote status %s\n", targetHost)
		fmt.Printf("  - Pull results: abs remote pull %s\n", targetHost)
	}

	return ensureRemoteEnvironmentAndWorker(cfg, targetHost, remoteWorkDir, transport, quiet)
}

func resolveFilesToPush(args []string, defaultDir string, quiet bool) ([]string, error) {
	files := findAudioFilesForRemote(args, defaultDir)
	if len(files) == 0 {
		return nil, fmt.Errorf("no audio (.mp3) files found to push for batch processing")
	}

	var toPush []string
	for _, f := range files {
		if !isEpisodeCompleted(f) && !isEpisodeInRemoteFlight(f) {
			toPush = append(toPush, f)
		}
	}
	if len(toPush) == 0 {
		if len(args) > 0 && len(files) > 0 {
			toPush = files
		} else {
			if !quiet {
				fmt.Println("All audio files are already completed or currently queued/processing remotely.")
			}
			return nil, nil
		}
	}
	return toPush, nil
}

func pushSingleAudioFile(f, defaultDir, remoteWorkDir, targetHost string, priority int, transport RemoteTransport) error {
	relPath, _ := computeRelativeMediaDir(defaultDir, f)
	remoteDstDir := fmt.Sprintf("%s/%s", remoteWorkDir, filepath.Dir(relPath))
	remoteDstFile := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
	remoteDstStatus := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)

	mkdirCmd := fmt.Sprintf("mkdir -p %s", shellQuoteHomePath(remoteDstDir))
	_, _ = transport.Exec(targetHost, mkdirCmd)

	localStat := getOrCreateEpisodeStatus(f)

	remoteStat := *localStat
	remoteStat.Status = StateAwaitingTranscription
	remoteStat.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if priority > 0 {
		remoteStat.Priority = priority
	}

	workDir := workDirFor(f)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory for %s: %w", f, err)
	}
	tmpStatPath := filepath.Join(workDir, fmt.Sprintf("rem_stat_%d.json", time.Now().UnixNano()))
	verifyTempFile(tmpStatPath)

	if err := saveEpisodeStatus(tmpStatPath, &remoteStat); err != nil {
		return fmt.Errorf("failed to save staging status for %s: %w", f, err)
	}
	defer os.Remove(tmpStatPath)

	if err := transport.Upload(targetHost, f, remoteDstFile); err != nil {
		return fmt.Errorf("failed to upload audio %s to %s: %w", f, targetHost, err)
	}
	if err := transport.Upload(targetHost, tmpStatPath, remoteDstStatus); err != nil {
		return fmt.Errorf("failed to upload status file for %s to %s: %w", f, targetHost, err)
	}

	localStat.Status = StateQueuedRemote
	if priority > 0 {
		localStat.Priority = priority
	}
	if err := saveEpisodeStatus(statusPathFor(f), localStat); err != nil {
		return fmt.Errorf("failed to save local status for %s: %w", f, err)
	}
	return nil
}

func ensureRemoteEnvironmentAndWorker(cfg *Config, targetHost string, remoteWorkDir string, transport RemoteTransport, quiet bool) error {
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return nil
	}
	if transport == nil {
		transport = getRemoteTransport()
	}
	if _, isReal := transport.(*DefaultSSHTransport); !isReal {
		return nil
	}
	if strings.Contains(targetHost, "-box") || strings.Contains(targetHost, "mock") || strings.Contains(targetHost, "test") {
		return nil
	}
	if remoteWorkDir == "" {
		remoteWorkDir = "~/abs_remote"
	}

	if cfg != nil && cfg.WhisperWakeCommand != "" {
		if !quiet {
			fmt.Printf("[+] Ensuring remote host '%s' is awake and reachable...\n", targetHost)
		}
		wakeWhisperServer(cfg.WhisperURL, cfg.WhisperWakeCommand, quiet)
	}

	if !isRemoteHostReachable(targetHost, transport) {
		return fmt.Errorf("remote host '%s' is unreachable via SSH", targetHost)
	}

	checkWhisperServiceReadiness(cfg, targetHost, transport, quiet)
	startRemoteWorkerProcess(targetHost, remoteWorkDir, transport, quiet)
	verifyRemoteWorkerStartup(targetHost, remoteWorkDir, transport, quiet)
	return nil
}

func checkWhisperServiceReadiness(cfg *Config, targetHost string, transport RemoteTransport, quiet bool) {
	if !quiet {
		fmt.Printf("[+] Checking Faster-Whisper container on %s...\n", targetHost)
	}

	startContainerCmd := `
CID=$(docker ps -a --filter ancestor=fedirz/faster-whisper-server -q | head -n 1)
if [ -n "$CID" ]; then
    RUNNING=$(docker inspect -f '{{.State.Running}}' "$CID" 2>/dev/null)
    if [ "$RUNNING" != "true" ]; then
        docker start "$CID" >/dev/null 2>&1
    fi
fi
`
	_, _ = transport.Exec(targetHost, startContainerCmd)

	whisperCheckURL := fmt.Sprintf("http://%s:8000/health", targetHost)
	if cfg != nil && cfg.WhisperURL != "" {
		whisperCheckURL = cfg.WhisperURL
	}
	client := &http.Client{Timeout: 3 * time.Second}
	whisperReady := false
	for i := 0; i < 30; i++ {
		resp, err := client.Get(whisperCheckURL)
		if err == nil {
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				whisperReady = true
				break
			}
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	if !whisperReady {
		rootURL := fmt.Sprintf("http://%s:8000/", targetHost)
		if resp, err := client.Get(rootURL); err == nil {
			resp.Body.Close()
			whisperReady = true
		}
	}

	if !whisperReady {
		if !quiet {
			fmt.Printf("[-] Warning: Whisper service on %s did not respond to health check within 30s. Continuing with worker start...\n", targetHost)
		}
	} else if !quiet {
		fmt.Printf("[✓] Faster-Whisper service is ready on %s.\n", targetHost)
	}
}

func startRemoteWorkerProcess(targetHost, remoteWorkDir string, transport RemoteTransport, quiet bool) {
	triggerFile := fmt.Sprintf("%s/.scan_trigger", remoteWorkDir)
	_, _ = transport.Exec(targetHost, fmt.Sprintf("touch %s", triggerFile))

	if !quiet {
		fmt.Printf("[+] Starting remote worker on %s...\n", targetHost)
	}

	workerCmd := fmt.Sprintf("nohup ~/.local/bin/abs remote scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, workerCmd); err != nil {
		altCmd := fmt.Sprintf("nohup abs remote scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
		_, _ = transport.Exec(targetHost, altCmd)
	}
}

func verifyRemoteWorkerStartup(targetHost, remoteWorkDir string, transport RemoteTransport, quiet bool) {
	if !quiet {
		fmt.Printf("[+] Verifying remote conversion startup on %s...\n", targetHost)
	}

	workerStarted := false
	var activeTaskName string
	var activeTaskDuration float64

	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)

		activeOut, _ := transport.Exec(targetHost, fmt.Sprintf("grep -l -E '\"status\": \"(transcribing_remotely|cutting_remotely)\"' %s/*/*.mp3.json 2>/dev/null", remoteWorkDir))
		activeFiles := splitLines(strings.TrimSpace(activeOut))
		if len(activeFiles) > 0 && activeFiles[0] != "" {
			activeJsonPath := activeFiles[0]
			activeTaskName = cleanRemoteRelPath(activeJsonPath, remoteWorkDir)
			if data, err := transport.Exec(targetHost, fmt.Sprintf("cat %q", activeJsonPath)); err == nil && data != "" {
				var st EpisodeStatusFile
				if json.Unmarshal([]byte(data), &st) == nil {
					activeTaskDuration = st.Original.DurationSec
				}
			}
			workerStarted = true
			break
		}

		lockCheck, _ := transport.Exec(targetHost, fmt.Sprintf("test -f %s/.worker.lock && pgrep -f 'abs.*(scan|worker)' && echo RUNNING", remoteWorkDir))
		if strings.Contains(lockCheck, "RUNNING") {
			workerStarted = true
			if i >= 3 {
				break
			}
		}
	}

	if !quiet {
		if activeTaskName != "" {
			durStr := "--:--"
			if activeTaskDuration > 0 {
				durStr = formatClock(activeTaskDuration)
			}
			fmt.Printf("[✓] Remote worker is active on %s: converting '%s' (Length: %s)\n", targetHost, activeTaskName, durStr)
		} else if workerStarted {
			fmt.Printf("[✓] Remote worker successfully started and running on %s.\n", targetHost)
		} else {
			fmt.Printf("[-] Notice: Remote worker was launched on %s (check status with: abs remote status %s).\n", targetHost, targetHost)
		}
	}
}
