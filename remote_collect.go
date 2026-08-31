package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func syncAudiobookshelfDuration(cfg *Config, filePath string, duration float64) {
	if cfg == nil || cfg.AudiobookshelfURL == "" {
		return
	}

	client, err := getABSClient(*cfg, true)
	if err != nil {
		return
	}

	items, err := client.PodcastItems()
	if err != nil {
		return
	}

	parentDir := filepath.Base(filepath.Dir(filePath))
	for _, item := range items {
		title := item.Media.Metadata.Title
		if strings.EqualFold(title, parentDir) || strings.EqualFold(item.ID, parentDir) {
			_, _ = client.Request(fmt.Sprintf("/api/items/%s/scan", item.ID), "POST", nil)
			break
		}
	}
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

	remoteWorkDir := "~/.abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}
	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)

	out, err := transport.Exec(targetHost, fmt.Sprintf("ls -1 %s 2>/dev/null", remoteStagingDir))
	if err != nil || strings.TrimSpace(out) == "" {
		if !quiet {
			fmt.Printf("No remote batches found on %s.\n", targetHost)
		}
		return nil
	}

	batchEntries := splitLines(strings.TrimSpace(out))
	totalPulled := 0
	var totalCutSaved float64

	for _, batchID := range batchEntries {
		batchID = strings.TrimSpace(batchID)
		if batchID == "" {
			continue
		}

		tempPullDir := filepath.Join(os.TempDir(), "abs_pull", batchID)
		_ = os.MkdirAll(tempPullDir, 0755)

		manifestPath := filepath.Join(tempPullDir, "manifest.json")
		remoteManifest := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)

		if err := transport.Download(targetHost, remoteManifest, manifestPath); err != nil {
			_ = os.RemoveAll(tempPullDir)
			continue
		}

		manifest, err := loadManifest(manifestPath)
		if err != nil {
			_ = os.RemoveAll(tempPullDir)
			continue
		}

		if manifest.Status != BatchStatusCompleted && manifest.CompletedItems == 0 {
			if !quiet {
				fmt.Printf("Batch %s is currently '%s' (%d/%d completed). Skipping.\n",
					batchID, manifest.Status, manifest.CompletedItems, manifest.TotalItems)
			}
			_ = os.RemoveAll(tempPullDir)
			continue
		}

		tempOutDir := filepath.Join(tempPullDir, "out")
		_ = os.MkdirAll(tempOutDir, 0755)
		remoteOut := fmt.Sprintf("%s/%s/out/", remoteStagingDir, batchID)
		_ = transport.RsyncFrom(targetHost, remoteOut, tempOutDir+"/")

		batchPulled := 0
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

			if fileExists(destMP3) {
				checkPrecutSymlink(destPrecut)
				safeMove(destMP3, destPrecut)
			}

			if fileExists(srcAudio) {
				safeMove(srcAudio, destMP3)
			}
			if fileExists(srcCuts) {
				safeMove(srcCuts, destCuts)
			}
			if fileExists(srcTranscript) {
				safeMove(srcTranscript, destTranscript)
			}

			syncAudiobookshelfDuration(cfg, destMP3, item.CleanedDurationSec)

			batchPulled++
			totalPulled++
			totalCutSaved += item.CutDurationSec

			if !quiet {
				fmt.Printf("✓ Pulled %s -> %s (ad time cut: %.1fs)\n", item.AudioFileName, destMP3, item.CutDurationSec)
			}
		}

		_, _ = transport.Exec(targetHost, fmt.Sprintf("rm -rf %s/%s", remoteStagingDir, batchID))
		_ = os.RemoveAll(tempPullDir)

		if !quiet && batchPulled > 0 {
			fmt.Printf("Batch %s completed and pulled (%d item(s)). Remote staging cleaned up.\n", batchID, batchPulled)
		}
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Pull Summary: Collected %d episode(s) from %s (saved %s of ad time).\n",
			totalPulled, targetHost, formatTime(totalCutSaved))
	}

	return nil
}
