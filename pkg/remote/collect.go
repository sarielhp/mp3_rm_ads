package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/backend"
	"github.com/sariel/abs/pkg/format"
	"github.com/sariel/abs/pkg/pipeline"
	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

func RunRemoteAck(remoteDir string, relPaths []string) error {
	resolvedDir := ResolveLocalPath(remoteDir)
	donePath := filepath.Join(resolvedDir, "done.json")
	archPath := filepath.Join(resolvedDir, "archive.json")

	for _, relPath := range relPaths {
		if strings.TrimSpace(relPath) == "" {
			continue
		}
		audioPath := filepath.Join(resolvedDir, relPath)

		_ = os.Remove(audioPath)
		_ = os.Remove(audioPath + ".precut")
		_ = os.Remove(audioPath + ".tmp.mp3")

		statPath := pipeline.StatusPathFor(audioPath)
		if st, err := pipeline.LoadEpisodeStatus(statPath); err == nil && st != nil {
			st.Status = types.StateArchived
			_ = pipeline.SaveEpisodeStatus(statPath, st)
		}

		_ = archiveDoneEpisode(donePath, archPath, relPath)
	}
	return nil
}

func archiveDoneEpisode(donePath, archPath, relPath string) error {
	doneM, err := LoadDoneManifest(donePath)
	if err != nil {
		return err
	}
	item, ok := doneM.Episodes[relPath]
	if !ok {
		return nil
	}
	delete(doneM.Episodes, relPath)
	_ = SaveDoneManifest(donePath, doneM)

	item.Status = types.StateArchived
	_ = AddDoneEpisode(archPath, item)
	return nil
}

func RunRemotePull(cfg *types.Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote pull requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = GetRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	localPodcastsDir := ""
	if cfg != nil {
		localPodcastsDir = cfg.PodcastsDir
	}
	if localPodcastsDir == "" {
		localPodcastsDir = "."
	}

	collectLock, err := util.AcquireCollectLock(localPodcastsDir)
	if err != nil || collectLock == nil {
		if verbose {
			fmt.Println("Another collect operation is currently in progress; skipping.")
		}
		return nil
	}
	defer collectLock.Release()

	totalPulled := 0
	var totalCutSaved float64

	pCount, pCut := PullMirrorCompletedEpisodes(cfg, targetHost, remoteWorkDir, localPodcastsDir, transport, quiet)
	totalPulled += pCount
	totalCutSaved += pCut

	bCount, bCut := PullStagedBatches(cfg, targetHost, remoteWorkDir, localPodcastsDir, transport)
	totalPulled += bCount
	totalCutSaved += bCut

	if !quiet {
		fmt.Println()
		fmt.Printf("Pull Summary: Collected %d episode(s) from %s (saved %s of ad time).\n",
			totalPulled, targetHost, format.FormatTime(totalCutSaved))
	}

	return nil
}

func PullMirrorCompletedEpisodes(cfg *types.Config, targetHost, remoteWorkDir, localPodcastsDir string, transport RemoteTransport, quiet bool) (int, float64) {
	tempDonePath := filepath.Join(os.TempDir(), fmt.Sprintf("abs_done_%d.json", time.Now().UnixNano()))
	remoteDoneFile := fmt.Sprintf("%s/done.json", remoteWorkDir)

	totalPulled := 0
	var totalCutSaved float64

	if err := transport.Download(targetHost, remoteDoneFile, tempDonePath); err == nil {
		if doneM, err := LoadDoneManifest(tempDonePath); err == nil && len(doneM.Episodes) > 0 {
			if !quiet {
				fmt.Printf("Found %d completed episode(s) in %s:%s\n", len(doneM.Episodes), targetHost, remoteDoneFile)
			}

			var verifiedRelPaths []string
			for relPath, item := range doneM.Episodes {
				if item.Status != types.StateReadyForCopyBack {
					continue
				}
				if pullSingleDoneEpisode(cfg, relPath, item, targetHost, remoteWorkDir, localPodcastsDir, transport, quiet) {
					totalPulled++
					totalCutSaved += item.CutDurationSec
					verifiedRelPaths = append(verifiedRelPaths, relPath)
				}
			}

			if len(verifiedRelPaths) > 0 {
				ackRemoteVerifiedEpisodes(targetHost, remoteWorkDir, verifiedRelPaths, transport)
			}
		}
		_ = os.Remove(tempDonePath)
	}
	return totalPulled, totalCutSaved
}

