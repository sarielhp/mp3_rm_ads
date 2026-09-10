package adremoval

import (
	"abs/pkg/config"
	"abs/pkg/podcast"
	"abs/pkg/remote"
	"abs/pkg/types"
	"abs/pkg/util"
)

// The ad-removal engine works in the same vocabulary as the rest of the
// program. These are aliases for the shared definitions, not copies of them.

type (
	Config              = types.Config
	ProcOptions         = types.ProcOptions
	LLMProfile          = types.LLMProfile
	AdSegment           = types.AdSegment
	TranscriptionData   = types.TranscriptionData
	CutsData            = types.CutsData
	EpisodeStatusFile   = types.EpisodeStatusFile
	EpisodeAudioMeta    = types.EpisodeAudioMeta
	EpisodeAdCut        = types.EpisodeAdCut
	WhisperEngine       = types.WhisperEngine
	WhisperProfile      = types.WhisperProfile
	ResolvedPodcast     = podcast.ResolvedPodcast
	PodcastConfig       = config.PodcastConfig
	BackendConfig       = types.BackendConfig
	RemoteConfig        = types.RemoteConfig
	RemoteOptions       = types.RemoteOptions
	WhisperConfig       = types.WhisperConfig
	RemoteDoneItem      = remote.RemoteDoneItem
	DefaultSSHTransport = remote.DefaultSSHTransport
	fileLockWrapper     = util.FileLockWrapper
)

const (
	AdRemovalNone              = config.AdRemovalNone
	StateAwaitingTranscription = types.StateAwaitingTranscription
	StateTranscribingLocally   = types.StateTranscribingLocally
	StateTranscribingRemotely  = types.StateTranscribingRemotely
	StateNeedsAdR              = types.StateNeedsAdR
	StateCuttingLocally        = types.StateCuttingLocally
	StateCuttingRemotely       = types.StateCuttingRemotely
	StateQueuedRemote          = types.StateQueuedRemote
	StateReadyForCopyBack      = types.StateReadyForCopyBack
	StateCopiedBack            = types.StateCopiedBack
	StateDone                  = types.StateDone
	StateArchived              = types.StateArchived
	StateFailed                = types.StateFailed
	StateDownloaded            = types.StateDownloaded
	WhisperEngineLocal         = types.WhisperEngineLocal
	WhisperEngineGemini        = types.WhisperEngineGemini
	WhisperEngineDocker        = types.WhisperEngineDocker
)
