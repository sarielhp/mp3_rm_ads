package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"abs/pkg/audio"
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/detect"
	"abs/pkg/format"
	"abs/pkg/pipeline"
	"abs/pkg/types"
	"abs/pkg/util"
)

func ResolveLocalPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func ScanAudioFiles(rootDir string) []string {
	var files []string
	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".work" || strings.HasPrefix(info.Name(), ".") || strings.HasSuffix(info.Name(), "-1") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".mp3") && !strings.HasSuffix(info.Name(), ".tmp.mp3") {
			if info.Name() != "podcast.mp3" && !strings.Contains(path, "-1/") {
				if info.Size() > 0 {
					files = append(files, path)
				}
			}
		}
		return nil
	})
	return files
}

func RunRemoteScan(cfg *types.Config, targetDir string, ifDirty bool, quiet, verbose bool) error {
	remoteDir := targetDir
	if remoteDir == "" {
		if cfg != nil && cfg.RemoteWorkDir != "" {
			remoteDir = cfg.RemoteWorkDir
		} else {
			remoteDir = "~/abs_remote"
		}
	}
	resolvedDir := ResolveLocalPath(remoteDir)
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("failed to create scan directory %s: %w", resolvedDir, err)
	}

	triggerPath := filepath.Join(resolvedDir, ".scan_trigger")
	if ifDirty && !util.FileExists(triggerPath) {
		return nil
	}
	_ = os.Remove(triggerPath)

	unlock, err := util.AcquireWorkerLock(resolvedDir)
	if err != nil {
		return err
	}
	defer unlock()

	if cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
	}
	profile, _ := config.SelectLLMProfile(cfg, "")
	donePath := filepath.Join(resolvedDir, "done.json")

	processedCount := 0
	hostname, _ := os.Hostname()

	for {
		pendingFiles := collectPendingAudioFiles(resolvedDir)
		if len(pendingFiles) == 0 {
			if !quiet && processedCount == 0 {
				fmt.Printf("Scan complete: No pending audio files found in %s.\n", resolvedDir)
			}
			break
		}

		SortAudioFilesByDuration(pendingFiles)
		audioFile := pendingFiles[0]

		if processSingleScannedAudio(audioFile, resolvedDir, hostname, donePath, cfg, profile, quiet, verbose) {
			processedCount++
		}
	}

	if !quiet {
		fmt.Printf("\nScan completed. Processed and queued %d episode(s) in %s/done.json.\n", processedCount, resolvedDir)
	}
	return nil
}

func collectPendingAudioFiles(resolvedDir string) []string {
	files := ScanAudioFiles(resolvedDir)
	var pendingFiles []string
	for _, audioFile := range files {
		statPath := pipeline.StatusPathFor(audioFile)
		st, _ := pipeline.LoadEpisodeStatus(statPath)
		if st != nil && (st.Status == types.StateReadyForCopyBack || st.Status == types.StateDone || st.Status == types.StateArchived || st.Status == types.StateCopiedBack || st.Status == types.StateFailed) {
			continue
		}
		pendingFiles = append(pendingFiles, audioFile)
	}
	return pendingFiles
}

func processSingleScannedAudio(audioFile, resolvedDir, hostname, donePath string, cfg *types.Config, profile types.LLMProfile, quiet, verbose bool) bool {
	statPath := pipeline.StatusPathFor(audioFile)
	st, _ := pipeline.LoadEpisodeStatus(statPath)

	relPath, _ := filepath.Rel(resolvedDir, audioFile)
	if relPath == "" || strings.HasPrefix(relPath, "..") {
		relPath = filepath.Base(audioFile)
	}

	origDuration := audio.GetAudioDuration(audioFile)
	if origDuration <= 0 {
		dur := backend.GetMP3DiskDuration(audioFile)
		origDuration = dur
	}
	origSize := int64(0)
	if fi, err := os.Stat(audioFile); err == nil {
		origSize = fi.Size()
	}

	st = initRemoteScanStatus(audioFile, hostname, origDuration, origSize, st, statPath)
	if !quiet {
		fmt.Printf("Scanning & Processing: %s (Length: %s, %.1fs)\n", relPath, format.FormatClock(origDuration), origDuration)
	}

	_, adSegments, err := transcribeAndDetectAdsRemoteScan(audioFile, origDuration, cfg, profile, quiet, verbose)
	if err != nil {
		st.Status = types.StateFailed
		st.LastError = fmt.Sprintf("transcription error: %v", err)
		_ = pipeline.SaveEpisodeStatus(statPath, st)
		return false
	}

	st.Ads = make([]types.EpisodeAdCut, 0, len(adSegments))
	for _, ad := range adSegments {
		st.Ads = append(st.Ads, types.EpisodeAdCut(ad))
	}

	cutsResult := format.SaveCutsJSON(audioFile, origDuration, adSegments, &profile, quiet)
	cleanDuration, cleanSize := applyRemoteScanAudioCut(audioFile, cutsResult.KeepSegments, origDuration, origSize, st, statPath)

	st.Cleaned = types.EpisodeAudioMeta{
		Filename:      filepath.Base(audioFile),
		DurationSec:   cleanDuration,
		SizeBytes:     cleanSize,
		AdDurationSec: origDuration - cleanDuration,
	}
	st.Status = types.StateReadyForCopyBack
	st.LastError = ""
	_ = pipeline.SaveEpisodeStatus(statPath, st)

	doneItem := RemoteDoneItem{
		RelPath:             relPath,
		Status:              types.StateReadyForCopyBack,
		OriginalDurationSec: origDuration,
		CleanedDurationSec:  cleanDuration,
		CutDurationSec:      origDuration - cleanDuration,
		OriginalSizeBytes:   origSize,
		CleanedSizeBytes:    cleanSize,
		CompletedAt:         time.Now().UTC().Format(time.RFC3339),
		WorkerHost:          hostname,
	}
	_ = AddDoneEpisode(donePath, doneItem)

	if !quiet {
		fmt.Printf("✓ Finished %s (Saved %.1fs, Ready for copy back)\n", relPath, origDuration-cleanDuration)
	}
	return true
}

