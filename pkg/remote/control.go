package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/pipeline"
	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

func RunRemoteStop(cfg *types.Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote stop requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = GetRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	lockData, _ := transport.Exec(targetHost, fmt.Sprintf("cat %s/.worker.lock 2>/dev/null", remoteWorkDir))
	var lockPID int
	if strings.TrimSpace(lockData) != "" {
		lines := util.SplitLines(strings.TrimSpace(lockData))
		if len(lines) > 0 {
			_, _ = fmt.Sscanf(lines[0], "%d", &lockPID)
		}
	}
	if lockPID > 0 {
		killPidCmd := fmt.Sprintf("kill -9 %d 2>/dev/null || kill %d 2>/dev/null || true", lockPID, lockPID)
		_, _ = transport.Exec(targetHost, killPidCmd)
	}

	killWorkerCmd := "pkill -9 -f 'abs.*(scan|worker|batch-worker)' 2>/dev/null || pkill -f 'abs.*(scan|worker|batch-worker)' 2>/dev/null || true; pkill -9 -f 'ffmpeg.*abs' 2>/dev/null || pkill -f 'ffmpeg.*abs' 2>/dev/null || true"
	_, _ = transport.Exec(targetHost, killWorkerCmd)

	cleanLockCmd := fmt.Sprintf("rm -f %s/.worker.lock %s/.scan_trigger", remoteWorkDir, remoteWorkDir)
	_, _ = transport.Exec(targetHost, cleanLockCmd)

	resetStatusCmd := fmt.Sprintf("sed -i 's/\"status\": \"transcribing_remotely\"/\"status\": \"awaiting_transcription\"/g' %s/*/*.mp3.json 2>/dev/null; sed -i 's/\"status\": \"cutting_remotely\"/\"status\": \"awaiting_transcription\"/g' %s/*/*.mp3.json 2>/dev/null; rm -rf %s/*/.work", remoteWorkDir, remoteWorkDir, remoteWorkDir)
	_, _ = transport.Exec(targetHost, resetStatusCmd)

	dockerStopCmds := []string{
		"docker stop whisper.cpp-server 2>/dev/null || true",
	}
	if cfg != nil && cfg.WhisperDockerContainer != "" && cfg.WhisperDockerContainer != "whisper.cpp-server" {
		dockerStopCmds = append(dockerStopCmds, fmt.Sprintf("docker stop %s 2>/dev/null || true", cfg.WhisperDockerContainer))
	}
	dockerStopCmds = append(dockerStopCmds,
		"docker stop $(docker ps -q --filter 'name=whisper' 2>/dev/null || true) 2>/dev/null || true",
		"docker stop $(docker ps -q --filter 'ancestor=fedirz/faster-whisper-server' 2>/dev/null || true) 2>/dev/null || true",
		"docker stop $(docker ps -q --filter 'ancestor=whisper.cpp-server' 2>/dev/null || true) 2>/dev/null || true",
	)
	for _, dcmd := range dockerStopCmds {
		_, _ = transport.Exec(targetHost, dcmd)
	}

	killWhisperCmd := "pkill -9 -f 'whisper-server' 2>/dev/null || pkill -f 'whisper-server' 2>/dev/null || true; pkill -9 -f 'whisper.cpp' 2>/dev/null || pkill -f 'whisper.cpp' 2>/dev/null || true; pkill -9 -f 'whisper_server' 2>/dev/null || pkill -f 'whisper_server' 2>/dev/null || true; pkill -9 -f 'faster-whisper-server' 2>/dev/null || pkill -f 'faster-whisper-server' 2>/dev/null || true"
	_, _ = transport.Exec(targetHost, killWhisperCmd)

	if !quiet {
		fmt.Printf("Stopped remote worker process and Whisper server on %s.\n", targetHost)
	}

	return nil
}

