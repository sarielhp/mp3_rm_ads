package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runRemoteAck(remoteDir string, relPaths []string) error {
	resolvedDir := resolveLocalPath(remoteDir)
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

		statPath := statusPathFor(audioPath)
		if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
			st.Status = StateArchived
			_ = saveEpisodeStatus(statPath, st)
		}

		_ = archiveDoneEpisode(donePath, archPath, relPath)
	}
	return nil
}

func runRemotePull(cfg *Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote pull requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
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

	collectLock, err := acquireCollectLock(localPodcastsDir)
	if err != nil || collectLock == nil {
		if verbose {
			fmt.Println("Another collect operation is currently in progress; skipping.")
		}
		return nil
	}
	defer collectLock.Release()

	totalPulled := 0
	var totalCutSaved float64

	tempDonePath := filepath.Join(os.TempDir(), fmt.Sprintf("abs_done_%d.json", time.Now().UnixNano()))
	remoteDoneFile := fmt.Sprintf("%s/done.json", remoteWorkDir)

	if err := transport.Download(targetHost, remoteDoneFile, tempDonePath); err == nil {
		if doneM, err := loadDoneManifest(tempDonePath); err == nil && len(doneM.Episodes) > 0 {
			if !quiet {
				fmt.Printf("Found %d completed episode(s) in %s:%s\n", len(doneM.Episodes), targetHost, remoteDoneFile)
			}

			var verifiedRelPaths []string

			for relPath, item := range doneM.Episodes {
				if item.Status != StateReadyForCopyBack {
					continue
				}

				localDestAudio := filepath.Join(localPodcastsDir, relPath)
				localDestDir := filepath.Dir(localDestAudio)
				_ = os.MkdirAll(localDestDir, 0755)

				baseRel := stripExt(relPath)
				remoteBase := fmt.Sprintf("%s/%s", remoteWorkDir, baseRel)
				remoteAudio := fmt.Sprintf("%s/%s", remoteWorkDir, relPath)
				remoteStat := fmt.Sprintf("%s/%s.json", remoteWorkDir, relPath)
				remoteCuts := fmt.Sprintf("%s.cuts.json", remoteBase)
				remoteTrans := fmt.Sprintf("%s.transcript.json", remoteBase)

				tempItemDir := filepath.Join(workDirFor(localDestAudio), fmt.Sprintf("pull_%d", time.Now().UnixNano()))
				_ = os.MkdirAll(tempItemDir, 0755)

				tempAudio := filepath.Join(tempItemDir, filepath.Base(localDestAudio))
				tempStat := filepath.Join(tempItemDir, filepath.Base(localDestAudio)+".json")
				tempCuts := filepath.Join(tempItemDir, filepath.Base(baseRel)+".cuts.json")
				tempTrans := filepath.Join(tempItemDir, filepath.Base(baseRel)+".transcript.json")

				if err := transport.Download(targetHost, remoteAudio, tempAudio); err != nil {
					_ = os.RemoveAll(tempItemDir)
					continue
				}

				fi, err := os.Stat(tempAudio)
				if err != nil || fi.Size() == 0 || (item.CleanedSizeBytes > 0 && fi.Size() != item.CleanedSizeBytes) {
					if !quiet && fi != nil && item.CleanedSizeBytes > 0 && fi.Size() != item.CleanedSizeBytes {
						fmt.Fprintf(os.Stderr, "Warning: Download integrity verification failed for %s: expected %d bytes, got %d. Preserving on remote.\n", relPath, item.CleanedSizeBytes, fi.Size())
					}
					_ = os.RemoveAll(tempItemDir)
					continue
				}

				_ = transport.Download(targetHost, remoteStat, tempStat)
				_ = transport.Download(targetHost, remoteCuts, tempCuts)
				_ = transport.Download(targetHost, remoteTrans, tempTrans)

				localPrecut := localDestAudio + ".precut"
				if fileExists(localDestAudio) && !fileExists(localPrecut) {
					checkPrecutSymlink(localPrecut)
					if mvErr := safeMove(localDestAudio, localPrecut); mvErr != nil {
						fmt.Fprintf(os.Stderr, "Error: could not preserve the local original for %s: %v\n", relPath, mvErr)
						fmt.Fprintf(os.Stderr, "Leaving the remote copy in place; nothing was changed locally.\n")
						_ = os.RemoveAll(tempItemDir)
						continue
					}
				}

				if mvErr := safeMove(tempAudio, localDestAudio); mvErr != nil {
					fmt.Fprintf(os.Stderr, "Error: could not install the pulled audio for %s: %v\n", relPath, mvErr)
					fmt.Fprintf(os.Stderr, "The download is kept at %s and the remote copy is NOT acknowledged.\n", tempAudio)
					continue
				}
				localBase := stripExt(localDestAudio)
				if fileExists(tempCuts) {
					_ = safeMove(tempCuts, localBase+".cuts.json")
				}
				if fileExists(tempTrans) {
					_ = safeMove(tempTrans, localBase+".transcript.json")
				}

				localStat := getOrCreateEpisodeStatus(localDestAudio)
				if fileExists(tempStat) {
					if loaded, err := loadEpisodeStatus(tempStat); err == nil {
						localStat = loaded
					}
				}
				localStat.Status = StateDone
				_ = saveEpisodeStatus(statusPathFor(localDestAudio), localStat)

				syncAudiobookshelfDuration(cfg, localDestAudio, item.CleanedDurationSec)

				_ = os.RemoveAll(tempItemDir)
				totalPulled++
				totalCutSaved += item.CutDurationSec
				verifiedRelPaths = append(verifiedRelPaths, relPath)

				if !quiet {
					fmt.Printf("✓ Pulled %s -> %s (saved %.1fs)\n", relPath, localDestAudio, item.CutDurationSec)
				}
			}

			if len(verifiedRelPaths) > 0 {
				var quotedArgs []string
				for _, p := range verifiedRelPaths {
					quotedArgs = append(quotedArgs, fmt.Sprintf("%q", p))
				}
				ackArgs := strings.Join(quotedArgs, " ")
				ackCmd := fmt.Sprintf("abs remote ack %s || ~/.local/bin/abs remote ack %s", ackArgs, ackArgs)
				_, errAck := transport.Exec(targetHost, ackCmd)
				if errAck != nil {
					for _, p := range verifiedRelPaths {
						remAudio := fmt.Sprintf("%s/%s", remoteWorkDir, p)
						delCmd := fmt.Sprintf("rm -f %s %s.precut %s.tmp.mp3", remAudio, remAudio, remAudio)
						_, _ = transport.Exec(targetHost, delCmd)
					}
				}
			}
		}
		_ = os.Remove(tempDonePath)
	}

	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)
	out, _ := transport.Exec(targetHost, fmt.Sprintf("ls -1 %s 2>/dev/null", remoteStagingDir))
	if strings.TrimSpace(out) != "" {
		batchEntries := splitLines(strings.TrimSpace(out))
		for _, batchID := range batchEntries {
			batchID = strings.TrimSpace(batchID)
			if batchID == "" {
				continue
			}
			tempPullDir := filepath.Join(os.TempDir(), "abs_pull", batchID)
			_ = os.MkdirAll(tempPullDir, 0755)
			manifestPath := filepath.Join(tempPullDir, "manifest.json")
			remoteManifest := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)
			if err := transport.Download(targetHost, remoteManifest, manifestPath); err == nil {
				if manifest, err := loadManifest(manifestPath); err == nil && (manifest.Status == BatchStatusCompleted || manifest.CompletedItems > 0) {
					tempOutDir := filepath.Join(tempPullDir, "out")
					_ = os.MkdirAll(tempOutDir, 0755)
					remoteOut := fmt.Sprintf("%s/%s/out/", remoteStagingDir, batchID)
					_ = transport.RsyncFrom(targetHost, remoteOut, tempOutDir+"/")
					for _, item := range manifest.Items {
						if item.Status != BatchStatusCompleted {
							continue
						}
						srcAudio := filepath.Join(tempOutDir, item.AudioFileName)
						baseName := stripExt(item.AudioFileName)
						srcCuts := filepath.Join(tempOutDir, baseName+".cuts.json")
						srcTranscript := filepath.Join(tempOutDir, baseName+".transcript.json")

						destMP3 := item.SourceFile
						destDir := filepath.Dir(destMP3)
						_ = os.MkdirAll(destDir, 0755)

						destBase := stripExt(destMP3)
						destPrecut := destMP3 + ".precut"
						destCuts := destBase + ".cuts.json"
						destTranscript := destBase + ".transcript.json"

						if fileExists(destMP3) && !fileExists(destPrecut) {
							checkPrecutSymlink(destPrecut)
							if mvErr := safeMove(destMP3, destPrecut); mvErr != nil {
								fmt.Fprintf(os.Stderr, "Error: could not preserve the original for %s: %v\n", destMP3, mvErr)
								continue
							}
						}
						if fileExists(srcAudio) {
							if mvErr := safeMove(srcAudio, destMP3); mvErr != nil {
								fmt.Fprintf(os.Stderr, "Error: could not install the cut audio for %s: %v\n", destMP3, mvErr)
								fmt.Fprintf(os.Stderr, "The remote staging directory is NOT removed.\n")
								continue
							}
						}
						if fileExists(srcCuts) {
							_ = safeMove(srcCuts, destCuts)
						}
						if fileExists(srcTranscript) {
							_ = safeMove(srcTranscript, destTranscript)
						}
						updateEpisodeStatus(destMP3, func(st *EpisodeStatusFile) { st.Status = StateDone })
						syncAudiobookshelfDuration(cfg, destMP3, item.CleanedDurationSec)
						totalPulled++
						totalCutSaved += item.CutDurationSec
					}
					_, _ = transport.Exec(targetHost, fmt.Sprintf("rm -rf %s/%s", remoteStagingDir, batchID))
				}
			}
			_ = os.RemoveAll(tempPullDir)
		}
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Pull Summary: Collected %d episode(s) from %s (saved %s of ad time).\n",
			totalPulled, targetHost, formatTime(totalCutSaved))
	}

	return nil
}

func triggerBackgroundCollect(cfg *Config) {
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
		_ = runRemotePull(cfg, h, nil, true, false)
	}
}
