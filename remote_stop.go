package main

import (
	"fmt"
	"strings"
)

func runRemoteStop(cfg *Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote stop requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
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