func initRemoteScanStatus(audioFile, hostname string, origDuration float64, origSize int64, st *types.EpisodeStatusFile, statPath string) *types.EpisodeStatusFile {
	if st == nil {
		st = &types.EpisodeStatusFile{
			Version:   1,
			MediaFile: filepath.Base(audioFile),
			Status:    types.StateAwaitingTranscription,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			Original: types.EpisodeAudioMeta{
				Filename:    filepath.Base(audioFile),
				DurationSec: origDuration,
				SizeBytes:   origSize,
			},
		}
	}
	st.Status = types.StateTranscribingRemotely
	st.CurrentStep = "Step 1/3: Whisper Transcription"
	st.StepStartedAt = time.Now().UTC().Format(time.RFC3339)
	st.WorkerHost = hostname
	st.Original.DurationSec = origDuration
	_ = pipeline.SaveEpisodeStatus(statPath, st)
	return st
}

func transcribeAndDetectAdsRemoteScan(audioFile string, origDuration float64, cfg *types.Config, profile types.LLMProfile, quiet, verbose bool) (*types.TranscriptionData, []types.AdSegment, error) {
	transcriptJSON := util.StripExt(audioFile) + ".transcript.json"
	speedFactor := cfg.WhisperSpeedFactor
	if speedFactor <= 0 {
		speedFactor = 4.5
	}

	t0 := time.Now()
	isNewlyTranscribed := false

	cli := types.CLIOptions{
		ProcOptions: types.ProcOptions{
			Quiet:          quiet,
			Verbose:        verbose,
			SaveTranscript: true,
		},
	}
	transData, err := pipeline.LoadOrTranscribe(audioFile, transcriptJSON, *cfg, cli, profile, origDuration, speedFactor, cfg.WhisperLanguage, cfg.WhisperPrompt, map[string]string{}, &isNewlyTranscribed, &t0)
	if err != nil {
		return nil, nil, err
	}

	_ = pipeline.SaveJSONTranscript(audioFile, transData, transcriptJSON, quiet, map[string]string{})

	formattedTranscript := pipeline.FormatTranscript(transData, origDuration)
	adSegments, err := detect.DetectAdsLLM(formattedTranscript, profile, profile.APIKey)
	if err != nil {
		return nil, nil, fmt.Errorf("ad detection failed: %w", err)
	}
	if len(adSegments) > 0 {
		adSegments = format.MergeIntervals(adSegments)
	}
	return transData, adSegments, nil
}

func applyRemoteScanAudioCut(audioFile string, keepSegments [][2]float64, origDuration float64, origSize int64, st *types.EpisodeStatusFile, statPath string) (float64, int64) {
	cleanDuration := origDuration
	cleanSize := origSize

	if len(keepSegments) == 0 {
		return cleanDuration, cleanSize
	}

	st.Status = types.StateCuttingRemotely
	st.CurrentStep = "Step 3/3: FFmpeg Audio Cutting"
	st.StepStartedAt = time.Now().UTC().Format(time.RFC3339)
	_ = pipeline.SaveEpisodeStatus(statPath, st)

	workDir := util.WorkDirFor(audioFile)
	_ = os.MkdirAll(workDir, 0755)
	tempOut := filepath.Join(workDir, filepath.Base(audioFile)+".tmp.mp3")
	util.VerifyTempFile(tempOut)

	if audio.CutAudioFFmpeg(audioFile, keepSegments, tempOut) {
		precutPath := audioFile + ".precut"
		installed := true
		if !util.FileExists(precutPath) {
			if mvErr := util.SafeMove(audioFile, precutPath); mvErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not preserve the original for %s: %v\n", audioFile, mvErr)
				installed = false
			}
		}
		if installed {
			if mvErr := util.SafeMove(tempOut, audioFile); mvErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not install the cut audio for %s: %v\n", audioFile, mvErr)
			} else {
				cleanDuration = audio.GetAudioDuration(audioFile)
				if fi, err := os.Stat(audioFile); err == nil {
					cleanSize = fi.Size()
				}
				st.Original.Filename = filepath.Base(precutPath)
			}
		}
	}
	_ = os.RemoveAll(workDir)
	return cleanDuration, cleanSize
}

func RunRemoteWorkerLoop(cfg *types.Config, targetDir string, daemon bool, quiet, verbose bool) error {
	remoteDir := targetDir
	if remoteDir == "" {
		if cfg != nil && cfg.RemoteWorkDir != "" {
			remoteDir = cfg.RemoteWorkDir
		} else {
			remoteDir = "~/abs_remote"
		}
	}
	resolvedDir := ResolveLocalPath(remoteDir)
	_ = os.MkdirAll(resolvedDir, 0755)

	unlock, err := util.AcquireWorkerLock(resolvedDir)
	if err != nil {
		return err
	}
	defer unlock()

	for {
		if err := RunRemoteScan(cfg, resolvedDir, false, quiet, verbose); err != nil {
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
