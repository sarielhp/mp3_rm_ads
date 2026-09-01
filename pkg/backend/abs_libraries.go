package backend

import (
	"encoding/json"
	"fmt"
)

func (c *AudiobookshelfBackend) Libraries() ([]Library, error) {
	body, err := c.Request("/api/libraries", "GET", nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Libraries []Library `json:"libraries"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.Libraries, nil
}

func (c *AudiobookshelfBackend) PodcastLibraries() ([]Library, error) {
	libs, err := c.Libraries()
	if err != nil {
		return nil, err
	}

	var result []Library
	for _, lib := range libs {
		if lib.MediaType == "podcast" {
			result = append(result, lib)
		}
	}
	return result, nil
}

func (c *AudiobookshelfBackend) GetPodcast(id string) (*Podcast, error) {
	body, err := c.Request(fmt.Sprintf("/api/items/%s", id), "GET", nil)
	if err != nil {
		return nil, err
	}

	var item Podcast
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *AudiobookshelfBackend) GetItem(id string) (*Podcast, error) {
	return c.GetPodcast(id)
}

func (c *AudiobookshelfBackend) Podcasts() ([]Podcast, error) {
	libs, err := c.PodcastLibraries()
	if err != nil {
		return nil, err
	}

	var podcasts []Podcast
	for _, lib := range libs {
		body, err := c.Request(fmt.Sprintf("/api/libraries/%s/items", lib.ID), "GET", nil)
		if err != nil {
			return nil, err
		}

		var res struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}

		type itemResult struct {
			idx  int
			item *Podcast
			err  error
		}
		resChan := make(chan itemResult, len(res.Results))
		sem := make(chan struct{}, 10)

		for i, itemSummary := range res.Results {
			go func(idx int, id string) {
				sem <- struct{}{}
				defer func() { <-sem }()
				detailed, err := c.GetPodcast(id)
				resChan <- itemResult{idx: idx, item: detailed, err: err}
			}(i, itemSummary.ID)
		}

		ordered := make([]*Podcast, len(res.Results))
		var firstErr error
		for i := 0; i < len(res.Results); i++ {
			r := <-resChan
			if r.err != nil && firstErr == nil {
				firstErr = r.err
			}
			ordered[r.idx] = r.item
		}
		if firstErr != nil {
			return nil, firstErr
		}
		for _, item := range ordered {
			if item != nil {
				podcasts = append(podcasts, *item)
			}
		}
	}

	return podcasts, nil
}

func (c *AudiobookshelfBackend) PodcastItems() ([]Podcast, error) {
	return c.Podcasts()
}

func (c *AudiobookshelfBackend) CreatePodcast(libraryID, folderID, path, title, feedURL string) (*Podcast, error) {
	payload := map[string]interface{}{
		"libraryId": libraryID,
		"folderId":  folderID,
		"path":      path,
		"media": map[string]interface{}{
			"metadata": map[string]interface{}{
				"title":   title,
				"feedUrl": feedURL,
			},
		},
	}
	body, err := c.Request("/api/podcasts", "POST", payload)
	if err != nil {
		return nil, err
	}
	var item Podcast
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
