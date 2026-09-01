package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (c *PodFetchBackend) ResetPodcastDateCheckAPI(itemID string) error {
	if itemID == "" || c.Host == "" {
		return nil
	}
	_, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/refresh", itemID), "POST", nil)
	return err
}

func (c *PodFetchBackend) ResetPodcastDateCheck(itemID, title string) error {
	var errs []string
	if c.DBPath != "" {
		if err := resetPodFetchDateCheckDB(c.DBPath, itemID, title); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.Host != "" && itemID != "" {
		if err := c.ResetPodcastDateCheckAPI(itemID); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 && c.DBPath == "" && c.Host == "" {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (c *PodFetchBackend) Scan(opts ScanOptions) (ScanResult, error) {
	podcastsDir := opts.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = c.PodcastsDir
	}
	if podcastsDir == "" {
		return ScanResult{}, fmt.Errorf("podcasts_dir is not configured")
	}

	podcasts, err := c.Podcasts()
	if err != nil {
		return ScanResult{}, fmt.Errorf("failed to fetch podcasts: %w", err)
	}

	res := ScanResult{
		CheckedPodcasts: len(podcasts),
		Podcasts:        podcasts,
	}

	if opts.EpisodesOnly {
		return res, nil
	}

	existing := make(map[string]bool)
	if dirEntries, err := os.ReadDir(podcastsDir); err == nil {
		for _, e := range dirEntries {
			if e.IsDir() {
				existing[strings.ToLower(e.Name())] = true
			}
		}
	}

	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		relBase := filepath.Base(item.RelPath)

		isNew := true
		if existing[strings.ToLower(title)] || existing[strings.ToLower(relBase)] {
			isNew = false
		}

		if isNew {
			res.NewPodcasts++
			safeName := sanitizePodcastName(title)
			if safeName == "" {
				safeName = sanitizePodcastName(relBase)
			}
			if safeName == "" {
				safeName = "podcast_" + item.ID
			}

			podDir := filepath.Join(podcastsDir, safeName)
			_ = os.MkdirAll(podDir, 0755)

			detailsDir := filepath.Join(podDir, ".cache", "details")
			_ = os.MkdirAll(detailsDir, 0755)
			coverDest := filepath.Join(detailsDir, "cover.jpg")
			_ = c.DownloadCover(item.ID, coverDest)
		}
	}

	return res, nil
}

func (c *PodFetchBackend) Rescan(opts RescanOptions) (RescanResult, error) {
	podcastsDir := opts.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = c.PodcastsDir
	}

	podcasts, err := c.Podcasts()
	if err != nil {
		return RescanResult{}, fmt.Errorf("failed to fetch podcasts: %w", err)
	}

	var totalRescanned, totalChecked int
	for _, p := range podcasts {
		if opts.PodcastID != "" && p.ID != opts.PodcastID && !strings.EqualFold(p.Media.Metadata.Title, opts.PodcastID) {
			continue
		}
		for _, ep := range p.Media.Episodes {
			totalChecked++
			hostPath := ""
			if ep.AudioFile != nil && ep.AudioFile.Metadata != nil {
				hostPath = resolveHostPathLocal(ep.AudioFile.Metadata.Path, podcastsDir)
			}
			if hostPath == "" && podcastsDir != "" {
				candidate := filepath.Join(podcastsDir, p.RelPath, ep.Title+".mp3")
				if _, err := os.Stat(candidate); err == nil {
					hostPath = candidate
				}
			}
			if hostPath == "" {
				continue
			}

			diskDur := GetMP3DiskDuration(hostPath)
			if diskDur > 0 && ep.Duration > 0 && diskDur < ep.Duration-0.5 {
				if !opts.DryRun {
					_ = c.SyncDuration(hostPath, diskDur)
				}
				totalRescanned++
			}
		}
	}

	return RescanResult{
		RescanCount:  totalRescanned,
		CheckedCount: totalChecked,
	}, nil
}

func (c *PodFetchBackend) SyncDuration(filePath string, duration float64) error {
	var errs []string
	if c.DBPath != "" {
		if err := updatePodFetchDurationDB(c.DBPath, filePath, duration); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.Host != "" {
		parentDir := filepath.Base(filepath.Dir(filePath))
		_, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/refresh", parentDir), "POST", nil)
		if err != nil {
			_, _ = c.Request("/api/v1/podcasts/refresh", "POST", nil)
		}
	}
	if len(errs) > 0 && c.Host == "" {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (c *PodFetchBackend) ApplyKeepPolicy(podcastID, podcastTitle string, keep int, dryRun, verbose, quiet bool) (int, error) {
	updatedItem, err := c.GetPodcast(podcastID)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch updated podcast for keep policy: %w", err)
	}

	downloadedEpisodes := updatedItem.Media.Episodes
	sortedDownloaded := make([]Episode, len(downloadedEpisodes))
	copy(sortedDownloaded, downloadedEpisodes)

	getEpPubMS := func(ep Episode) int64 {
		if ep.PublishedAt > 0 {
			return ep.PublishedAt
		}
		return ParsePubDate(ep.PubDate)
	}

	sort.Slice(sortedDownloaded, func(i, j int) bool {
		return getEpPubMS(sortedDownloaded[i]) < getEpPubMS(sortedDownloaded[j])
	})

	deletedCount := 0
	if len(sortedDownloaded) > keep {
		toDeleteCount := len(sortedDownloaded) - keep
		episodesToDelete := sortedDownloaded[:toDeleteCount]

		for _, ep := range episodesToDelete {
			if !dryRun {
				if err := c.DeletePodcastEpisode(podcastID, ep.ID); err == nil {
					deletedCount++
				}
			}
		}
	}
	return deletedCount, nil
}

func (c *PodFetchBackend) WaitForActiveDownloads(podcasts []Podcast, quiet bool, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	startTime := time.Now()

	for {
		hasActive := false
		for _, p := range podcasts {
			dls, err := c.ActiveDownloads(p.ID)
			if err == nil && len(dls) > 0 {
				hasActive = true
				break
			}
		}
		if !hasActive {
			break
		}
		time.Sleep(2 * time.Second)
		if time.Since(startTime) > timeout {
			break
		}
	}
	return nil
}
