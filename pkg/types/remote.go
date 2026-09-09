package types

type RemoteBatchStatus string

const (
	BatchStatusQueued     RemoteBatchStatus = "queued"
	BatchStatusProcessing RemoteBatchStatus = "processing"
	BatchStatusCompleted  RemoteBatchStatus = "completed"
	BatchStatusFailed     RemoteBatchStatus = "failed"
	BatchStatusCancelled  RemoteBatchStatus = "cancelled"
)

type RemoteBatchJobItem struct {
	ID                  string            `json:"id"`
	SourceFile          string            `json:"source_file"`
	RelativePath        string            `json:"relative_path,omitempty"`
	AudioFileName       string            `json:"audio_file_name"`
	Status              RemoteBatchStatus `json:"status"`
	Error               string            `json:"error,omitempty"`
	OriginalDurationSec float64           `json:"original_duration_sec,omitempty"`
	CleanedDurationSec  float64           `json:"cleaned_duration_sec,omitempty"`
	CutDurationSec      float64           `json:"cut_duration_sec,omitempty"`
	CleanedAudioFile    string            `json:"cleaned_audio_file,omitempty"`
	CutsJSONFile        string            `json:"cuts_json_file,omitempty"`
	TranscriptJSONFile  string            `json:"transcript_json_file,omitempty"`
	Priority            int               `json:"priority,omitempty"`
}

type RemoteBatchManifest struct {
	BatchID        string               `json:"batch_id"`
	CreatedAt      string               `json:"created_at"`
	UpdatedAt      string               `json:"updated_at,omitempty"`
	Host           string               `json:"host,omitempty"`
	Status         RemoteBatchStatus    `json:"status"`
	TotalItems     int                  `json:"total_items"`
	CompletedItems int                  `json:"completed_items"`
	FailedItems    int                  `json:"failed_items"`
	Items          []RemoteBatchJobItem `json:"items"`
}

type RemoteServerStatus struct {
	Host           string                `json:"host"`
	Reachable      bool                  `json:"reachable"`
	BinaryVersion  string                `json:"binary_version,omitempty"`
	WhisperStatus  string                `json:"whisper_status,omitempty"`
	ActiveBatches  []RemoteBatchManifest `json:"active_batches,omitempty"`
	WorkerRunning  bool                  `json:"worker_running"`
	ActiveTask     string                `json:"active_task,omitempty"`
	ActiveStage    string                `json:"active_stage,omitempty"`
	ActiveElapsed  string                `json:"active_elapsed,omitempty"`
	ActiveETA      string                `json:"active_eta,omitempty"`
	ActiveDuration string                `json:"active_duration,omitempty"`
	QueuedTasks    []string              `json:"queued_tasks,omitempty"`
	Message        string                `json:"message,omitempty"`
}
