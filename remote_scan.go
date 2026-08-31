package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func resolveLocalPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func scanAudioFiles(rootDir string) []string {
	var files []string
	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".work" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".mp3") && !strings.HasSuffix(info.Name(), ".tmp.mp3") {
			if info.Size() > 0 {
				files = append(files, path)
			}
		}
		return nil
	})
	return files
}

func runRemoteScan(cfg *Config, targetDir string, ifDirty bool, quiet, verbose bool) error {
	remoteDir := targetDir
	if remoteDir == "" {
		if cfg != nil && cfg.RemoteWorkDir != "" {
			remoteDir = cfg.RemoteWorkDir
		} else {
			remoteDir = "~/abs_remote"
		}
	}
	resolvedDir := resolveLocalPath(remoteDir)
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("failed to create scan directory %s: %w", resolvedDir, err)
	}

	triggerPath := filepath.Join(resolvedDir, ".scan_trigger")
	if ifDirty {
		if !fileExists(triggerPath) {
			return nil
		}
	}
	_ = os.Remove(triggerPath)

	unlock, err := acquireWorkerLock(resolvedDir)
	if err != nil {
		return err
	}
	defer unlock()

	files := scanAudioFiles(resolvedDir)
	if len(files) == 0 {
		if !quiet {
			fmt.Printf("Scan complete: No pending audio files found in %s.\n", resolvedDir)
		}
		return nil
	}

	if cfg == nil {
		c := loadConfig()
		cfg = &c
	}
	profile := selectProfile(*cfg, "")
	donePath := filepath.Join(resolvedDir, "done.json")

	processedCount := 0
	hostname, _ := os.Hostname()

	for _, audioFile := range files {
		statPath := statusPathFor(audioFile)
		st, _ := loadEpisodeStatus(statPath)
		if st != nil && (st.Status == StateReadyForCopyBack || st.Status == StateDone || st.Status == StateArchived || st.Status == StateCopiedBack) {
			continue
		}

		relPath, _ := filepath.Rel(resolvedDir, audioFile)
		if relPath == "" || strings.HasPrefix(relPath, "..") {
			relPath = filepath.Base(audioFile)
		}

		origDuration := getAudioDuration(audioFile)
		if origDuration <= 0 {
			origDuration = getMP3DiskDuration(audioFile)
		}
		origSize := int64(0)
		if fi, err := os.Stat(audioFile); err == nil {
			origSize = fi.Size()
		}

		if st == nil {
			st = &EpisodeStatusFile{
				Version:   1,
				MediaFile: filepath.Base(audioFile),
				Status:    StateAwaitingTranscription,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				Original: EpisodeAudioMeta{
					Filename:    filepath.Base(audioFile),
					DurationSec: origDuration,
					SizeBytes:   origSize,
				},
			}
		}

		st.Status = StateTranscribingRemotely
		st.WorkerHost = hostname
		_ = saveEpisodeStatus(statPath, st)

		if !quiet {
			fmt.Printf("Scanning & Processing: %s (Duration: %.1fs)\n", relPath, origDuration)
		}

		baseName := stripExt(audioFile)
		transcriptJSON := baseName + ".transcript.json"
		speedFactor := cfg.WhisperSpeedFactor
		if speedFactor <= 0 {
			speedFactor = 7.0
		}

		t0 := time.Now()
		isNewlyTranscribed := false
		cliOpts := CLIOptions{
			SaveTranscript: true,
			Quiet:          quiet,
			Verbose:        verbose,
		}

		transData, err := loadOrTranscribe(audioFile, transcriptJSON, *cfg, cliOpts, profile, origDuration, speedFactor, cfg.WhisperLanguage, cfg.WhisperPrompt, map[string]string{}, &isNewlyTranscribed, &t0)
		if err != nil {
			st.Status = StateFailed
			st.LastError = fmt.Sprintf("transcription error: %v", err)
			_ = saveEpisodeStatus(statPath, st)
			continue
		}

		saveJSONTranscript(audioFile, transData, transcriptJSON, quiet, map[string]string{})

		formattedTranscript := formatTranscript(transData, origDuration)
		adSegments := detectAdsLLM(formattedTranscript, profile)
		if len(adSegments) > 0 {
			adSegments = mergeIntervals(adSegments)
		}

		st.Ads = make([]EpisodeAdCut, 0, len(adSegments))
		for _, ad := range adSegments {
			st.Ads = append(st.Ads, EpisodeAdCut{
				Start:  ad.Start,
				End:    ad.End,
				Reason: ad.Reason,
			})
		}

		cutsResult := saveCutsJSON(audioFile, origDuration, adSegments, &profile, quiet)
		keepSegments := cutsResult.KeepSegments

		cleanDuration := origDuration
		cleanSize := origSize

		if len(adSegments) > 0 && len(keepSegments) > 0 {
			st.Status = StateCuttingRemotely
			_ = saveEpisodeStatus(statPath, st)

			workDir := workDirFor(audioFile)
			_ = os.MkdirAll(workDir, 0755)
			tempOut := filepath.Join(workDir, filepath.Base(audioFile)+".tmp.mp3")
			verifyTempFile(tempOut)

			if cutAudioFFmpeg(audioFile, keepSegments, tempOut) {
				precutPath := audioFile + ".precut"
				safeMove(audioFile, precutPath)
				safeMove(tempOut, audioFile)
				cleanDuration = getAudioDuration(audioFile)
				if fi, err := os.Stat(audioFile); err == nil {
					cleanSize = fi.Size()
				}
				st.Original.Filename = filepath.Base(precutPath)
			}
			_ = os.RemoveAll(workDir)
		}

		st.Cleaned = EpisodeAudioMeta{
			Filename:      filepath.Base(audioFile),
			DurationSec:   cleanDuration,
			SizeBytes:     cleanSize,
			AdDurationSec: origDuration - cleanDuration,
		}
		st.Status = StateReadyForCopyBack
		st.LastError = ""
		_ = saveEpisodeStatus(statPath, st)

		doneItem := RemoteDoneItem{
			RelPath:             relPath,
			Status:              StateReadyForCopyBack,
			OriginalDurationSec: origDuration,
			CleanedDurationSec:  cleanDuration,
			CutDurationSec:      origDuration - cleanDuration,
			OriginalSizeBytes:   origSize,
			CleanedSizeBytes:    cleanSize,
			CompletedAt:         time.Now().UTC().Format(time.RFC3339),
			WorkerHost:          hostname,
		}
		_ = addDoneEpisode(donePath, doneItem)
		processedCount++

		if !quiet {
			fmt.Printf("✓ Finished %s (Saved %.1fs, Ready for copy back)\n", relPath, origDuration-cleanDuration)
		}
	}

	if !quiet {
		fmt.Printf("\nScan completed. Processed and queued %d episode(s) in %s/done.json.\n", processedCount, resolvedDir)
	}
	return nil
}

func runRemoteWorkerLoop(cfg *Config, targetDir string, daemon bool, quiet, verbose bool) error {
	remoteDir := targetDir
	if remoteDir == "" {
		if cfg != nil && cfg.RemoteWorkDir != "" {
			remoteDir = cfg.RemoteWorkDir
		} else {
			remoteDir = "~/abs_remote"
		}
	}
	resolvedDir := resolveLocalPath(remoteDir)
	_ = os.MkdirAll(resolvedDir, 0755)

	unlock, err := acquireWorkerLock(resolvedDir)
	if err != nil {
		return err
	}
	defer unlock()

	for {
		if err := runRemoteScan(cfg, resolvedDir, false, quiet, verbose); err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "Worker error: %v\n", err)
			}
		}
		if !daemon {
			break
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}
