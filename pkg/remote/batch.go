package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"abs/pkg/audio"
	"abs/pkg/backend"
	"abs/pkg/format"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/types"
	"abs/pkg/util"
)

func FindAudioFilesForRemote(paths []string, defaultDir string) []string {
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
			mp3s := util.FindMP3Files(p)
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
		} else if strings.HasSuffix(strings.ToLower(p), ".mp3") {
			absP, err := filepath.Abs(p)
			if err == nil && !seen[absP] {
				seen[absP] = true
				results = append(results, absP)
			}
		}
	}
	return results
}

func SortAudioFilesByQueuePolicy(files []string, now time.Time) {
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
		durMap[f] = GetEpisodeDurationForQueue(f)
		priMap[f] = GetEpisodePriorityForQueue(f)
		isRec, pt := IsEpisodeRecent24h(f, now)
		pubMap[f] = pt
		recentMap[f] = isRec
	}

	sort.SliceStable(files, func(i, j int) bool {
		fi, fj := files[i], files[j]
		if pi, pj := priMap[fi], priMap[fj]; pi != pj {
			return pi > pj
		}
		recI, recJ := recentMap[fi], recentMap[fj]
		if recI != recJ {
			return recI && !recJ
		}
		if recI {
			pubI, pubJ := pubMap[fi], pubMap[fj]
			if !pubI.Equal(pubJ) {
				return pubI.After(pubJ)
			}
			di, dj := durMap[fi], durMap[fj]
			if di != dj {
				return di < dj
			}
			return fi < fj
		}
		di, dj := durMap[fi], durMap[fj]
		if di != dj {
			return di < dj
		}
		pubI, pubJ := pubMap[fi], pubMap[fj]
		if !pubI.Equal(pubJ) {
			return pubI.After(pubJ)
		}
		return fi < fj
	})
}

func SortAudioFilesByDuration(files []string) {
	SortAudioFilesByQueuePolicy(files, time.Now())
}

func GetEpisodeDurationForQueue(audioPath string) float64 {
	statPath := pipeline.StatusPathFor(audioPath)
	if st, err := pipeline.LoadEpisodeStatus(statPath); err == nil && st != nil {
		if st.Original.DurationSec > 0 {
			return st.Original.DurationSec
		}
	}
	dur := audio.GetAudioDuration(audioPath)
	if dur <= 0 {
		dur = backend.GetMP3DiskDuration(audioPath)
	}
	if dur <= 0 {
		if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
			return float64(fi.Size()) / 16000.0
		}
	}
	return dur
}

func GetEpisodePriorityForQueue(audioPath string) int {
	statPath := pipeline.StatusPathFor(audioPath)
	if st, err := pipeline.LoadEpisodeStatus(statPath); err == nil && st != nil {
		return st.Priority
	}
	return 0
}

func IsEpisodeRecent24h(audioPath string, now time.Time) (bool, time.Time) {
	pt := podcast.GetEpisodePublicationTime(audioPath)
	if pt.IsZero() {
		return false, pt
	}
	cutoff := now.Add(-24 * time.Hour)
	return pt.After(cutoff), pt
}

func ComputeRelativeMediaDir(baseDir, fullPath string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(fullPath), nil
	}
	return rel, nil
}

func PushSingleAudioFile(f, defaultDir, remoteWorkDir, targetHost string, priority int, transport RemoteTransport) error {
	relPath, _ := ComputeRelativeMediaDir(defaultDir, f)
	remoteDstDir := fmt.Sprintf("%s/%s", remoteWorkDir, filepath.Dir(relPath))
	remoteDstFile := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
	remoteDstStatus := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)

	mkdirCmd := fmt.Sprintf("mkdir -p %s", ShellQuoteHomePath(remoteDstDir))
	_, _ = transport.Exec(targetHost, mkdirCmd)

	localStat := pipeline.GetOrCreateEpisodeStatus(f)
	remoteStat := *localStat
	remoteStat.Status = types.StateAwaitingTranscription
	remoteStat.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if priority > 0 {
		remoteStat.Priority = priority
	}

	workDir := util.WorkDirFor(f)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory for %s: %w", f, err)
	}
	tmpStatPath := filepath.Join(workDir, fmt.Sprintf("rem_stat_%d.json", time.Now().UnixNano()))
	util.VerifyTempFile(tmpStatPath)

	if err := pipeline.SaveEpisodeStatus(tmpStatPath, &remoteStat); err != nil {
		return fmt.Errorf("failed to save staging status for %s: %w", f, err)
	}
	defer os.Remove(tmpStatPath)

	if err := transport.Upload(targetHost, f, remoteDstFile); err != nil {
		return fmt.Errorf("failed to upload audio %s to %s: %w", f, targetHost, err)
	}
	if err := transport.Upload(targetHost, tmpStatPath, remoteDstStatus); err != nil {
		return fmt.Errorf("failed to upload status file for %s to %s: %w", f, targetHost, err)
	}

	localStat.Status = types.StateQueuedRemote
	if priority > 0 {
		localStat.Priority = priority
	}
	if err := pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(f), localStat); err != nil {
		return fmt.Errorf("failed to save local status for %s: %w", f, err)
	}
	return nil
}

