package remote

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pod/pkg/types"
	"pod/pkg/util"
)

type RemoteDoneItem struct {
	RelPath             string             `json:"rel_path"`
	Status              types.EpisodeState `json:"status"`
	OriginalDurationSec float64            `json:"original_duration_sec,omitempty"`
	CleanedDurationSec  float64            `json:"cleaned_duration_sec,omitempty"`
	CutDurationSec      float64            `json:"cut_duration_sec,omitempty"`
	OriginalSizeBytes   int64              `json:"original_size_bytes,omitempty"`
	CleanedSizeBytes    int64              `json:"cleaned_size_bytes,omitempty"`
	CompletedAt         string             `json:"completed_at,omitempty"`
	WorkerHost          string             `json:"worker_host,omitempty"`
}

type RemoteDoneManifest struct {
	UpdatedAt string                    `json:"updated_at"`
	Episodes  map[string]RemoteDoneItem `json:"episodes"`
}

func GenerateBatchID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("batch-%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

func LoadManifest(path string) (*types.RemoteBatchManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", path, err)
	}
	var manifest types.RemoteBatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest json %s: %w", path, err)
	}
	return &manifest, nil
}

func SaveManifest(path string, m *types.RemoteBatchManifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for manifest %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest json: %w", err)
	}
	data = append(data, '\n')
	if err := util.WriteFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file %s: %w", path, err)
	}
	return nil
}

func UpdateManifestItem(m *types.RemoteBatchManifest, itemID string, status types.RemoteBatchStatus, errStr string) {
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
	RecalculateManifestStats(m)
}

func RecalculateManifestStats(m *types.RemoteBatchManifest) {
	completed := 0
	failed := 0
	for _, item := range m.Items {
		switch item.Status {
		case types.BatchStatusCompleted:
			completed++
		case types.BatchStatusFailed:
			failed++
		}
	}
	m.TotalItems = len(m.Items)
	m.CompletedItems = completed
	m.FailedItems = failed
	if completed+failed == len(m.Items) && len(m.Items) > 0 {
		if completed > 0 {
			m.Status = types.BatchStatusCompleted
		} else {
			m.Status = types.BatchStatusFailed
		}
	}
}

func LoadDoneManifest(path string) (*RemoteDoneManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RemoteDoneManifest{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				Episodes:  make(map[string]RemoteDoneItem),
			}, nil
		}
		return nil, fmt.Errorf("failed to read done manifest %s: %w", path, err)
	}
	var m RemoteDoneManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse done manifest %s: %w", path, err)
	}
	if m.Episodes == nil {
		m.Episodes = make(map[string]RemoteDoneItem)
	}
	return &m, nil
}

func SaveDoneManifest(path string, m *RemoteDoneManifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for done manifest %s: %w", dir, err)
	}
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if m.Episodes == nil {
		m.Episodes = make(map[string]RemoteDoneItem)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal done manifest: %w", err)
	}
	return util.WriteFileAtomic(path, append(data, '\n'), 0644)
}

func withDoneManifestLock(manifestPath string, fn func() error) error {
	fl, err := util.AcquireFileLock(manifestPath)
	if err != nil {
		return err
	}
	if fl == nil {
		return fmt.Errorf("could not acquire lock for %s", manifestPath)
	}
	defer fl.Release()
	return fn()
}

func AddDoneEpisode(manifestPath string, item RemoteDoneItem) error {
	return withDoneManifestLock(manifestPath, func() error {
		m, err := LoadDoneManifest(manifestPath)
		if err != nil {
			return err
		}
		if item.CompletedAt == "" {
			item.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		}
		m.Episodes[item.RelPath] = item
		return SaveDoneManifest(manifestPath, m)
	})
}

func RemoveDoneEpisode(manifestPath, relPath string) error {
	return withDoneManifestLock(manifestPath, func() error {
		m, err := LoadDoneManifest(manifestPath)
		if err != nil {
			return err
		}
		delete(m.Episodes, relPath)
		return SaveDoneManifest(manifestPath, m)
	})
}
