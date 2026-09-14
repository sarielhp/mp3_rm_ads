package backend

import (
	"encoding/json"
	"fmt"
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

func unmarshalPodFetchPodcastList(body []byte) ([]podFetchItemDTO, error) {
	var dtos []podFetchItemDTO
	if err := json.Unmarshal(body, &dtos); err == nil {
		return dtos, nil
	}
	var wrapper struct {
		Podcasts []podFetchItemDTO `json:"podcasts"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Podcasts, nil
}
