package tui

import (
	"abs/pkg/config"
	"abs/pkg/player"
	"abs/pkg/podcast"
	"abs/pkg/types"
)

type (
	Config               = types.Config
	PodcastConfig        = config.PodcastConfig
	DownloadQueueItem    = podcast.DownloadQueueItem
	UnifiedQueueItem     = types.UnifiedQueueItem
	TranscriptionData    = types.TranscriptionData
	TranscriptionSegment = types.TranscriptionSegment
	CutEntry             = types.CutEntry
	CutsData             = types.CutsData
	PlayerTrack          = types.PlayerTrack
	TUIColorConfig       = types.TUIColorConfig

	CachedPodcastIndex   = podcast.CachedPodcastIndex
	CachedEpisodeSummary = podcast.CachedEpisodeSummary
	CachedEpisodeDetails = podcast.CachedEpisodeDetails
)

const (
	AdRemovalNone   = config.AdRemovalNone
	AdRemovalAll    = config.AdRemovalAll
	AdRemovalLatest = config.AdRemovalLatest

	DownloadPolicyLatestK = config.DownloadPolicyLatestK
	DownloadPolicyAll     = config.DownloadPolicyAll
)

var globalPlayer = player.GetGlobalPlayer()
