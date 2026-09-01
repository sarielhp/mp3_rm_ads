package main

import "github.com/sariel/abs/pkg/backend"

type (
	Backend               = backend.Backend
	LibraryFolder         = backend.LibraryFolder
	Library               = backend.Library
	AudioFileMetadata     = backend.AudioFileMetadata
	PodcastAudioFile      = backend.PodcastAudioFile
	Episode               = backend.Episode
	PodcastEpisode        = backend.Episode
	PodcastMetadata       = backend.PodcastMetadata
	PodcastMedia          = backend.PodcastMedia
	Podcast               = backend.Podcast
	PodcastItem           = backend.Podcast
	FeedEnclosure         = backend.FeedEnclosure
	FeedEpisode           = backend.FeedEpisode
	ActiveDownload        = backend.ActiveDownload
	PodcastFeedInfo       = backend.OPMLFeed
	PodcastCadence        = backend.PodcastCadence
	PodcastFrequencyInfo  = backend.PodcastFrequencyInfo
	ScanOptions           = backend.ScanOptions
	ScanResult            = backend.ScanResult
	RescanOptions         = backend.RescanOptions
	RescanResult          = backend.RescanResult
	OPMLExportOptions     = backend.OPMLExportOptions
	OPMLImportOptions     = backend.OPMLImportOptions
	OPMLImportResult      = backend.OPMLImportResult
	AudiobookshelfBackend = backend.AudiobookshelfBackend
	ABSClient             = backend.AudiobookshelfBackend

	absItem      = backend.Podcast
	absEpisode   = backend.Episode
	absAudioFile = backend.PodcastAudioFile
	absLibrary   = backend.Library
	absFolder    = backend.LibraryFolder

	absLibrariesResp struct {
		Libraries []backend.Library `json:"libraries"`
	}
	absItemsResp struct {
		Results []backend.Podcast `json:"results"`
	}
	SimplecastEpisode struct {
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
)

const (
	CadenceHourly       = backend.CadenceHourly
	CadenceDaily        = backend.CadenceDaily
	CadenceWeekly       = backend.CadenceWeekly
	CadenceMonthly      = backend.CadenceMonthly
	CadenceIntermittent = backend.CadenceIntermittent
)
