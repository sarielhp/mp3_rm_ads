package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (c *PodFetchBackend) Podcasts() ([]Podcast, error) {
	if c.Host == "" {
		if c.DBPath != "" {
			return fetchPodFetchPodcastsDB(c.DBPath)
		}
		return nil, fmt.Errorf("neither host nor db_path configured for podfetch")
	}

	body, err := c.Request("/api/v1/podcasts", "GET", nil)
	if err != nil {
		if c.DBPath != "" {
			return fetchPodFetchPodcastsDB(c.DBPath)
		}
		return nil, err
	}

	dtos, err := unmarshalPodFetchPodcastList(body)
	if err != nil {
		if c.DBPath != "" {
			return fetchPodFetchPodcastsDB(c.DBPath)
		}
		return nil, err
	}
	return c.fetchPodcastDetails(dtos), nil
}

// fetchPodcastDetails fills in each podcast's episodes. PodFetch exposes them
// only one podcast at a time, so the requests are issued concurrently; doing
// them in sequence made listing a library cost one round trip per podcast.
func (c *PodFetchBackend) fetchPodcastDetails(dtos []podFetchItemDTO) []Podcast {
	const maxConcurrent = 10

	ordered := make([]Podcast, len(dtos))
	sem := make(chan struct{}, maxConcurrent)
	var wg syncWaitGroup

	for i, dto := range dtos {
		wg.Add(1)
		go func(idx int, dto podFetchItemDTO) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			idStr := fmt.Sprintf("%v", dto.ID)
			if pod, err := c.GetPodcast(idStr); err == nil && pod != nil {
				ordered[idx] = *pod
				return
			}
			ordered[idx] = mapPodFetchDTOToPodcast(dto, nil)
		}(i, dto)
	}
	wg.Wait()

	return ordered
}

func (c *PodFetchBackend) GetPodcast(id string) (*Podcast, error) {
	if c.Host != "" {
		body, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s", id), "GET", nil)
		if err != nil {
			if c.DBPath != "" {
				return fetchPodFetchPodcastDB(c.DBPath, id)
			}
			return nil, err
		}

		var detailed struct {
			podFetchItemDTO
			Episodes []podFetchEpisodeDTO `json:"episodes"`
		}
		if err := json.Unmarshal(body, &detailed); err == nil && detailed.ID != nil && len(detailed.Episodes) > 0 {
			pod := mapPodFetchDTOToPodcast(detailed.podFetchItemDTO, detailed.Episodes)
			return &pod, nil
		}

		var dto podFetchItemDTO
		if err := json.Unmarshal(body, &dto); err != nil {
			if c.DBPath != "" {
				return fetchPodFetchPodcastDB(c.DBPath, id)
			}
			return nil, err
		}

		epBody, epErr := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/episodes", id), "GET", nil)
		var epDTOs []podFetchEpisodeDTO
		if epErr == nil {
			epDTOs = unmarshalPodFetchEpisodes(epBody)
		}
		if len(epDTOs) == 0 && c.DBPath != "" {
			return fetchPodFetchPodcastDB(c.DBPath, id)
		}
		pod := mapPodFetchDTOToPodcast(dto, epDTOs)
		return &pod, nil
	}

	if c.DBPath != "" {
		return fetchPodFetchPodcastDB(c.DBPath, id)
	}

	return nil, fmt.Errorf("neither host nor db_path configured for podfetch")
}

func (c *PodFetchBackend) CreatePodcast(libraryID, folderID, path, title, feedURL string) (*Podcast, error) {
	if c.Host != "" {
		payload := map[string]string{
			"rssfeed":   feedURL,
			"name":      title,
			"directory": filepath.Base(path),
		}
		body, err := c.Request("/api/v1/podcasts", "POST", payload)
		if err != nil {
			body, err = c.Request("/api/v1/podcasts/feed", "POST", payload)
		}
		if err != nil {
			return nil, err
		}

		var dto podFetchItemDTO
		if err := json.Unmarshal(body, &dto); err == nil && dto.ID != nil {
			pod := mapPodFetchDTOToPodcast(dto, nil)
			return &pod, nil
		}
		return &Podcast{
			ID:      title,
			RelPath: filepath.Base(path),
			Media: PodcastMedia{
				Metadata: PodcastMetadata{
					Title:   title,
					FeedURL: feedURL,
				},
			},
		}, nil
	}

	if c.DBPath != "" {
		return createPodFetchPodcastDB(c.DBPath, title, filepath.Base(path), feedURL)
	}

	return nil, fmt.Errorf("neither host nor db_path configured for podfetch")
}

