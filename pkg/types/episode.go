package types

type EpisodeState string

const (
	StateDownloaded            EpisodeState = "downloaded"
	StateQueuedRemote          EpisodeState = "queued_remote"
	StateAwaitingTranscription EpisodeState = "awaiting_transcription"
	StateTranscribingLocally   EpisodeState = "transcribing_locally"
	StateTranscribingRemotely  EpisodeState = "transcribing_remotely"
	StateCuttingLocally        EpisodeState = "cutting_locally"
	StateCuttingRemotely       EpisodeState = "cutting_remotely"
	StateReadyForCopyBack      EpisodeState = "ready_for_copy_back"
	StateCopiedBack            EpisodeState = "copied_back"
	StateDone                  EpisodeState = "done"
	StateArchived              EpisodeState = "archived"
	StateFailed                EpisodeState = "failed"
	StateNeedsAdR              EpisodeState = "needs_adr"
)

type EpisodeAudioMeta struct {
	Filename      string  `json:"filename,omitempty"`
	DurationSec   float64 `json:"duration_sec,omitempty"`
	SizeBytes     int64   `json:"size_bytes,omitempty"`
	AdDurationSec float64 `json:"ad_duration_sec,omitempty"`
}

type EpisodeAdCut struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Reason string  `json:"reason,omitempty"`
}

type EpisodeStatusFile struct {
	ID                    string           `json:"id,omitempty"`
	Version               int              `json:"version"`
	MediaFile             string           `json:"media_file"`
	Status                EpisodeState     `json:"status"`
	CurrentStep           string           `json:"current_step,omitempty"`
	StepStartedAt         string           `json:"step_started_at,omitempty"`
	CreatedAt             string           `json:"created_at"`
	UpdatedAt             string           `json:"updated_at"`
	PublishedAt           string           `json:"published_at,omitempty"`
	PublicationSource     string           `json:"publication_source,omitempty"`
	WorkerHost            string           `json:"worker_host,omitempty"`
	Original              EpisodeAudioMeta `json:"original,omitempty"`
	Cleaned               EpisodeAudioMeta `json:"cleaned,omitempty"`
	Ads                   []EpisodeAdCut   `json:"ads,omitempty"`
	LastError             string           `json:"last_error,omitempty"`
	Priority              int              `json:"priority,omitempty"`
	Favorite              bool             `json:"favorite,omitempty"`
	AdDetectionSuccessful *bool            `json:"ad_detection_successful,omitempty"`
	AdDetectionStatus     string           `json:"ad_detection_status,omitempty"`
	AdDetectionError      string           `json:"ad_detection_error,omitempty"`
	AdDetectionModel      string           `json:"ad_detection_model,omitempty"`
}

func (s *EpisodeStatusFile) SetFavorite(fav bool) {
	s.Favorite = fav
}

func (s *EpisodeStatusFile) IsFavorite() bool {
	return s.Favorite
}