func pullSingleDoneEpisode(cfg *types.Config, relPath string, item RemoteDoneItem, targetHost, remoteWorkDir, localPodcastsDir string, transport RemoteTransport, quiet bool) bool {
	localDestAudio, relOK := SafeRelUnder(localPodcastsDir, relPath)
	if !relOK {
		fmt.Fprintf(os.Stderr, "Refusing %q from the remote manifest: it escapes the podcasts directory.\n", relPath)
		return false
	}
	localDestDir := filepath.Dir(localDestAudio)
	_ = os.MkdirAll(localDestDir, 0755)

	baseRel := util.StripExt(relPath)
	remoteBase := fmt.Sprintf("%s/%s", remoteWorkDir, baseRel)
	remoteAudio := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
	remoteStat := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)
	remoteCuts := fmt.Sprintf("%s.cuts.json", remoteBase)
	remoteTrans := fmt.Sprintf("%s.transcript.json", remoteBase)

	tempItemDir := filepath.Join(util.WorkDirFor(localDestAudio), fmt.Sprintf("pull_%d", time.Now().UnixNano()))
	_ = os.MkdirAll(tempItemDir, 0755)
	defer os.RemoveAll(tempItemDir)

	tempAudio := filepath.Join(tempItemDir, filepath.Base(localDestAudio))
	tempStat := filepath.Join(tempItemDir, filepath.Base(localDestAudio)+".json")
	tempCuts := filepath.Join(tempItemDir, filepath.Base(baseRel)+".cuts.json")
	tempTrans := filepath.Join(tempItemDir, filepath.Base(baseRel)+".transcript.json")

	if err := transport.Download(targetHost, remoteAudio, tempAudio); err != nil {
		return false
	}

	fi, err := os.Stat(tempAudio)
	if err != nil || fi.Size() == 0 || (item.CleanedSizeBytes > 0 && fi.Size() != item.CleanedSizeBytes) {
		if !quiet && fi != nil && item.CleanedSizeBytes > 0 && fi.Size() != item.CleanedSizeBytes {
			fmt.Fprintf(os.Stderr, "Warning: Download integrity verification failed for %s: expected %d bytes, got %d. Preserving on remote.\n", relPath, item.CleanedSizeBytes, fi.Size())
		}
		return false
	}

	_ = transport.Download(targetHost, remoteStat, tempStat)
	_ = transport.Download(targetHost, remoteCuts, tempCuts)
	_ = transport.Download(targetHost, remoteTrans, tempTrans)

	localPrecut := localDestAudio + ".precut"
	if util.FileExists(localDestAudio) && !util.FileExists(localPrecut) {
		if mvErr := util.SafeMove(localDestAudio, localPrecut); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not preserve the local original for %s: %v\n", relPath, mvErr)
			return false
		}
	}

	if mvErr := util.SafeMove(tempAudio, localDestAudio); mvErr != nil {
		fmt.Fprintf(os.Stderr, "Error: could not install the pulled audio for %s: %v\n", relPath, mvErr)
		return false
	}
	localBase := util.StripExt(localDestAudio)
	if util.FileExists(tempCuts) {
		_ = util.SafeMove(tempCuts, localBase+".cuts.json")
	}
	if util.FileExists(tempTrans) {
		_ = util.SafeMove(tempTrans, localBase+".transcript.json")
	}

	localStat := pipeline.GetOrCreateEpisodeStatus(localDestAudio)
	if util.FileExists(tempStat) {
		if loaded, err := pipeline.LoadEpisodeStatus(tempStat); err == nil {
			localStat = loaded
		}
	}
	localStat.Status = types.StateDone
	_ = pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(localDestAudio), localStat)

	SyncAudiobookshelfDuration(cfg, localDestAudio, item.CleanedDurationSec)

	if !quiet {
		fmt.Printf("✓ Pulled %s -> %s (saved %.1fs)\n", relPath, localDestAudio, item.CutDurationSec)
	}
	return true
}

