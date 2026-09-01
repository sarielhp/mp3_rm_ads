package backend

import (
	"fmt"
	"path/filepath"
)

type podFetchItemDTO struct {
	ID        interface{} `json:"id"`
	Name      string      `json:"name"`
	Title     string      `json:"title"`
	Directory string      `json:"directory"`
	RSSFeed   string      `json:"rssfeed"`
	FeedURL   string      `json:"feed_url"`
	URL       string      `json:"url"`
	ImageURL  string      `json:"image_url"`
	Summary   string      `json:"summary"`
	Desc      string      `json:"description"`
	Author    string      `json:"author"`
}

type podFetchEpisodeDTO struct {
	ID              interface{} `json:"id"`
	PodcastID       interface{} `json:"podcast_id"`
	EpisodeID       string      `json:"episode_id"`
	GUID            string      `json:"guid"`
	Name            string      `json:"name"`
	Title           string      `json:"title"`
	URL             string      `json:"url"`
	DateOfRecording string      `json:"date_of_recording"`
	PubDate         string      `json:"pub_date"`
	Date            string      `json:"date"`
	TotalTime       float64     `json:"total_time"`
	Duration        float64     `json:"duration"`
	LocalURL        string      `json:"local_url"`
	FilePath        string      `json:"file_path"`
	Description     string      `json:"description"`
	Summary         string      `json:"summary"`
	Status          string      `json:"status"`
}

func mapPodFetchDTOToPodcast(dto podFetchItemDTO, epDTOs []podFetchEpisodeDTO) Podcast {
	idStr := fmt.Sprintf("%v", dto.ID)
	title := dto.Name
	if title == "" {
		title = dto.Title
	}
	feedURL := dto.RSSFeed
	if feedURL == "" {
		feedURL = dto.FeedURL
	}
	if feedURL == "" {
		feedURL = dto.URL
	}
	summary := dto.Summary
	if summary == "" {
		summary = dto.Desc
	}
	dir := dto.Directory
	if dir == "" {
		dir = sanitizePodcastName(title)
	}

	var episodes []Episode
	for _, epDTO := range epDTOs {
		episodes = append(episodes, mapPodFetchDTOToEpisode(epDTO))
	}

	return Podcast{
		ID:        idStr,
		RelPath:   dir,
		MediaType: "podcast",
		Media: PodcastMedia{
			ID: idStr,
			Metadata: PodcastMetadata{
				Title:       title,
				Author:      dto.Author,
				Description: summary,
				FeedURL:     feedURL,
				ImageURL:    dto.ImageURL,
			},
			Episodes: episodes,
		},
	}
}

func mapPodFetchDTOToEpisode(dto podFetchEpisodeDTO) Episode {
	epID := fmt.Sprintf("%v", dto.ID)
	title := dto.Name
	if title == "" {
		title = dto.Title
	}
	guid := dto.EpisodeID
	if guid == "" {
		guid = dto.GUID
	}
	dateStr := dto.DateOfRecording
	if dateStr == "" {
		dateStr = dto.PubDate
	}
	if dateStr == "" {
		dateStr = dto.Date
	}
	dur := dto.TotalTime
	if dur <= 0 {
		dur = dto.Duration
	}
	desc := dto.Description
	if desc == "" {
		desc = dto.Summary
	}
	locURL := dto.LocalURL
	if locURL == "" {
		locURL = dto.FilePath
	}

	ep := Episode{
		ID:           epID,
		Title:        title,
		GUID:         guid,
		PubDate:      dateStr,
		PublishedAt:  ParsePubDate(dateStr),
		Duration:     dur,
		Description:  desc,
		EnclosureURL: dto.URL,
	}

	if locURL != "" {
		ep.AudioFile = &PodcastAudioFile{
			Duration: dur,
			Metadata: &AudioFileMetadata{
				Filename: filepath.Base(locURL),
				Path:     locURL,
				RelPath:  locURL,
			},
		}
	}

	return ep
}

func mapPodFetchDTOToFeedEpisode(dto podFetchEpisodeDTO) FeedEpisode {
	title := dto.Name
	if title == "" {
		title = dto.Title
	}
	guid := dto.EpisodeID
	if guid == "" {
		guid = dto.GUID
	}
	dateStr := dto.DateOfRecording
	if dateStr == "" {
		dateStr = dto.PubDate
	}
	if dateStr == "" {
		dateStr = dto.Date
	}
	dur := dto.TotalTime
	if dur <= 0 {
		dur = dto.Duration
	}
	desc := dto.Description
	if desc == "" {
		desc = dto.Summary
	}

	return FeedEpisode{
		Title:           title,
		GUID:            guid,
		PubDate:         dateStr,
		PublishedAt:     ParsePubDate(dateStr),
		DurationSeconds: dur,
		Description:     desc,
		EnclosureURL:    dto.URL,
		Enclosure: &FeedEnclosure{
			URL:  dto.URL,
			Type: "audio/mpeg",
		},
	}
}
