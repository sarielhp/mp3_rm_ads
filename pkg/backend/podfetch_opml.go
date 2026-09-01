package backend

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

func (c *PodFetchBackend) FetchPodcastFeeds(silent, verbose bool) ([]OPMLFeed, error) {
	podcasts, err := c.Podcasts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve podcasts: %w", err)
	}

	var feeds []OPMLFeed
	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = item.ID
		}
		feedURL := item.Media.Metadata.FeedURL
		if feedURL == "" {
			slug, err := c.OpenRSSFeed(item.ID, c.Host)
			if err == nil && slug != "" {
				feedURL = slug
			}
		}
		if feedURL != "" {
			feeds = append(feeds, OPMLFeed{
				Title: title,
				URL:   feedURL,
			})
		}
	}
	return feeds, nil
}

func (c *PodFetchBackend) ExportOPML(opts OPMLExportOptions) ([]byte, error) {
	feeds, err := c.FetchPodcastFeeds(opts.Quiet, opts.Verbose)
	if err != nil {
		return nil, err
	}
	return BuildOPMLXML(feeds)
}

func (c *PodFetchBackend) ImportOPML(data []byte, opts OPMLImportOptions) (OPMLImportResult, error) {
	feeds, err := ParseOPMLXML(data)
	if err != nil {
		return OPMLImportResult{}, err
	}

	existingItems, err := c.Podcasts()
	if err != nil && c.Host != "" {
		return OPMLImportResult{}, err
	}

	existingFeeds := make(map[string]bool)
	existingTitles := make(map[string]bool)
	for _, it := range existingItems {
		if u := strings.TrimSpace(it.Media.Metadata.FeedURL); u != "" {
			existingFeeds[strings.ToLower(u)] = true
		}
		if t := strings.TrimSpace(it.Media.Metadata.Title); t != "" {
			existingTitles[strings.ToLower(t)] = true
		}
	}

	res := OPMLImportResult{TotalFeeds: len(feeds)}
	for _, f := range feeds {
		normURL := strings.ToLower(strings.TrimSpace(f.URL))
		normTitle := strings.ToLower(strings.TrimSpace(f.Title))

		if existingFeeds[normURL] || existingTitles[normTitle] {
			res.AlreadyExisted++
			continue
		}

		safeTitle := sanitizePodcastTitle(f.Title)
		_, err := c.CreatePodcast("", "", safeTitle, f.Title, f.URL)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				res.AlreadyExisted++
				existingTitles[normTitle] = true
				continue
			}
			continue
		}

		existingFeeds[normURL] = true
		existingTitles[normTitle] = true
		res.Subscribed++
	}

	return res, nil
}

type rssFeedXML struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
			GUID        string `xml:"guid"`
			Duration    string `xml:"duration"`
			Season      string `xml:"season"`
			Episode     string `xml:"episode"`
			Enclosure   struct {
				URL  string `xml:"url,attr"`
				Type string `xml:"type,attr"`
			} `xml:"enclosure"`
		} `xml:"item"`
	} `xml:"channel"`
}

func parseRSSFeedXML(rawXML []byte) ([]FeedEpisode, error) {
	var feed rssFeedXML
	if err := xml.Unmarshal(rawXML, &feed); err != nil {
		return nil, err
	}

	var episodes []FeedEpisode
	for _, item := range feed.Channel.Items {
		var durSec float64
		if item.Duration != "" {
			parts := strings.Split(item.Duration, ":")
			if len(parts) == 3 {
				h, _ := strconv.ParseFloat(parts[0], 64)
				m, _ := strconv.ParseFloat(parts[1], 64)
				s, _ := strconv.ParseFloat(parts[2], 64)
				durSec = h*3600 + m*60 + s
			} else if len(parts) == 2 {
				m, _ := strconv.ParseFloat(parts[0], 64)
				s, _ := strconv.ParseFloat(parts[1], 64)
				durSec = m*60 + s
			} else if len(parts) == 1 {
				durSec, _ = strconv.ParseFloat(parts[0], 64)
			}
		}

		ep := FeedEpisode{
			Title:           item.Title,
			Subtitle:        item.Description,
			Description:     item.Description,
			PubDate:         item.PubDate,
			PublishedAt:     ParsePubDate(item.PubDate),
			DurationSeconds: durSec,
			GUID:            item.GUID,
			Season:          item.Season,
			Episode:         item.Episode,
			EnclosureURL:    item.Enclosure.URL,
			Enclosure: &FeedEnclosure{
				URL:  item.Enclosure.URL,
				Type: item.Enclosure.Type,
			},
		}
		episodes = append(episodes, ep)
	}

	return episodes, nil
}
