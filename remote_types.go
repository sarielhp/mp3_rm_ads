package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	Host          string                `json:"host"`
	Reachable     bool                  `json:"reachable"`
	BinaryVersion string                `json:"binary_version,omitempty"`
	WhisperStatus string                `json:"whisper_status,omitempty"`
	ActiveBatches []RemoteBatchManifest `json:"active_batches,omitempty"`
	WorkerRunning bool                  `json:"worker_running"`
	Message       string                `json:"message,omitempty"`
}

func generateBatchID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("batch-%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

func loadManifest(path string) (*RemoteBatchManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", path, err)
	}
	var manifest RemoteBatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest json %s: %w", path, err)
	}
	return &manifest, nil
}

func saveManifest(path string, m *RemoteBatchManifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for manifest %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file %s: %w", path, err)
	}
	return nil
}

func updateManifestItem(m *RemoteBatchManifest, itemID string, status RemoteBatchStatus, errStr string) {
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	for i := range m.Items {
		if m.Items[i].ID == itemID {
			m.Items[i].Status = status
			if errStr != "" {
				m.Items[i].Error = errStr
			}
			break
		}
	}
	recalculateManifestStats(m)
}

func recalculateManifestStats(m *RemoteBatchManifest) {
	completed := 0
	failed := 0
	for _, item := range m.Items {
		switch item.Status {
		case BatchStatusCompleted:
			completed++
		case BatchStatusFailed:
			failed++
		}
	}
	m.TotalItems = len(m.Items)
	m.CompletedItems = completed
	m.FailedItems = failed
	if completed+failed == len(m.Items) && len(m.Items) > 0 {
		if completed > 0 {
			m.Status = BatchStatusCompleted
		} else {
			m.Status = BatchStatusFailed
		}
	}
}
