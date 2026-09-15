package cli

import (
	"fmt"
	"os"

	"pod/pkg/backend"
	"pod/pkg/config"
	"pod/pkg/podcast"
	"pod/pkg/progress"
	"pod/pkg/remote"
	"pod/pkg/types"
	"pod/pkg/util"
)

type (
	Config             = types.Config
	CLIOptions         = types.CLIOptions
	ProcOptions        = types.ProcOptions
	PolicyOptions      = types.PolicyOptions
	LLMProfile         = types.LLMProfile
	AdSegment          = types.AdSegment
	TranscriptionData  = types.TranscriptionData
	CutsData           = types.CutsData
	CutEntry           = types.CutEntry
	EpisodeStatusFile  = types.EpisodeStatusFile
	PlayerTrack        = types.PlayerTrack
	Podcast            = backend.Podcast
	PodcastConfig      = config.PodcastConfig
	FeedEpisode        = backend.FeedEpisode
	Episode            = backend.Episode
	WhisperEngine      = types.WhisperEngine
	WhisperProfile     = types.WhisperProfile
	RemoteTransport    = remote.RemoteTransport
	RemoteBatchJobItem = types.RemoteBatchJobItem
	ResolvedPodcast    = podcast.ResolvedPodcast
	ResolvedEpisode    = podcast.ResolvedEpisode
	ResolvedID         = podcast.ResolvedID
	syncWG             = util.SyncWG
)

const (
	AdRemovalNone   = config.AdRemovalNone
	AdRemovalLatest = config.AdRemovalLatest
	AdRemovalAll    = config.AdRemovalAll

	DownloadPolicyNone    = config.DownloadPolicyNone
	DownloadPolicyLatest  = config.DownloadPolicyLatest
	DownloadPolicyLatestK = config.DownloadPolicyLatestK
	DownloadPolicyAll     = config.DownloadPolicyAll

	StateTranscribingLocally  = types.StateTranscribingLocally
	StateTranscribingRemotely = types.StateTranscribingRemotely
	StateCuttingLocally       = types.StateCuttingLocally
	StateCuttingRemotely      = types.StateCuttingRemotely
	StateQueuedRemote         = types.StateQueuedRemote
	StateCopiedBack           = types.StateCopiedBack
	StateDone                 = types.StateDone
	StateDownloaded           = types.StateDownloaded

	BatchStatusProcessing = types.BatchStatusProcessing
	BatchStatusCompleted  = types.BatchStatusCompleted
	BatchStatusFailed     = types.BatchStatusFailed

	WhisperEngineLocal  = types.WhisperEngineLocal
	WhisperEngineDocker = types.WhisperEngineDocker
)

func fatalError(formatStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatStr, args...)
	os.Exit(1)
}

func loadConfig() Config {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil {
		c := config.DefaultConfig()
		return c
	}
	return *cfg
}

// reporter turns the CLI's quiet/verbose flags into the progress.Reporter that
// library calls take. Quiet discards; otherwise info goes to stdout, warnings
// to stderr, and detail only when --verbose.
func reporter(cli CLIOptions) progress.Reporter {
	if cli.Quiet {
		return progress.Discard
	}
	return progress.Writer(os.Stdout, os.Stderr, cli.Verbose)
}