func ackRemoteVerifiedEpisodes(targetHost, remoteWorkDir string, verifiedRelPaths []string, transport RemoteTransport) {
	var quotedArgs []string
	for _, p := range verifiedRelPaths {
		quotedArgs = append(quotedArgs, ShellQuote(p))
	}
	ackArgs := strings.Join(quotedArgs, " ")
	ackCmd := fmt.Sprintf("abs offload ack %s || ~/.local/bin/abs offload ack %s || abs remote ack %s || ~/.local/bin/abs remote ack %s", ackArgs, ackArgs, ackArgs, ackArgs)
	if _, errAck := transport.Exec(targetHost, ackCmd); errAck != nil {
		for _, p := range verifiedRelPaths {
			_, _ = transport.Exec(targetHost, remoteCleanupCommand(remoteWorkDir, p))
		}
	}
}

func PullStagedBatches(cfg *types.Config, targetHost, remoteWorkDir, localPodcastsDir string, transport RemoteTransport) (int, float64) {
	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)
	out, _ := transport.Exec(targetHost, fmt.Sprintf("ls -1 %s 2>/dev/null", remoteStagingDir))
	if strings.TrimSpace(out) == "" {
		return 0, 0
	}

	totalPulled := 0
	var totalCutSaved float64

	batchEntries := util.SplitLines(strings.TrimSpace(out))
	for _, batchID := range batchEntries {
		batchID = strings.TrimSpace(batchID)
		if batchID == "" || !ValidateBatchID(batchID) {
			continue
		}
		tempPullDir := filepath.Join(os.TempDir(), "abs_pull", batchID)
		_ = os.MkdirAll(tempPullDir, 0755)
		manifestPath := filepath.Join(tempPullDir, "manifest.json")
		remoteManifest := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)
		if err := transport.Download(targetHost, remoteManifest, manifestPath); err == nil {
			if manifest, err := LoadManifest(manifestPath); err == nil && (manifest.Status == types.BatchStatusCompleted || manifest.CompletedItems > 0) {
				tempOutDir := filepath.Join(tempPullDir, "out")
				_ = os.MkdirAll(tempOutDir, 0755)
				remoteOut := fmt.Sprintf("%s/%s/out/", remoteStagingDir, batchID)
				_ = transport.RsyncFrom(targetHost, remoteOut, tempOutDir+"/")
				for _, item := range manifest.Items {
					if pullSingleStagedBatchItem(cfg, item, tempOutDir, localPodcastsDir) {
						totalPulled++
						totalCutSaved += item.CutDurationSec
					}
				}
				_, _ = transport.Exec(targetHost, fmt.Sprintf("rm -rf %s/%s", ShellQuoteHomePath(remoteStagingDir), ShellQuote(batchID)))
			}
		}
		_ = os.RemoveAll(tempPullDir)
	}
	return totalPulled, totalCutSaved
}