func RunRemoteCancel(cfg *types.Config, host, batchID string, transport RemoteTransport, quiet bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote cancel requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = GetRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}
	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)

	if batchID != "" {
		if !ValidateBatchID(batchID) {
			return fmt.Errorf("invalid batch ID format: %q", batchID)
		}
		tempDir := filepath.Join(os.TempDir(), "abs_cancel", batchID)
		_ = os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		manPath := filepath.Join(tempDir, "manifest.json")
		remoteMan := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)

		if err := transport.Download(targetHost, remoteMan, manPath); err == nil {
			if m, err := LoadManifest(manPath); err == nil {
				m.Status = types.BatchStatusCancelled
				m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				_ = SaveManifest(manPath, m)
				_ = transport.Upload(targetHost, manPath, remoteMan)
			}
		}

		killCmd := fmt.Sprintf("pkill -f %s 2>/dev/null || true", ShellQuote("batch-worker.*"+batchID))
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

func RunRemoteClear(cfg *types.Config, host string, transport RemoteTransport, quiet bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote clear requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = GetRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	killCmd := fmt.Sprintf("pkill -f 'abs.*(scan|worker|batch-worker)' 2>/dev/null || true; pkill -f 'ffmpeg.*abs' 2>/dev/null || true; pkill -f 'whisper-server' 2>/dev/null || true; pkill -f 'whisper_server' 2>/dev/null || true; rm -f %s/.worker.lock %s/.scan_trigger", remoteWorkDir, remoteWorkDir)
	_, _ = transport.Exec(targetHost, killCmd)

	_, _ = transport.Exec(targetHost, whisperDockerRestartCommand())

	findPendingCmd := fmt.Sprintf("grep -l -E '\"status\": \"(awaiting_transcription|transcribing_remotely|cutting_remotely|queued_remote)\"' %s/*/*.mp3.json 2>/dev/null", remoteWorkDir)
	pendingOut, _ := transport.Exec(targetHost, findPendingCmd)
	removedCount := 0

	if strings.TrimSpace(pendingOut) != "" {
		lines := util.SplitLines(strings.TrimSpace(pendingOut))
		for _, statPath := range lines {
			statPath = strings.TrimSpace(statPath)
			if statPath == "" {
				continue
			}
			audioPath := strings.TrimSuffix(statPath, ".json")
			basePath := util.StripExt(audioPath)
			delCmd := fmt.Sprintf("rm -f %s %s %s %s",
				ShellQuoteHomePath(audioPath),
				ShellQuoteHomePath(statPath),
				ShellQuoteHomePath(basePath+".cuts.json"),
				ShellQuoteHomePath(basePath+".transcript.json"))
			_, _ = transport.Exec(targetHost, delCmd)
			removedCount++
		}
	}

	cleanWorkCmd := fmt.Sprintf("rm -rf %s/*/.work %s/staging/* 2>/dev/null || true", remoteWorkDir, remoteWorkDir)
	_, _ = transport.Exec(targetHost, cleanWorkCmd)

	localResetCount := 0
	if cfg != nil && cfg.PodcastsDir != "" {
		mp3s := util.FindMP3Files(cfg.PodcastsDir)
		for _, mp3 := range mp3s {
			stat := pipeline.GetOrCreateEpisodeStatus(mp3)
			if stat.Status == types.StateQueuedRemote || stat.Status == types.StateTranscribingRemotely || stat.Status == types.StateCuttingRemotely || stat.Status == types.StateAwaitingTranscription {
				stat.Status = types.StateDownloaded
				stat.WorkerHost = ""
				_ = pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(mp3), stat)
				localResetCount++
			}
		}
	}

	if !quiet {
		fmt.Printf("Cleared remote queue on %s: removed %d queued episode(s), reset %d local status file(s).\n", targetHost, removedCount, localResetCount)
	}

	return nil
}

func whisperDockerRestartCommand() string {
	return "ids=$(docker ps -q --filter 'ancestor=fedirz/faster-whisper-server' 2>/dev/null); " +
		"if [ -n \"$ids\" ]; then docker restart $ids 2>/dev/null || true; fi"
}
