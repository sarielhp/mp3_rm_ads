package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runRemoteCancel(cfg *Config, host, batchID string, transport RemoteTransport, quiet bool) error {
	targetHost, _, err := ResolveProcessingHost(cfg, host, transport)
	if err != nil {
		return err
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("remote cancel requires a configured remote host or host argument")
	}

	if transport == nil {
		transport = getRemoteTransport()
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}
	remoteStagingDir := fmt.Sprintf("%s/staging", remoteWorkDir)

	if batchID != "" {
		if !validateBatchID(batchID) {
			return fmt.Errorf("invalid batch ID format: %q", batchID)
		}
		tempDir := filepath.Join(os.TempDir(), "abs_cancel", batchID)
		_ = os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		manPath := filepath.Join(tempDir, "manifest.json")
		remoteMan := fmt.Sprintf("%s/%s/manifest.json", remoteStagingDir, batchID)

		if err := transport.Download(targetHost, remoteMan, manPath); err == nil {
			if m, err := loadManifest(manPath); err == nil {
				m.Status = BatchStatusCancelled
				m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				_ = saveManifest(manPath, m)
				_ = transport.Upload(targetHost, manPath, remoteMan)
			}
		}

		killCmd := fmt.Sprintf("pkill -f %s 2>/dev/null || true", shellQuote("batch-worker.*"+batchID))
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

func cleanRemoteRelPath(rawPath, remoteWorkDir string) string {
	rawPath = strings.TrimSuffix(strings.TrimSpace(rawPath), ".json")
	if idx := strings.Index(rawPath, "abs_remote/"); idx != -1 {
		return rawPath[idx+len("abs_remote/"):]
	}
	return filepath.Base(rawPath)
}

func resolveLocalAudioPath(podcastsDir, relTask string) string {
	exact := filepath.Join(podcastsDir, relTask)
	if _, err := os.Stat(exact); err == nil {
		return exact
	}
	podName := filepath.Dir(relTask)
	baseName := filepath.Base(relTask)
	podDir := filepath.Join(podcastsDir, podName)
	if fi, err := os.Stat(podDir); err == nil && fi.IsDir() {
		if entries, err := os.ReadDir(podDir); err == nil {
			basePrefix := stripExt(baseName)
			if idx := strings.LastIndex(basePrefix, " ("); idx != -1 && strings.HasSuffix(basePrefix, ")") {
				basePrefix = strings.TrimSpace(basePrefix[:idx])
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				en := e.Name()
				if strings.HasSuffix(en, ".mp3") || strings.HasSuffix(en, ".m4a") {
					enPrefix := stripExt(en)
					if idx := strings.LastIndex(enPrefix, " ("); idx != -1 && strings.HasSuffix(enPrefix, ")") {
						enPrefix = strings.TrimSpace(enPrefix[:idx])
					}
					if en == baseName || enPrefix == basePrefix || strings.HasPrefix(en, basePrefix) || strings.HasPrefix(basePrefix, enPrefix) {
						return filepath.Join(podDir, en)
					}
				}
			}
		}
	}
	return exact
}