func pullSingleStagedBatchItem(cfg *types.Config, item types.RemoteBatchJobItem, tempOutDir, localPodcastsDir string) bool {
	if item.Status != types.BatchStatusCompleted {
		return false
	}
	srcAudio := filepath.Join(tempOutDir, filepath.Base(item.AudioFileName))
	baseName := util.StripExt(filepath.Base(item.AudioFileName))
	srcCuts := filepath.Join(tempOutDir, baseName+".cuts.json")
	srcTranscript := filepath.Join(tempOutDir, baseName+".transcript.json")

	destMP3, destOK := ResolveManifestDest(localPodcastsDir, item)
	if !destOK {
		fmt.Fprintf(os.Stderr, "Refusing manifest entry %q: it does not resolve inside the podcasts directory.\n", item.AudioFileName)
		return false
	}
	destDir := filepath.Dir(destMP3)
	_ = os.MkdirAll(destDir, 0755)

	destBase := util.StripExt(destMP3)
	destPrecut := destMP3 + ".precut"
	destCuts := destBase + ".cuts.json"
	destTranscript := destBase + ".transcript.json"

	if util.FileExists(destMP3) && !util.FileExists(destPrecut) {
		if mvErr := util.SafeMove(destMP3, destPrecut); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not preserve the original for %s: %v\n", destMP3, mvErr)
			return false
		}
	}
	if util.FileExists(srcAudio) {
		if mvErr := util.SafeMove(srcAudio, destMP3); mvErr != nil {
			fmt.Fprintf(os.Stderr, "Error: could not install the cut audio for %s: %v\n", destMP3, mvErr)
			return false
		}
	}
	if util.FileExists(srcCuts) {
		_ = util.SafeMove(srcCuts, destCuts)
	}
	if util.FileExists(srcTranscript) {
		_ = util.SafeMove(srcTranscript, destTranscript)
	}
	_ = pipeline.UpdateEpisodeStatus(destMP3, func(st *types.EpisodeStatusFile) { st.Status = types.StateDone })
	SyncAudiobookshelfDuration(cfg, destMP3, item.CleanedDurationSec)
	return true
}

func TriggerBackgroundCollect(cfg *types.Config) {
	if cfg == nil {
		return
	}
	var hosts []string
	if cfg.RemoteHost != "" && !strings.EqualFold(cfg.RemoteHost, "local") {
		hosts = append(hosts, cfg.RemoteHost)
	}
	if cfg.RemoteFFmpegHost != "" && !strings.EqualFold(cfg.RemoteFFmpegHost, "local") && cfg.RemoteFFmpegHost != cfg.RemoteHost {
		hosts = append(hosts, cfg.RemoteFFmpegHost)
	}
	for _, h := range hosts {
		_ = RunRemotePull(cfg, h, nil, true, false)
	}
}

func remoteCleanupCommand(remoteWorkDir, relPath string) string {
	base := strings.TrimSuffix(remoteWorkDir, "/") + "/" + relPath
	return fmt.Sprintf("rm -f %s %s %s",
		ShellQuoteHomePath(base),
		ShellQuoteHomePath(base+".precut"),
		ShellQuoteHomePath(base+".tmp.mp3"))
}

func SafeRelUnder(base, rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", false
	}
	joined := filepath.Join(absBase, rel)
	if joined != absBase && !strings.HasPrefix(joined, absBase+string(os.PathSeparator)) {
		return "", false
	}
	return joined, true
}

func ResolveManifestDest(localPodcastsDir string, item types.RemoteBatchJobItem) (string, bool) {
	if item.RelativePath != "" {
		if p, ok := SafeRelUnder(localPodcastsDir, item.RelativePath); ok {
			return p, true
		}
	}
	if item.SourceFile == "" {
		return "", false
	}
	absBase, err := filepath.Abs(localPodcastsDir)
	if err != nil {
		return "", false
	}
	absSrc, err := filepath.Abs(item.SourceFile)
	if err != nil {
		return "", false
	}
	if absSrc != absBase && !strings.HasPrefix(absSrc, absBase+string(os.PathSeparator)) {
		return "", false
	}
	return absSrc, true
}

func SyncAudiobookshelfDuration(cfg *types.Config, filePath string, duration float64) {
	if cfg == nil {
		return
	}
	if cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" && cfg.PodfetchURL == "" && cfg.PodfetchDBPath == "" {
		return
	}
	b, err := backend.FromAppConfig(cfg, true)
	if err != nil || b == nil {
		return
	}
	_ = b.SyncDuration(filePath, duration)
}
