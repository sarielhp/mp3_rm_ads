package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func readStatePath(downloadDir string) string {
	return filepath.Join(downloadDir, "seen_cache.json")
}

func loadReadState(downloadDir string) map[string]bool {
	readIDs := make(map[string]bool)
	data, err := os.ReadFile(readStatePath(downloadDir))
	if err != nil {
		return readIDs
	}
	_ = json.Unmarshal(data, &readIDs)
	return readIDs
}

func saveReadState(downloadDir string, readIDs map[string]bool) error {
	data, err := json.Marshal(readIDs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(downloadDir, 0700); err != nil {
		return err
	}
	p := readStatePath(downloadDir)
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func LoadReadState(downloadDir string) map[string]bool {
	return loadReadState(downloadDir)
}

func SaveReadState(downloadDir string, readIDs map[string]bool) error {
	return saveReadState(downloadDir, readIDs)
}

func MarkIDsRead(downloadDir string, ids []string, readIDs map[string]bool) error {
	if readIDs == nil {
		readIDs = loadReadState(downloadDir)
	}
	for _, id := range ids {
		readIDs[id] = true
	}
	return saveReadState(downloadDir, readIDs)
}

func repliedStatePath(downloadDir string) string {
	return filepath.Join(downloadDir, "replied_cache.json")
}

func LoadRepliedState(downloadDir string) map[string]bool {
	repliedIDs := make(map[string]bool)
	data, err := os.ReadFile(repliedStatePath(downloadDir))
	if err != nil {
		return repliedIDs
	}
	_ = json.Unmarshal(data, &repliedIDs)
	return repliedIDs
}

func SaveRepliedState(downloadDir string, repliedIDs map[string]bool) error {
	data, err := json.Marshal(repliedIDs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(downloadDir, 0700); err != nil {
		return err
	}
	p := repliedStatePath(downloadDir)
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func MarkIDsReplied(downloadDir string, ids []string, repliedIDs map[string]bool) error {
	if repliedIDs == nil {
		repliedIDs = LoadRepliedState(downloadDir)
	}
	for _, id := range ids {
		repliedIDs[id] = true
	}
	return SaveRepliedState(downloadDir, repliedIDs)
}
