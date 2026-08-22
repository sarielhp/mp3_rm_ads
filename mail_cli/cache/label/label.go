package label

import (
	"encoding/json"
	"os"
	"path/filepath"

	"mail_cli/cfg_g"
)

func idsPath(downloadDir, folder string) string {
	return filepath.Join(downloadDir, "indexes", cfg_g.SanitizeLabelForCache(folder)+".json")
}

func IDs(downloadDir, folder string) ([]string, error) {
	data, err := os.ReadFile(idsPath(downloadDir, folder))
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func ReplaceAll(downloadDir, folder string, msgIDs []string) error {
	dir := filepath.Join(downloadDir, "indexes")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(msgIDs)
	if err != nil {
		return err
	}
	p := idsPath(downloadDir, folder)
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func Add(downloadDir, folder, msgID string) error {
	ids, err := IDs(downloadDir, folder)
	if err != nil {
		if os.IsNotExist(err) {
			return ReplaceAll(downloadDir, folder, []string{msgID})
		}
		return err
	}
	for _, id := range ids {
		if id == msgID {
			return nil
		}
	}
	ids = append(ids, msgID)
	return ReplaceAll(downloadDir, folder, ids)
}

func Remove(downloadDir, folder, msgID string) error {
	ids, err := IDs(downloadDir, folder)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	found := false
	for i, id := range ids {
		if id == msgID {
			ids = append(ids[:i], ids[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	return ReplaceAll(downloadDir, folder, ids)
}

func Move(downloadDir, msgID, fromFolder, toFolder string) error {
	if fromFolder == toFolder {
		return nil
	}
	if err := Remove(downloadDir, fromFolder, msgID); err != nil {
		return err
	}
	return Add(downloadDir, toFolder, msgID)
}

func HasStructure(downloadDir string) bool {
	_, err := os.Stat(filepath.Join(downloadDir, "indexes"))
	return err == nil
}