func (c *PodFetchBackend) PodcastFeedEpisodes(feedURL string) ([]FeedEpisode, error) {
	if c.Host != "" {
		payload := map[string]string{"rssfeed": feedURL}
		body, err := c.Request("/api/v1/podcasts/feed", "POST", payload)
		if err == nil {
			var res struct {
				Episodes []podFetchEpisodeDTO `json:"episodes"`
			}
			if json.Unmarshal(body, &res) == nil && len(res.Episodes) > 0 {
				var eps []FeedEpisode
				for _, dto := range res.Episodes {
					eps = append(eps, mapPodFetchDTOToFeedEpisode(dto))
				}
				return eps, nil
			}
		}
	}

	return c.fetchFeedDirectFallback(feedURL)
}

func (c *PodFetchBackend) fetchFeedDirectFallback(feedURL string) ([]FeedEpisode, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest("GET", feedURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(c.getRetryDelay(attempt))
				continue
			}
			return nil, err
		}

		if resp.StatusCode == 429 || resp.StatusCode == 408 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
			if attempt < maxAttempts {
				time.Sleep(c.getRetryDelay(attempt))
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
		}

		return readAndParseDirectFeedXML(resp)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch podcast feed")
}

func readAndParseDirectFeedXML(resp *http.Response) ([]FeedEpisode, error) {
	defer resp.Body.Close()
	rawXML, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, err
	}
	return parseRSSFeedXML(rawXML)
}

func (c *PodFetchBackend) addPendingDownload(podcastID, epID string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.pendingEps == nil {
		c.pendingEps = make(map[string]map[string]bool)
	}
	if c.pendingEps[podcastID] == nil {
		c.pendingEps[podcastID] = make(map[string]bool)
	}
	c.pendingEps[podcastID][epID] = true
}

func (c *PodFetchBackend) findEpisodeIDForDownload(podcastID string, ep FeedEpisode) (string, bool, error) {
	if c.DBPath != "" {
		if db, err := getPodfetchDB(c.DBPath); err == nil {
			var epID, fileEpPath sql.NullString
			query := "SELECT episode_id, file_episode_path FROM podcast_episodes WHERE podcast_id = ? AND (guid = ? OR url = ? OR lower(name) = lower(?)) LIMIT 1"
			title := strings.TrimSpace(ep.Title)
			if err := db.QueryRow(query, podcastID, ep.GUID, ep.EnclosureURL, title).Scan(&epID, &fileEpPath); err == nil {
				isDone := fileEpPath.Valid && strings.TrimSpace(fileEpPath.String) != ""
				return epID.String, isDone, nil
			}
		}
	}
	if c.Host != "" {
		body, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/episodes", podcastID), "GET", nil)
		if err == nil {
			for _, d := range unmarshalPodFetchEpisodes(body) {
				match := false
				if ep.GUID != "" && d.GUID == ep.GUID {
					match = true
				} else if ep.EnclosureURL != "" && d.URL == ep.EnclosureURL {
					match = true
				} else if strings.EqualFold(d.Name, ep.Title) {
					match = true
				}
				if match {
					isDone := isPodFetchEpisodeDownloaded(d, d.LocalURL)
					epID := d.EpisodeID
					if epID == "" {
						epID = fmt.Sprintf("%v", d.ID)
					}
					return epID, isDone, nil
				}
			}
		}
	}
	// An RSS GUID is not a PodFetch episode id, and PodFetch answers a download
	// request for an id it does not recognise with HTTP 200 and no action. So
	// guessing here does not fail loudly, it fails silently: the run reports the
	// episode queued and nothing is ever downloaded. Say so instead.
	return "", false, fmt.Errorf("episode %q is not in the podfetch catalog; refresh the feed first", ep.Title)
}

