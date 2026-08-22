package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mail_cli/cfg_acc"
)

type CacheStore interface {
	GetLabelItems() ([]cfg_acc.LabelItem, error)
	SaveLabelItems(items []cfg_acc.LabelItem) error
	CachedLabelItemsAge() (time.Duration, error)
	IsActive() bool
}

type DiskCacheStore struct {
	DownloadDir string
}

func (d *DiskCacheStore) cachePath() string {
	return filepath.Join(d.DownloadDir, "labels_cache.json")
}

func (d *DiskCacheStore) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	if d.DownloadDir == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(d.cachePath())
	if err != nil {
		return nil, err
	}
	var items []cfg_acc.LabelItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (d *DiskCacheStore) SaveLabelItems(items []cfg_acc.LabelItem) error {
	if d.DownloadDir == "" {
		return nil
	}
	if err := os.MkdirAll(d.DownloadDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	p := d.cachePath()
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func (d *DiskCacheStore) CachedLabelItemsAge() (time.Duration, error) {
	if d.DownloadDir == "" {
		return 0, os.ErrNotExist
	}
	info, err := os.Stat(d.cachePath())
	if err != nil {
		return 0, err
	}
	return time.Since(info.ModTime()), nil
}

type NoOpCacheStore struct{}

func (n *NoOpCacheStore) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	return nil, os.ErrNotExist
}

func (n *NoOpCacheStore) SaveLabelItems(items []cfg_acc.LabelItem) error {
	return nil
}

func (n *NoOpCacheStore) CachedLabelItemsAge() (time.Duration, error) {
	return 0, os.ErrNotExist
}
func (d *DiskCacheStore) IsActive() bool { return true }
func (n *NoOpCacheStore) IsActive() bool { return false }
