package backend

import (
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
	if c.Host != "" {
		body, err := c.Request("/api/v1/podcasts", "GET", nil)
		if err != nil {
			if c.DBPath != "" {
				return fetchPodFetchPodcastsDB(c.DBPath)
			}
			return nil, err
		}

		var dtos []podFetchItemDTO
		if err := json.Unmarshal(body, &dtos); err != nil {
			var wrapper struct {
				Podcasts []podFetchItemDTO `json:"podcasts"`
			}
			if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
				if c.DBPath != "" {
					return fetchPodFetchPodcastsDB(c.DBPath)
				}
				return nil, err
			}
			dtos = wrapper.Podcasts
		}

		var podcasts []Podcast
		for _, dto := range dtos {
			idStr := fmt.Sprintf("%v", dto.ID)
			pod, err := c.GetPodcast(idStr)
			if err == nil && pod != nil {
				podcasts = append(podcasts, *pod)
			} else {
				podcasts = append(podcasts, mapPodFetchDTOToPodcast(dto, nil))
			}
		}
		return podcasts, nil
	}

	if c.DBPath != "" {
		return fetchPodFetchPodcastsDB(c.DBPath)
	}

	return nil, fmt.Errorf("neither host nor db_path configured for podfetch")
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
		if err := json.Unmarshal(body, &detailed); err == nil && detailed.ID != nil {
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
			_ = json.Unmarshal(epBody, &epDTOs)
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

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawXML, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, err
	}

	return parseRSSFeedXML(rawXML)
}

func (c *PodFetchBackend) DownloadEpisodes(podcastID string, episodes []FeedEpisode) error {
	if c.Host != "" {
		payload := map[string]interface{}{
			"episodes": episodes,
		}
		_, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/download", podcastID), "POST", payload)
		if err == nil {
			return nil
		}
		for _, ep := range episodes {
			epID := ep.GUID
			if epID == "" {
				epID = ep.Title
			}
			_, _ = c.Request(fmt.Sprintf("/api/v1/podcasts/episode/%s/download", epID), "POST", nil)
		}
		return nil
	}
	return nil
}

func (c *PodFetchBackend) DeletePodcastEpisode(podcastID, episodeID string) error {
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
	}
	if c.DBPath != "" {
		return deletePodFetchEpisodeDB(c.DBPath, podcastID, episodeID)
	}
	return nil
}

func (c *PodFetchBackend) ActiveDownloads(podcastID string) ([]ActiveDownload, error) {
	if c.Host != "" {
		body, err := c.Request(fmt.Sprintf("/api/v1/podcasts/%s/downloads", podcastID), "GET", nil)
		if err != nil {
			body, err = c.Request("/api/v1/podcasts/downloads", "GET", nil)
		}
		if err == nil {
			var dtos []podFetchEpisodeDTO
			if json.Unmarshal(body, &dtos) == nil {
				var dls []ActiveDownload
				for _, d := range dtos {
					dls = append(dls, ActiveDownload{
						ID:                  fmt.Sprintf("%v", d.ID),
						EpisodeDisplayTitle: d.Name,
						Title:               d.Name,
						EpisodeID:           d.EpisodeID,
						URL:                 d.URL,
					})
				}
				return dls, nil
			}
		}
	}
	if c.DBPath != "" {
		return fetchActiveDownloadsDB(c.DBPath, podcastID)
	}
	return nil, nil
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
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				data, _ := io.ReadAll(resp.Body)
				if len(data) > 0 {
					_ = os.MkdirAll(filepath.Dir(destPath), 0755)
					return os.WriteFile(destPath, data, 0644)
				}
			}
		}
	}
	return nil
}
