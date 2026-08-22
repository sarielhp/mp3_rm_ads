package app

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

// GetTempDir returns a directory path of the form:
// /tmp/<user>/mail_cli/<year>/<month>/<day>/<running counter>/
// and ensures it exists.
func GetTempDir() (string, error) {
	currUser, err := user.Current()
	var username string
	if err != nil {
		// Fallback to environment variables
		username = os.Getenv("USER")
		if username == "" {
			username = "default"
		}
	} else {
		username = currUser.Username
	}

	now := time.Now()
	baseDir := filepath.Join("/tmp", username, "mail_cli",
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
	)

	// Since we need a running counter, we scan baseDir for existing numbered subdirectories.
	counter := 1
	if files, err := os.ReadDir(baseDir); err == nil {
		maxNum := 0
		for _, f := range files {
			if f.IsDir() {
				var num int
				if _, err := fmt.Sscanf(f.Name(), "%d", &num); err == nil {
					if num > maxNum {
						maxNum = num
					}
				}
			}
		}
		counter = maxNum + 1
	}

	targetDir := filepath.Join(baseDir, fmt.Sprintf("%d", counter))
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return "", err
	}

	return targetDir, nil
}
