package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func getEpisodeDurationForQueue(audioPath string) float64 {
	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		if st.Original.DurationSec > 0 {
			return st.Original.DurationSec
		}
	}
	dur := getAudioDuration(audioPath)
	if dur <= 0 {
		dur = getMP3DiskDuration(audioPath)
	}
	if dur <= 0 {
		if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
			return float64(fi.Size()) / 16000.0
		}
	}
	return dur
}

func getEpisodePriorityForQueue(audioPath string) int {
	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		return st.Priority
	}
	return 0
}

func isEpisodeRecent24h(audioPath string, now time.Time) (bool, time.Time) {
	pt := getEpisodePublicationTime(audioPath)
	if pt.IsZero() {
		return false, pt
	}
	cutoff := now.Add(-24 * time.Hour)
	return pt.After(cutoff), pt
}

func sortAudioFilesByQueuePolicy(files []string, now time.Time) {
	if len(files) <= 1 {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	durMap := make(map[string]float64, len(files))
	priMap := make(map[string]int, len(files))
	pubMap := make(map[string]time.Time, len(files))
	recentMap := make(map[string]bool, len(files))

	for _, f := range files {
		durMap[f] = getEpisodeDurationForQueue(f)
		priMap[f] = getEpisodePriorityForQueue(f)
		isRec, pt := isEpisodeRecent24h(f, now)
		pubMap[f] = pt
		recentMap[f] = isRec
	}

	sort.SliceStable(files, func(i, j int) bool {
		fi := files[i]
		fj := files[j]

		pi := priMap[fi]
		pj := priMap[fj]
		if pi != pj {
			return pi > pj
		}

		recI := recentMap[fi]
		recJ := recentMap[fj]
		if recI != recJ {
			return recI && !recJ
		}

		if recI {
			pubI := pubMap[fi]
			pubJ := pubMap[fj]
			if !pubI.Equal(pubJ) {
				return pubI.After(pubJ)
			}
			di := durMap[fi]
			dj := durMap[fj]
			if di != dj {
				return di < dj
			}
			return fi < fj
		}

		di := durMap[fi]
		dj := durMap[fj]
		if di != dj {
			return di < dj
		}
		pubI := pubMap[fi]
		pubJ := pubMap[fj]
		if !pubI.Equal(pubJ) {
			return pubI.After(pubJ)
		}
		return fi < fj
	})
}

func sortAudioFilesByDuration(files []string) {
	sortAudioFilesByQueuePolicy(files, time.Now())
}

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
			if strings.HasSuffix(filepath.Clean(p), "-1") {
				continue
			}
			mp3s := findMP3Files(p)
			for _, m := range mp3s {
				absM, err := filepath.Abs(m)
				if err == nil && !seen[absM] && !strings.Contains(absM, "/.work/") && !strings.HasSuffix(absM, ".precut") {
					if strings.Contains(absM, "-1/") || filepath.Base(absM) == "podcast.mp3" {
						continue
					}
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
			return nil
		}
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
		relPath, _ := computeRelativeMediaDir(defaultDir, f)
		remoteDstDir := fmt.Sprintf("%s/%s", remoteWorkDir, filepath.Dir(relPath))
		remoteDstFile := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
		remoteDstStatus := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)

		mkdirDir := strings.Replace(remoteDstDir, "~", "$HOME", 1)
		mkdirCmd := fmt.Sprintf("mkdir -p %q", mkdirDir)
		_, _ = transport.Exec(targetHost, mkdirCmd)

		localStat := getOrCreateEpisodeStatus(f)
		localStat.Status = StateQueuedRemote
		if priority > 0 {
			localStat.Priority = priority
		}
		_ = saveEpisodeStatus(statusPathFor(f), localStat)

		remoteStat := *localStat
		remoteStat.Status = StateAwaitingTranscription
		remoteStat.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if priority > 0 {
			remoteStat.Priority = priority
		}

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

	if !quiet {
		fmt.Println()
		fmt.Printf("Successfully pushed %d episode(s) to %s:%s.\n", pushedCount, targetHost, remoteWorkDir)
		fmt.Printf("  - Check status: abs remote status %s\n", targetHost)
		fmt.Printf("  - Pull results: abs remote pull %s\n", targetHost)
	}

	return ensureRemoteEnvironmentAndWorker(cfg, targetHost, remoteWorkDir, transport, quiet)
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

	return nil
}
