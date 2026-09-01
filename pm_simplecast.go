package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func fetchSimplecastEpisodes(collectionID string) ([]FeedEpisode, error) {
	var episodes []FeedEpisode
	offset := 0
	client := &http.Client{Timeout: 60 * time.Second}

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
			Collection []SimplecastEpisode `json:"collection"`
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
