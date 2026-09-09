package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sariel/abs/pkg/types"
)

type RemoteBatchStatus = types.RemoteBatchStatus

const (
	BatchStatusQueued     = types.BatchStatusQueued
	BatchStatusProcessing = types.BatchStatusProcessing
	BatchStatusCompleted  = types.BatchStatusCompleted
	BatchStatusFailed     = types.BatchStatusFailed
	BatchStatusCancelled  = types.BatchStatusCancelled
)

type RemoteBatchJobItem = types.RemoteBatchJobItem
type RemoteBatchManifest = types.RemoteBatchManifest
type RemoteServerStatus = types.RemoteServerStatus

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
	if err := writeFileAtomic(path, data, 0644); err != nil {
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
