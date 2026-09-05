package backend

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func GetTokenFromDB(dbPath string) string {
	verifyAudiobookshelfNotDisabled("GetTokenFromDB")
	if dbPath == "" {
		return ""
	}
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return ""
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return ""
	}
	defer db.Close()

	var token string
	query := "SELECT token FROM users WHERE token IS NOT NULL AND token != '' ORDER BY createdAt ASC LIMIT 1;"
	err = db.QueryRow(query).Scan(&token)
	if err != nil {
		return ""
	}
	return token
}

func ResetPodcastDateCheckInDB(dbPath, itemID, title string) error {
	verifyAudiobookshelfNotDisabled("ResetPodcastDateCheckInDB")
	if dbPath == "" {
		return fmt.Errorf("database path does not exist: %s", dbPath)
	}
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return fmt.Errorf("database path does not exist: %s", dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}
	defer db.Close()

	if itemID != "" {
		_, err = db.Exec("UPDATE podcasts SET lastEpisodeCheck = '1970-01-01 00:00:00', maxNewEpisodesToDownload = 0 WHERE id = ?", itemID)
		if err == nil {
			return nil
		}
	}

	if title != "" {
		_, err = db.Exec("UPDATE podcasts SET lastEpisodeCheck = '1970-01-01 00:00:00', maxNewEpisodesToDownload = 0 WHERE title = ?", title)
		if err != nil {
			return fmt.Errorf("failed to reset podcast check date in db: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no item ID or title provided for podcast date reset")
}

func (c *AudiobookshelfBackend) ResetPodcastDateCheckAPI(itemID string) error {
	if itemID == "" || c.Host == "" {
		return nil
	}
	payload := map[string]interface{}{
		"maxNewEpisodesToDownload": 0,
	}
	_, err := c.Request(fmt.Sprintf("/api/items/%s/media", itemID), "PATCH", payload)
	if err != nil {
		_, err = c.Request(fmt.Sprintf("/api/items/%s", itemID), "PATCH", map[string]interface{}{
			"media": payload,
		})
	}
	return err
}

func (c *AudiobookshelfBackend) ResetPodcastDateCheck(itemID, title string) error {
	var errs []string
	if c.DBPath != "" {
		if err := ResetPodcastDateCheckInDB(c.DBPath, itemID, title); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.Host != "" && itemID != "" {
		if err := c.ResetPodcastDateCheckAPI(itemID); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 && c.DBPath == "" && c.Host == "" {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
