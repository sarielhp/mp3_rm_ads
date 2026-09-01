package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

func (c *AudiobookshelfBackend) PodcastFeedEpisodes(feedURL string) ([]FeedEpisode, error) {
	body, err := c.Request("/api/podcasts/feed", "POST", map[string]string{"rssFeed": feedURL})
	if err != nil {
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

func (c *AudiobookshelfBackend) DownloadEpisodes(podcastID string, episodes []FeedEpisode) error {
	_, err := c.Request(fmt.Sprintf("/api/podcasts/%s/download-episodes", podcastID), "POST", episodes)
	return err
}

func (c *AudiobookshelfBackend) DeletePodcastEpisode(podcastID, episodeID string) error {
	_, err := c.Request(fmt.Sprintf("/api/podcasts/%s/episode/%s?hard=1", podcastID, episodeID), "DELETE", nil)
	return err
}

func (c *AudiobookshelfBackend) ActiveDownloads(podcastID string) ([]ActiveDownload, error) {
	body, err := c.Request(fmt.Sprintf("/api/podcasts/%s/downloads", podcastID), "GET", nil)
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

func (c *AudiobookshelfBackend) OpenRSSFeed(podcastID, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = c.Host
	}
	baseURL = strings.TrimRight(baseURL, "/")

	item, _ := c.GetPodcast(podcastID)
	if item != nil && item.RSSFeed != nil && item.RSSFeed.Slug != "" {
		return item.RSSFeed.Slug, nil
	}

	payload := map[string]interface{}{
		"serverAddress": baseURL,
		"slug":          podcastID,
	}
	body, err := c.Request(fmt.Sprintf("/api/feeds/item/%s/open", podcastID), "POST", payload)
	if err == nil {
		var res struct {
			Slug string `json:"slug"`
			Feed struct {
				Slug string `json:"slug"`
			} `json:"feed"`
		}
		if json.Unmarshal(body, &res) == nil {
			slug := res.Feed.Slug
			if slug == "" {
				slug = res.Slug
			}
			if slug != "" {
				return slug, nil
			}
		}
		return podcastID, nil
	}

	feedsBody, fErr := c.Request("/api/feeds", "GET", nil)
	if fErr == nil {
		var feedsResp struct {
			Feeds []struct {
				EntityID string `json:"entityId"`
				Slug     string `json:"slug"`
			} `json:"feeds"`
		}
		if json.Unmarshal(feedsBody, &feedsResp) == nil {
			for _, f := range feedsResp.Feeds {
				if f.EntityID == podcastID && f.Slug != "" {
					return f.Slug, nil
				}
			}
		}
	}

	return "", err
}

func (c *AudiobookshelfBackend) DownloadCover(podcastID, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}
	url := fmt.Sprintf("/api/items/%s/cover", podcastID)
	data, err := c.Request(url, "GET", nil)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

type simplecastEpisode struct {
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	PublishedAt  string      `json:"published_at"`
	Type         string      `json:"type"`
	Season       interface{} `json:"season"`
	Number       interface{} `json:"number"`
	Duration     float64     `json:"duration"`
	GUID         string      `json:"guid"`
	EnclosureURL string      `json:"enclosure_url"`
}

func fetchSimplecastEpisodes(collectionID string) ([]FeedEpisode, error) {
	var episodes []FeedEpisode
	offset := 0
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		urlStr := fmt.Sprintf("https://api.simplecast.com/podcasts/%s/episodes?limit=100&offset=%d&private=false&sort=latest&status=published", collectionID, offset)
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			break
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		res, err := client.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			if res != nil {
				res.Body.Close()
			}
			break
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			break
		}

		var data struct {
			Collection []simplecastEpisode `json:"collection"`
			Pages      struct {
				Next interface{} `json:"next"`
			} `json:"pages"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			break
		}

		if len(data.Collection) == 0 {
			break
		}

		for _, scEp := range data.Collection {
			var pubMs int64
			pubStr := scEp.PublishedAt
			if pubStr != "" {
				if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
					pubMs = t.UnixNano() / 1e6
					pubStr = t.Format("Mon, 02 Jan 2006 15:04:05 -0700")
				} else if t, err := time.Parse(time.RFC3339Nano, pubStr); err == nil {
					pubMs = t.UnixNano() / 1e6
					pubStr = t.Format("Mon, 02 Jan 2006 15:04:05 -0700")
				}
			}

			seasonVal := ""
			if scEp.Season != nil {
				if m, ok := scEp.Season.(map[string]interface{}); ok {
					if n, ok := m["number"]; ok && n != nil {
						seasonVal = fmt.Sprintf("%v", n)
					}
				} else {
					seasonVal = fmt.Sprintf("%v", scEp.Season)
				}
			}

			epVal := ""
			if scEp.Number != nil {
				epVal = fmt.Sprintf("%v", scEp.Number)
			}

			epType := scEp.Type
			if epType == "" {
				epType = "full"
			}

			episodes = append(episodes, FeedEpisode{
				Title:            scEp.Title,
				Subtitle:         scEp.Description,
				Description:      scEp.Description,
				DescriptionPlain: scEp.Description,
				PubDate:          pubStr,
				PublishedAt:      pubMs,
				EpisodeType:      epType,
				Season:           seasonVal,
				Episode:          epVal,
				DurationSeconds:  scEp.Duration,
				GUID:             scEp.GUID,
				Enclosure: &FeedEnclosure{
					URL:  scEp.EnclosureURL,
					Type: "audio/mpeg",
				},
			})
		}

		if data.Pages.Next == nil {
			break
		}
		if s, ok := data.Pages.Next.(string); ok && s == "" {
			break
		}

		offset += 100
	}

	return episodes, nil
}
