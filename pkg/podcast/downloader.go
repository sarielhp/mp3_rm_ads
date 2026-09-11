package podcast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"abs/pkg/util"
)

const PodcastUserAgent = "abs/1.0 (+https://github.com/sarielhp/mp3_rm_ads; Podcast Downloader)"

type Downloader struct {
	Client *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		Client: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

func (d *Downloader) DownloadEpisode(ctx context.Context, enclosureURL, destPath string, quiet bool) error {
	if enclosureURL == "" {
		return fmt.Errorf("empty enclosure URL")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dest directory: %w", err)
	}

	workDir := filepath.Join(filepath.Dir(destPath), util.WorkDirName)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("create .work directory: %w", err)
	}

	tempPath := filepath.Join(workDir, filepath.Base(destPath)+".download")
	if err := util.VerifyTempFile(tempPath); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := d.fetchToFile(ctx, enclosureURL, tempPath, quiet); err != nil {
		return err
	}

	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("atomic rename %s to %s: %w", tempPath, destPath, err)
	}
	return nil
}

func (d *Downloader) fetchToFile(ctx context.Context, enclosureURL, tempPath string, quiet bool) error {
	req, err := http.NewRequestWithContext(ctx, "GET", enclosureURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", PodcastUserAgent)

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download HTTP %d %s", resp.StatusCode, resp.Status)
	}

	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("stream audio: %w", err)
	}
	if written == 0 {
		return fmt.Errorf("downloaded 0 bytes from %s", enclosureURL)
	}

	if !quiet {
		fmt.Printf("Downloaded %s (%.2f MB)\n", filepath.Base(tempPath), float64(written)/(1024*1024))
	}
	return nil
}
