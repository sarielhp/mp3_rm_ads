package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ABSClient struct {
	Host        string
	Token       string
	httpClient  *http.Client
	MaxAttempts int
	RetryDelay  time.Duration
	Quiet       bool
	Verbose     bool
	reqMu       sync.Mutex
}

func NewABSClient(host, token string) *ABSClient {
	return &ABSClient{
		Host:  strings.TrimRight(host, "/"),
		Token: token,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		MaxAttempts: 10,
		RetryDelay:  0,
	}
}

func (c *ABSClient) getRetryDelay(attempt int) time.Duration {
	if c.RetryDelay > 0 {
		return c.RetryDelay
	}
	if attempt <= 5 {
		return 1 * time.Second
	}
	return 2 * time.Second
}

func (c *ABSClient) Request(endpoint, method string, data interface{}) ([]byte, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	reqURL := fmt.Sprintf("%s%s", c.Host, endpoint)

	var jsonData []byte
	var err error
	if data != nil {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	maxAttempts := c.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	var lastErr error
	var lastStatusCode int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryDelay := c.getRetryDelay(attempt)

		var req *http.Request
		if jsonData != nil {
			req, err = http.NewRequest(method, reqURL, bytes.NewBuffer(jsonData))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		} else {
			req, err = http.NewRequest(method, reqURL, nil)
		}
		if err != nil {
			return nil, err
		}

		if c.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Failed to connect (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			fmt.Fprintf(os.Stderr, "[-] Connection error to %s after %d attempts: %v\n", reqURL, maxAttempts, err)
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Failed reading response body (attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			return nil, err
		}

		if res.StatusCode >= 500 || res.StatusCode == 429 || res.StatusCode == 408 {
			lastStatusCode = res.StatusCode
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTTP %d (attempt %d/%d). Waiting for container to wake up (retrying in %v)...\n", res.StatusCode, attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", res.StatusCode, reqURL, string(body))
			return nil, fmt.Errorf("HTTP Error %d", res.StatusCode)
		}

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			fmt.Fprintf(os.Stderr, "[-] HTTP Error %d for %s: %s\n", res.StatusCode, reqURL, string(body))
			return nil, fmt.Errorf("HTTP Error %d", res.StatusCode)
		}

		trimmed := bytes.TrimSpace(body)
		isHTML := bytes.HasPrefix(trimmed, []byte("<")) || strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html")
		if isHTML {
			if attempt < maxAttempts {
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML (Sablier/server waking up, attempt %d/%d). Retrying in %v...\n", attempt, maxAttempts, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			fmt.Fprintf(os.Stderr, "[-] Audiobookshelf returned HTML instead of JSON after %d attempts for %s: %s\n", maxAttempts, reqURL, string(body))
			return nil, fmt.Errorf("server returned HTML response instead of JSON (server may still be starting up)")
		}

		return body, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("HTTP Error %d", lastStatusCode)
}

func (c *ABSClient) PodcastLibraries() ([]Library, error) {
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

	var result []Library
	for _, lib := range res.Libraries {
		if lib.MediaType == "podcast" {
			result = append(result, lib)
		}
	}
	return result, nil
}

func (c *ABSClient) GetItem(itemID string) (*PodcastItem, error) {
	body, err := c.Request(fmt.Sprintf("/api/items/%s", itemID), "GET", nil)
	if err != nil {
		return nil, err
	}

	var item PodcastItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *ABSClient) PodcastItems() ([]PodcastItem, error) {
	libs, err := c.PodcastLibraries()
	if err != nil {
		return nil, err
	}

	var podcasts []PodcastItem
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
			item *PodcastItem
			err  error
		}
		resChan := make(chan itemResult, len(res.Results))
		sem := make(chan struct{}, 10)

		for i, itemSummary := range res.Results {
			go func(idx int, id string) {
				sem <- struct{}{}
				defer func() { <-sem }()
				detailed, err := c.GetItem(id)
				resChan <- itemResult{idx: idx, item: detailed, err: err}
			}(i, itemSummary.ID)
		}

		ordered := make([]*PodcastItem, len(res.Results))
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

func (c *ABSClient) PodcastFeedEpisodes(feedURL string) ([]FeedEpisode, error) {
	cached := globalFeedCache.Get(feedURL)
	var cachedETag, cachedLastMod string
	if cached != nil {
		cachedETag = cached.ETag
		cachedLastMod = cached.LastModified
	}

	directEps, newETag, newLastMod, notModified, err := fetchFeedDirect(feedURL, cachedETag, cachedLastMod)
	if err == nil {
		if notModified && cached != nil && len(cached.Episodes) > 0 {
			return cached.Episodes, nil
		}
		if len(directEps) > 0 {
			var latestGUID string
			if len(directEps) > 0 {
				latestGUID = directEps[0].GUID
			}
			globalFeedCache.Put(feedURL, &FeedCacheEntry{
				FeedURL:      feedURL,
				ETag:         newETag,
				LastModified: newLastMod,
				LastChecked:  time.Now(),
				LatestGUID:   latestGUID,
				Episodes:     directEps,
			})
			_ = globalFeedCache.Save()
			return directEps, nil
		}
	}

	body, err := c.Request("/api/podcasts/feed", "POST", map[string]string{"rssFeed": feedURL})
	if err != nil {
		if cached != nil && len(cached.Episodes) > 0 {
			return cached.Episodes, nil
		}
		return nil, err
	}

	var res struct {
		Podcast struct {
			Episodes []FeedEpisode `json:"episodes"`
		} `json:"podcast"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	episodes := res.Podcast.Episodes

	urlsToCheck := []string{feedURL}
	for _, ep := range episodes {
		if ep.Enclosure != nil && ep.Enclosure.URL != "" {
			urlsToCheck = append(urlsToCheck, ep.Enclosure.URL)
		}
	}

	hasSimplecast := false
	for _, u := range urlsToCheck {
		if strings.Contains(strings.ToLower(u), "simplecast") {
			hasSimplecast = true
			break
		}
	}

	if hasSimplecast {
		func() {
			client := &http.Client{Timeout: 10 * time.Second}
			req, err := http.NewRequest("GET", feedURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0")

			resFeed, err := client.Do(req)
			if err != nil || resFeed.StatusCode != http.StatusOK {
				if resFeed != nil {
					resFeed.Body.Close()
				}
				return
			}

			rawXML, err := io.ReadAll(resFeed.Body)
			resFeed.Body.Close()
			if err != nil {
				return
			}

			re := regexp.MustCompile(`awCollectionId=([0-9a-f-]{36})`)
			matches := re.FindStringSubmatch(string(rawXML))
			if len(matches) > 1 {
				cid := matches[1]
				scEps, err := fetchSimplecastEpisodes(cid)
				if err == nil && len(scEps) > len(episodes) {
					episodes = scEps
				}
			}
		}()
	}

	return episodes, nil
}

func (c *ABSClient) DownloadEpisodes(itemID string, episodeObjects []FeedEpisode) error {
	_, err := c.Request(fmt.Sprintf("/api/podcasts/%s/download-episodes", itemID), "POST", episodeObjects)
	return err
}

func (c *ABSClient) DeletePodcastEpisode(itemID, episodeID string) error {
	_, err := c.Request(fmt.Sprintf("/api/podcasts/%s/episode/%s?hard=1", itemID, episodeID), "DELETE", nil)
	return err
}

func (c *ABSClient) ActiveDownloads(itemID string) ([]ActiveDownload, error) {
	body, err := c.Request(fmt.Sprintf("/api/podcasts/%s/downloads", itemID), "GET", nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Downloads []ActiveDownload `json:"downloads"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.Downloads, nil
}
