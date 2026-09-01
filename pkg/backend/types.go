package backend

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

type LibraryFolder struct {
	ID        string `json:"id"`
	FullPath  string `json:"fullPath"`
	LibraryID string `json:"libraryId"`
}

type Library struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	MediaType string          `json:"mediaType"`
	Folders   []LibraryFolder `json:"folders"`
}

type AudioFileMetadata struct {
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path"`
	RelPath  string `json:"relPath,omitempty"`
	Size     int64  `json:"size"`
	CTimeMs  int64  `json:"ctimeMs,omitempty"`
	MTimeMs  int64  `json:"mtimeMs,omitempty"`
}

type PodcastAudioFile struct {
	Duration      float64            `json:"duration"`
	BitRate       int                `json:"bitRate,omitempty"`
	Codec         string             `json:"codec,omitempty"`
	Channels      int                `json:"channels,omitempty"`
	ChannelLayout string             `json:"channelLayout,omitempty"`
	Format        string             `json:"format,omitempty"`
	AddedAt       int64              `json:"addedAt,omitempty"`
	Metadata      *AudioFileMetadata `json:"metadata,omitempty"`
}

type Episode struct {
	ID           string            `json:"id"`
	Index        int               `json:"index,omitempty"`
	Season       string            `json:"season,omitempty"`
	Episode      string            `json:"episode,omitempty"`
	EpisodeType  string            `json:"episodeType,omitempty"`
	Title        string            `json:"title"`
	Subtitle     string            `json:"subtitle,omitempty"`
	Description  string            `json:"description,omitempty"`
	PubDate      string            `json:"pubDate,omitempty"`
	PublishedAt  int64             `json:"publishedAt,omitempty"`
	Duration     float64           `json:"duration,omitempty"`
	Size         int64             `json:"size,omitempty"`
	GUID         string            `json:"guid,omitempty"`
	EnclosureURL string            `json:"enclosureURL,omitempty"`
	AudioFile    *PodcastAudioFile `json:"audioFile,omitempty"`
}

type PodcastMetadata struct {
	Title       string   `json:"title"`
	Author      string   `json:"author,omitempty"`
	Description string   `json:"description,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Language    string   `json:"language,omitempty"`
	ReleaseDate string   `json:"releaseDate,omitempty"`
	FeedURL     string   `json:"feedUrl,omitempty"`
	ImageURL    string   `json:"imageUrl,omitempty"`
}

type PodcastMedia struct {
	ID        string          `json:"id,omitempty"`
	Metadata  PodcastMetadata `json:"metadata"`
	Episodes  []Episode       `json:"episodes,omitempty"`
	CoverPath string          `json:"coverPath,omitempty"`
}

type Podcast struct {
	ID        string       `json:"id"`
	Path      string       `json:"path,omitempty"`
	RelPath   string       `json:"relPath,omitempty"`
	MediaType string       `json:"mediaType,omitempty"`
	Media     PodcastMedia `json:"media"`
	RSSFeed   *struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"rssFeed,omitempty"`
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
	EnclosureURL     string         `json:"enclosureUrl,omitempty"`
	Enclosure        *FeedEnclosure `json:"enclosure,omitempty"`
}

func (f *FeedEpisode) UnmarshalJSON(data []byte) error {
	type Alias FeedEpisode
	aux := &struct {
		PublishedAt  interface{} `json:"publishedAt"`
		Season       interface{} `json:"season"`
		Episode      interface{} `json:"episode"`
		EnclosureURL string      `json:"enclosureUrl"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.EnclosureURL != "" && f.Enclosure == nil {
		f.Enclosure = &FeedEnclosure{URL: aux.EnclosureURL}
	}
	if f.Enclosure != nil && f.Enclosure.URL != "" && f.EnclosureURL == "" {
		f.EnclosureURL = f.Enclosure.URL
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
	DisplayTitle        string `json:"displayTitle"`
	Title               string `json:"title"`
	EpisodeID           string `json:"episodeId"`
	URL                 string `json:"url"`
	Episode             struct {
		Title        string `json:"title"`
		GUID         string `json:"guid"`
		EnclosureURL string `json:"enclosureUrl"`
	} `json:"episode"`
}

type OPMLFeed struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type OPMLDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    OPMLHead `xml:"head"`
	Body    OPMLBody `xml:"body"`
}

type OPMLHead struct {
	Title string `xml:"title"`
}

type OPMLBody struct {
	Outline OPMLOutlineGroup `xml:"outline"`
}

type OPMLOutlineGroup struct {
	Text     string        `xml:"text,attr"`
	Outlines []OPMLOutline `xml:"outline"`
}

type OPMLOutline struct {
	Type   string `xml:"type,attr"`
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

type PodcastCadence string

const (
	CadenceHourly       PodcastCadence = "hourly"
	CadenceDaily        PodcastCadence = "daily"
	CadenceWeekly       PodcastCadence = "weekly"
	CadenceMonthly      PodcastCadence = "monthly"
	CadenceIntermittent PodcastCadence = "intermittent"
)

type PodcastFrequencyInfo struct {
	Type                string    `json:"type"`
	EpisodesAnalyzed    int       `json:"episodes_analyzed"`
	AvgDaysInterval     float64   `json:"avg_days_interval"`
	MedianHoursInterval float64   `json:"median_hours_interval"`
	EpisodesPerWeek     float64   `json:"episodes_per_week"`
	AnalyzedAt          time.Time `json:"analyzed_at"`
}

type ScanOptions struct {
	PodcastsDir  string
	Quiet        bool
	Verbose      bool
	EpisodesOnly bool
	PodcastsOnly bool
}

type ScanResult struct {
	NewPodcasts     int
	CheckedPodcasts int
	NewEpisodes     int
	Podcasts        []Podcast
}

type RescanOptions struct {
	PodcastsDir string
	PodcastID   string
	DryRun      bool
	Verbose     bool
	Quiet       bool
}

type RescanResult struct {
	RescanCount  int
	CheckedCount int
}

type OPMLExportOptions struct {
	Quiet   bool
	Verbose bool
}

type OPMLImportOptions struct {
	Quiet   bool
	Verbose bool
}

type OPMLImportResult struct {
	Subscribed       int
	AlreadyExisted   int
	SkippedSelfFeeds int
	TotalFeeds       int
}
