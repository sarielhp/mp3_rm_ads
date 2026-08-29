package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Library struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
}

type AudioFileMetadata struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	CTimeMs int64  `json:"ctimeMs"`
	MTimeMs int64  `json:"mtimeMs"`
}

type PodcastAudioFile struct {
	Duration float64            `json:"duration"`
	AddedAt  int64              `json:"addedAt"`
	Metadata *AudioFileMetadata `json:"metadata,omitempty"`
}

type PodcastEpisode struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	PubDate      string            `json:"pubDate"`
	PublishedAt  int64             `json:"publishedAt"`
	EnclosureURL string            `json:"enclosureURL"`
	GUID         string            `json:"guid"`
	Duration     float64           `json:"duration"`
	AudioFile    *PodcastAudioFile `json:"audioFile,omitempty"`
}

type PodcastMetadata struct {
	Title   string `json:"title"`
	FeedURL string `json:"feedUrl"`
}

type PodcastMedia struct {
	ID       string           `json:"id"`
	Metadata PodcastMetadata  `json:"metadata"`
	Episodes []PodcastEpisode `json:"episodes"`
}

type PodcastItem struct {
	ID    string       `json:"id"`
	Media PodcastMedia `json:"media"`
}

type FeedEnclosure struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type FeedEpisode struct {
	Title            string         `json:"title"`
	Subtitle         string         `json:"subtitle,omitempty"`
	Description      string         `json:"description,omitempty"`
	DescriptionPlain string         `json:"descriptionPlain,omitempty"`
	PubDate          string         `json:"pubDate"`
	PublishedAt      int64          `json:"publishedAt"`
	EpisodeType      string         `json:"episodeType,omitempty"`
	Season           string         `json:"season,omitempty"`
	Episode          string         `json:"episode,omitempty"`
	DurationSeconds  float64        `json:"durationSeconds,omitempty"`
	GUID             string         `json:"guid,omitempty"`
	Enclosure        *FeedEnclosure `json:"enclosure,omitempty"`
}

func (f *FeedEpisode) UnmarshalJSON(data []byte) error {
	type Alias FeedEpisode
	aux := &struct {
		PublishedAt interface{} `json:"publishedAt"`
		Season      interface{} `json:"season"`
		Episode     interface{} `json:"episode"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.PublishedAt != nil {
		switch v := aux.PublishedAt.(type) {
		case float64:
			f.PublishedAt = int64(v)
		case int64:
			f.PublishedAt = v
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.PublishedAt = n
			}
		}
	}

	if aux.Season != nil {
		if m, ok := aux.Season.(map[string]interface{}); ok {
			if n, ok := m["number"]; ok && n != nil {
				f.Season = fmt.Sprintf("%v", n)
			}
		} else {
			f.Season = fmt.Sprintf("%v", aux.Season)
		}
	}

	if aux.Episode != nil {
		f.Episode = fmt.Sprintf("%v", aux.Episode)
	}

	return nil
}

type ActiveDownload struct {
	ID                  string `json:"id"`
	EpisodeDisplayTitle string `json:"episodeDisplayTitle"`
}

type SimplecastEpisode struct {
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
