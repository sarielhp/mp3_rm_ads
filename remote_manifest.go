package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RemoteDoneItem struct {
	RelPath             string       `json:"rel_path"`
	Status              EpisodeState `json:"status"`
	OriginalDurationSec float64      `json:"original_duration_sec,omitempty"`
	CleanedDurationSec  float64      `json:"cleaned_duration_sec,omitempty"`
	CutDurationSec      float64      `json:"cut_duration_sec,omitempty"`
	OriginalSizeBytes   int64        `json:"original_size_bytes,omitempty"`
	CleanedSizeBytes    int64        `json:"cleaned_size_bytes,omitempty"`
	CompletedAt         string       `json:"completed_at,omitempty"`
	WorkerHost          string       `json:"worker_host,omitempty"`
}

type RemoteDoneManifest struct {
	UpdatedAt string                    `json:"updated_at"`
	Episodes  map[string]RemoteDoneItem `json:"episodes"`
}

func loadDoneManifest(path string) (*RemoteDoneManifest, error) {
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

func saveDoneManifest(path string, m *RemoteDoneManifest) error {
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
	data = append(data, '\n')
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp done manifest %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically rename done manifest %s: %w", path, err)
	}
	return nil
}

func addDoneEpisode(manifestPath string, item RemoteDoneItem) error {
	m, err := loadDoneManifest(manifestPath)
	if err != nil {
		return err
	}
	if item.CompletedAt == "" {
		item.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	m.Episodes[item.RelPath] = item
	return saveDoneManifest(manifestPath, m)
}

func removeDoneEpisode(manifestPath, relPath string) error {
	m, err := loadDoneManifest(manifestPath)
	if err != nil {
		return err
	}
	delete(m.Episodes, relPath)
	return saveDoneManifest(manifestPath, m)
}

func archiveDoneEpisode(donePath, archivePath, relPath string) error {
	doneM, err := loadDoneManifest(donePath)
	if err != nil {
		return err
	}
	item, ok := doneM.Episodes[relPath]
	if !ok {
		item = RemoteDoneItem{
			RelPath:     relPath,
			Status:      StateArchived,
			CompletedAt: time.Now().UTC().Format(time.RFC3339),
		}
	} else {
		item.Status = StateArchived
		delete(doneM.Episodes, relPath)
		if err := saveDoneManifest(donePath, doneM); err != nil {
			return err
		}
	}

	archM, err := loadDoneManifest(archivePath)
	if err != nil {
		archM = &RemoteDoneManifest{
			Episodes: make(map[string]RemoteDoneItem),
		}
	}
	archM.Episodes[relPath] = item
	return saveDoneManifest(archivePath, archM)
}
