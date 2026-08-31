package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
			mp3s := findMP3Files(p)
			for _, m := range mp3s {
				absM, err := filepath.Abs(m)
				if err == nil && !seen[absM] && !strings.Contains(absM, "/.work/") && !strings.HasSuffix(absM, ".precut") {
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

func runRemotePush(cfg *Config, args []string, host string, transport RemoteTransport, quiet, verbose bool) error {
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

	batchID := generateBatchID()
	localStagingDir := filepath.Join(os.TempDir(), "abs_staging", batchID)
	localInDir := filepath.Join(localStagingDir, "in")
	localOutDir := filepath.Join(localStagingDir, "out")

	if err := os.MkdirAll(localInDir, 0755); err != nil {
		return fmt.Errorf("failed to create local staging directory: %w", err)
	}
	if err := os.MkdirAll(localOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create local staging out directory: %w", err)
	}
	defer os.RemoveAll(localStagingDir)

	manifest := RemoteBatchManifest{
		BatchID:    batchID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Host:       targetHost,
		Status:     BatchStatusQueued,
		TotalItems: len(files),
		Items:      make([]RemoteBatchJobItem, 0, len(files)),
	}

	if !quiet {
		fmt.Printf("Staging %d audio file(s) for remote batch %s...\n", len(files), batchID)
	}

	for i, f := range files {
		fname := filepath.Base(f)
		stagedInPath := filepath.Join(localInDir, fname)
		copyFile(f, stagedInPath)

		item := RemoteBatchJobItem{
			ID:            fmt.Sprintf("%s-%d", batchID, i+1),
			SourceFile:    f,
			AudioFileName: fname,
			Status:        BatchStatusQueued,
		}
		manifest.Items = append(manifest.Items, item)
	}

	manifestPath := filepath.Join(localStagingDir, "manifest.json")
	if err := saveManifest(manifestPath, &manifest); err != nil {
		return fmt.Errorf("failed to write local manifest: %w", err)
	}

	remoteWorkDir := "~/.abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}
	remoteBatchDir := fmt.Sprintf("%s/staging/%s", remoteWorkDir, batchID)

	if !quiet {
		fmt.Printf("Uploading batch to %s:%s...\n", targetHost, remoteBatchDir)
	}

	mkdirCmd := fmt.Sprintf("mkdir -p %s/in %s/out", remoteBatchDir, remoteBatchDir)
	if _, err := transport.Exec(targetHost, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create remote staging folder on %s: %w", targetHost, err)
	}

	if err := transport.RsyncTo(targetHost, localStagingDir+"/", remoteBatchDir+"/"); err != nil {
		if errUpload := transport.Upload(targetHost, manifestPath, fmt.Sprintf("%s/manifest.json", remoteBatchDir)); errUpload != nil {
			return fmt.Errorf("failed to rsync/upload batch to %s: %w", targetHost, err)
		}
		for _, item := range manifest.Items {
			localFile := filepath.Join(localInDir, item.AudioFileName)
			remoteDst := fmt.Sprintf("%s/in/%s", remoteBatchDir, item.AudioFileName)
			if errUp := transport.Upload(targetHost, localFile, remoteDst); errUp != nil {
				return fmt.Errorf("failed to upload audio file %s: %w", item.AudioFileName, errUp)
			}
		}
	}

	if !quiet {
		fmt.Printf("Triggering background worker on %s...\n", targetHost)
	}

	workerCmd := fmt.Sprintf("nohup ~/.local/bin/abs batch-worker --batch-dir %s > %s/worker.log 2>&1 &", remoteBatchDir, remoteBatchDir)
	if _, err := transport.Exec(targetHost, workerCmd); err != nil {
		altWorkerCmd := fmt.Sprintf("nohup abs batch-worker --batch-dir %s > %s/worker.log 2>&1 &", remoteBatchDir, remoteBatchDir)
		if _, errAlt := transport.Exec(targetHost, altWorkerCmd); errAlt != nil {
			return fmt.Errorf("failed to trigger batch-worker on %s: %w", targetHost, err)
		}
	}

	if !quiet {
		fmt.Println()
		fmt.Printf("Successfully pushed batch %s to %s (%d files).\n", batchID, targetHost, len(files))
		fmt.Printf("  - Check status: abs remote status %s\n", targetHost)
		fmt.Printf("  - Pull results: abs remote pull %s\n", targetHost)
	}

	return nil
}
