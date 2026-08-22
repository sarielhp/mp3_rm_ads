package outlook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mail_cli/cache"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
)

type graphAccountMessage struct {
	ID               string    `json:"id"`
	ParentFolderID   string    `json:"parentFolderId"`
	IsRead           bool      `json:"isRead"`
	ReceivedDateTime time.Time `json:"receivedDateTime"`
}

// FetchLatestAccountEmails queries Microsoft Graph account-wide for the latest N emails.
func (c *OutlookClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	// 1. Get folder ID -> full path mapping
	items, _, err := c.fetchFoldersRecursive("", "")
	if err != nil {
		return nil, err
	}
	folderMap := make(map[string]string)
	for _, it := range items {
		// fetchFoldersRecursive returned items; to get IDs we can match by name or fetch directly
		folderMap[it.Name] = it.FullName
	}

	// 2. Query messages across all folders
	urlStr := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages?$top=%d&$orderby=receivedDateTime desc&$select=id,parentFolderId,isRead,receivedDateTime", limit)
	resp, err := c.client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching account messages failed: status %d", resp.StatusCode)
	}

	var res struct {
		Value []graphAccountMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	cacheDir := c.config.DownloadDir
	_ = os.MkdirAll(filepath.Join(cacheDir, "messages"), 0700)

	readIDs := cache.LoadReadState(cacheDir)
	for _, m := range res.Value {
		readIDs[m.ID] = m.IsRead
	}
	_ = cache.SaveReadState(cacheDir, readIDs)

	// 3. Download missing bodies in parallel
	numWorkers := 5
	if numWorkers > len(res.Value) {
		numWorkers = len(res.Value)
	}

	taskChan := make(chan graphAccountMessage, len(res.Value))
	for _, m := range res.Value {
		taskChan <- m
	}
	close(taskChan)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range taskChan {
				if exists, err := msg.Exists(cacheDir, m.ID); err == nil && exists {
					continue
				}
				emailBytes, err := c.downloadRawMessage(m.ID)
				if err != nil {
					slog.Error("Failed to download raw message", slog.String("msgID", m.ID), slog.Any("error", err))
					continue
				}
				now := m.ReceivedDateTime
				if now.IsZero() {
					now = time.Now()
				}
				_ = msg.Store(cacheDir, m.ID, emailBytes, now)
			}
		}()
	}
	wg.Wait()

	// 4. Build results
	var refs []cfg_acc.MessageFolderRef
	for _, m := range res.Value {
		folderName := "Inbox"
		if name, ok := folderMap[m.ParentFolderID]; ok && name != "" {
			folderName = name
		}
		refs = append(refs, cfg_acc.MessageFolderRef{
			MessageID: m.ID,
			Folder:    folderName,
		})
	}

	return refs, nil
}
