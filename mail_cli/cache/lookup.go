package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/cache/msg"
	"mail_cli/email"
)

func FindCachedEmailByID(downloadDir string, targetID string) (string, string, error) {
	targetUpper := strings.ToUpper(strings.TrimSpace(targetID))
	var foundID string
	msg.ForEachID(downloadDir, func(id string) bool {
		if strings.ToUpper(id) == targetUpper {
			foundID = id
			return true
		}
		return false
	})

	if foundID == "" {
		msg.ForEachID(downloadDir, func(id string) bool {
			if email.ComputeShortID(id) == targetUpper {
				foundID = id
				return true
			}
			return false
		})
	}

	if foundID == "" {
		return "", "", fmt.Errorf("no cached email found with ID %q", targetID)
	}

	idxDir := filepath.Join(downloadDir, "indexes")
	entries, err := os.ReadDir(idxDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, rErr := os.ReadFile(filepath.Join(idxDir, e.Name()))
			if rErr != nil {
				continue
			}
			var ids []string
			if err := json.Unmarshal(data, &ids); err != nil {
				continue
			}
			for _, id := range ids {
				if id == foundID {
					return foundID, strings.TrimSuffix(e.Name(), ".json"), nil
				}
			}
		}
	}

	return foundID, "", nil
}