func (c *PodFetchBackend) DownloadEpisodes(podcastID string, episodes []FeedEpisode) error {
	if c.Host == "" {
		return nil
	}
	var failures []string
	for _, ep := range episodes {
		epID, isDone, err := c.findEpisodeIDForDownload(podcastID, ep)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if isDone {
			continue
		}
		_, err = c.Request(fmt.Sprintf("/api/v1/podcasts/%s/episodes/download", epID), "PUT", nil)
		if err != nil {
			_, err = c.Request(fmt.Sprintf("/api/v1/podcasts/episode/%s/download", epID), "POST", nil)
		}
		if err != nil {
			payload := map[string]interface{}{"episodes": []FeedEpisode{ep}}
			_, err = c.Request(fmt.Sprintf("/api/v1/podcasts/%s/download", podcastID), "POST", payload)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("queue %q: %v", ep.Title, err))
		} else {
			c.addPendingDownload(podcastID, epID)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func (c *PodFetchBackend) DeletePodcastEpisode(podcastID, episodeID string) error {
	var lastErr error
	if c.Host != "" {
		endpoint := fmt.Sprintf("/api/v1/podcasts/%s/episodes/%s", podcastID, episodeID)
		_, err := c.Request(endpoint, "DELETE", nil)
		if err != nil {
			endpoint = fmt.Sprintf("/api/v1/podcasts/episode/%s", episodeID)
			_, err = c.Request(endpoint, "DELETE", nil)
		}
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if c.DBPath != "" {
		return deletePodFetchEpisodeDB(c.DBPath, podcastID, episodeID)
	}
	return lastErr
}

func (c *PodFetchBackend) isEpisodeDownloadFinished(podcastID, epID string) bool {
	if c.DBPath != "" {
		if db, err := getPodfetchDB(c.DBPath); err == nil {
			var fileEpPath sql.NullString
			query := "SELECT file_episode_path FROM podcast_episodes WHERE episode_id = ? OR id = ? LIMIT 1"
			if err := db.QueryRow(query, epID, epID).Scan(&fileEpPath); err == nil {
				return fileEpPath.Valid && strings.TrimSpace(fileEpPath.String) != ""
			}
		}
	}
	if c.Host != "" {
		body, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/episodes", podcastID), "GET", nil)
		if err == nil {
			for _, d := range unmarshalPodFetchEpisodes(body) {
				if d.EpisodeID == epID || fmt.Sprintf("%v", d.ID) == epID {
					return isPodFetchEpisodeDownloaded(d, d.LocalURL)
				}
			}
		}
	}
	return false
}

func (c *PodFetchBackend) ActiveDownloads(podcastID string) ([]ActiveDownload, error) {
	if c.Host != "" {
		body, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/downloads", podcastID), "GET", nil)
		if err != nil {
			body, err = c.Request("/api/v1/podcasts/downloads", "GET", nil)
		}
		if err == nil {
			var dtos []podFetchEpisodeDTO
			if json.Unmarshal(body, &dtos) == nil && len(dtos) > 0 {
				var active []ActiveDownload
				for _, d := range dtos {
					active = append(active, ActiveDownload{
						ID:                  fmt.Sprintf("%v", d.ID),
						EpisodeDisplayTitle: d.Name,
						Title:               d.Name,
						EpisodeID:           d.EpisodeID,
						URL:                 d.URL,
					})
				}
				return active, nil
			}
		}
	}

	var active []ActiveDownload

	c.pendingMu.Lock()
	if c.pendingEps != nil && len(c.pendingEps[podcastID]) > 0 {
		for epID := range c.pendingEps[podcastID] {
			if c.isEpisodeDownloadFinished(podcastID, epID) {
				delete(c.pendingEps[podcastID], epID)
			} else {
				active = append(active, ActiveDownload{
					ID:        epID,
					EpisodeID: epID,
				})
			}
		}
	}
	c.pendingMu.Unlock()

	if c.DBPath != "" {
		if dbDls, err := fetchActiveDownloadsDB(c.DBPath, podcastID); err == nil {
			active = append(active, dbDls...)
		}
	}

	return active, nil
}

func (c *PodFetchBackend) OpenRSSFeed(podcastID, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = c.Host
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return fmt.Sprintf("%s/rss/%s", baseURL, podcastID), nil
}

func (c *PodFetchBackend) DownloadCover(podcastID, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}
	if c.Host != "" {
		url := fmt.Sprintf("/api/v1/podcasts/%s/cover", podcastID)
		data, err := c.Request(url, "GET", nil)
		if err == nil && len(data) > 0 {
			_ = os.MkdirAll(filepath.Dir(destPath), 0755)
			return os.WriteFile(destPath, data, 0644)
		}
	}
	pod, err := c.GetPodcast(podcastID)
	if err == nil && pod != nil && pod.Media.Metadata.ImageURL != "" {
		imgURL := pod.Media.Metadata.ImageURL
		if strings.HasPrefix(imgURL, "http://") || strings.HasPrefix(imgURL, "https://") {
			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Get(imgURL)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					const maxImageSize = 10 * 1024 * 1024
					data, _ := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
					if len(data) > 0 {
						_ = os.MkdirAll(filepath.Dir(destPath), 0755)
						return os.WriteFile(destPath, data, 0644)
					}
				}
			}
		}
	}
	return nil
}