func RunRemotePush(cfg *types.Config, args []string, host string, transport RemoteTransport, priority int, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote push requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = GetRemoteTransport()
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
			st := pipeline.GetOrCreateEpisodeStatus(f)
			st.Priority = priority
			_ = pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(f), st)
		}
	}

	SortAudioFilesByDuration(toPush)

	if !quiet {
		if priority > 0 {
			fmt.Printf("Pushing %d audio file(s) [priority: %d] to mirror directory on %s:%s...\n", len(toPush), priority, targetHost, remoteWorkDir)
		} else {
			fmt.Printf("Pushing %d audio file(s) to mirror directory on %s:%s...\n", len(toPush), targetHost, remoteWorkDir)
		}
	}

	pushedCount := 0
	for _, f := range toPush {
		if err := PushSingleAudioFile(f, defaultDir, remoteWorkDir, targetHost, priority, transport); err != nil {
			return err
		}
		pushedCount++
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Successfully pushed %d episode(s) to %s:%s.\n", pushedCount, targetHost, remoteWorkDir)
		fmt.Printf("  - Check status: abs offload status %s\n", targetHost)
		fmt.Printf("  - Pull results: abs offload pull %s\n", targetHost)
	}

	return EnsureRemoteEnvironmentAndWorker(cfg, targetHost, remoteWorkDir, transport, quiet)
}

func resolveFilesToPush(args []string, defaultDir string, quiet bool) ([]string, error) {
	files := FindAudioFilesForRemote(args, defaultDir)
	if len(files) == 0 {
		return nil, fmt.Errorf("no audio (.mp3) files found to push for batch processing")
	}

	var toPush []string
	for _, f := range files {
		if !pipeline.IsEpisodeCompleted(f) && !pipeline.IsEpisodeInRemoteFlight(f) {
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

func EnsureRemoteEnvironmentAndWorker(cfg *types.Config, targetHost, remoteWorkDir string, transport RemoteTransport, quiet bool) error {
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return nil
	}
	if transport == nil {
		transport = GetRemoteTransport()
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

	if !IsRemoteHostReachable(targetHost, transport) {
		return fmt.Errorf("remote host '%s' is unreachable via SSH", targetHost)
	}

	checkWhisperServiceReadiness(cfg, targetHost, transport, quiet)
	startRemoteWorkerProcess(targetHost, remoteWorkDir, transport, quiet)
	verifyRemoteWorkerStartup(targetHost, remoteWorkDir, transport, quiet)
	return nil
}

func checkWhisperServiceReadiness(cfg *types.Config, targetHost string, transport RemoteTransport, quiet bool) {
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

	workerCmd := fmt.Sprintf("nohup ~/.local/bin/abs offload scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, workerCmd); err != nil {
		altCmd := fmt.Sprintf("nohup abs offload scan %s < /dev/null > %s/worker.log 2>&1 &", remoteWorkDir, remoteWorkDir)
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
		activeFiles := util.SplitLines(strings.TrimSpace(activeOut))
		if len(activeFiles) > 0 && activeFiles[0] != "" {
			activeJsonPath := activeFiles[0]
			activeTaskName = cleanRemoteRelPath(activeJsonPath, remoteWorkDir)
			if data, err := transport.Exec(targetHost, fmt.Sprintf("cat %s", ShellQuoteHomePath(activeJsonPath))); err == nil && data != "" {
				var st types.EpisodeStatusFile
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
				durStr = format.FormatClock(activeTaskDuration)
			}
			fmt.Printf("[✓] Remote worker is active on %s: converting '%s' (Length: %s)\n", targetHost, activeTaskName, durStr)
		} else if workerStarted {
			fmt.Printf("[✓] Remote worker successfully started and running on %s.\n", targetHost)
		} else {
			fmt.Printf("[-] Notice: Remote worker was launched on %s (check status with: abs offload status %s).\n", targetHost, targetHost)
		}
	}
}

func cleanRemoteRelPath(p, remoteWorkDir string) string {
	clean := p
	if remoteWorkDir != "" {
		clean = strings.TrimPrefix(clean, remoteWorkDir+"/")
	}
	clean = strings.TrimSuffix(clean, ".json")
	return clean
}

func wakeWhisperServer(whisperURL string, wakeCmd string, quiet bool) {
	if whisperURL == "" {
		return
	}
	if wakeCmd != "" {
		if !quiet {
			fmt.Printf("Running whisper wake command: %s\n", wakeCmd)
		}
		cmd := exec.Command("/bin/sh", "-c", wakeCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil && !quiet {
			fmt.Printf("Warning: Whisper wake command failed: %v\n", err)
		}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", whisperURL, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
