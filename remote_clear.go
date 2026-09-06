package main

import (
	"fmt"
	"strings"
)

func runRemoteClear(cfg *Config, host string, transport RemoteTransport, quiet bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote clear requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
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
		lines := splitLines(strings.TrimSpace(pendingOut))
		for _, statPath := range lines {
			statPath = strings.TrimSpace(statPath)
			if statPath == "" {
				continue
			}
			audioPath := strings.TrimSuffix(statPath, ".json")
			basePath := stripExt(audioPath)
			delCmd := fmt.Sprintf("rm -f %s %s %s %s",
				shellQuoteHomePath(audioPath),
				shellQuoteHomePath(statPath),
				shellQuoteHomePath(basePath+".cuts.json"),
				shellQuoteHomePath(basePath+".transcript.json"))
			_, _ = transport.Exec(targetHost, delCmd)
			removedCount++
		}
	}

	cleanWorkCmd := fmt.Sprintf("rm -rf %s/*/.work %s/staging/* 2>/dev/null || true", remoteWorkDir, remoteWorkDir)
	_, _ = transport.Exec(targetHost, cleanWorkCmd)

	localResetCount := 0
	if cfg != nil && cfg.PodcastsDir != "" {
		mp3s := findMP3Files(cfg.PodcastsDir)
		for _, mp3 := range mp3s {
			stat := getOrCreateEpisodeStatus(mp3)
			if stat.Status == StateQueuedRemote || stat.Status == StateTranscribingRemotely || stat.Status == StateCuttingRemotely || stat.Status == StateAwaitingTranscription {
				stat.Status = StateDownloaded
				stat.WorkerHost = ""
				_ = saveEpisodeStatus(statusPathFor(mp3), stat)
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
