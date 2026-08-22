package outlook

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
)

type graphMessage struct {
	ID               string    `json:"id"`
	IsRead           bool      `json:"isRead"`
	ReceivedDateTime time.Time `json:"receivedDateTime"`
}

func (c *OutlookClient) downloadRawMessage(messageID string) ([]byte, error) {
	resp, err := c.client.Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/$value", messageID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to download raw message %s, status %d: %s", messageID, resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (c *OutlookClient) FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	folderID, err := c.getFolderID(folderName)
	if err != nil {
		return nil, err
	}

	limit := c.config.Limit
	if limit <= 0 {
		limit = 100
	}

	slog.Info("FetchAndDownloadEmails: querying mailbox", slog.String("folder", folderName), slog.String("folderID", folderID))
	urlStr := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/mailFolders/%s/messages?$top=%d&$select=id,isRead,receivedDateTime", folderID, limit)

	resp, err := c.client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetching messages failed: status %d: %s", resp.StatusCode, string(b))
	}

	var res struct {
		Value []graphMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var ids []string
	cacheDir := c.config.DownloadDir
	readIDs := cache.LoadReadState(cacheDir)

	for _, msgItem := range res.Value {
		ids = append(ids, msgItem.ID)
		readIDs[msgItem.ID] = msgItem.IsRead
	}

	if err := cache.SaveReadState(cacheDir, readIDs); err != nil {
		slog.Error("Failed to save read state after Outlook sync", slog.Any("error", err))
	}

	_ = os.MkdirAll(filepath.Join(cacheDir, "messages"), 0700)

	if len(ids) == 0 {
		return nil, nil
	}

	slog.Info("Refreshing Outlook email cache", slog.Int("count", len(ids)), slog.String("folder", folderName))
	numWorkers := 5
	if numWorkers > len(res.Value) {
		numWorkers = len(res.Value)
	}

	taskChan := make(chan graphMessage, len(res.Value))
	for _, m := range res.Value {
		taskChan <- m
	}
	close(taskChan)

	var wg sync.WaitGroup
	var mu sync.Mutex

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
				dateStr := now.Format("2006/01/02")
				dir := filepath.Join(cacheDir, "messages", dateStr)
				_ = os.MkdirAll(dir, 0700)
				emlPath := filepath.Join(dir, m.ID+".eml")
				_ = os.WriteFile(emlPath, emailBytes, 0600)

				mu.Lock()
				if err := msg.Store(cacheDir, m.ID, emailBytes, now); err != nil {
					slog.Error("Failed to store message in msg cache", slog.String("msgID", m.ID), slog.Any("error", err))
				}
				mu.Unlock()
				slog.Info("Cached Outlook email locally", slog.String("message_id", m.ID), slog.String("folder", folderName))
			}
		}()
	}
	wg.Wait()

	if err := label.ReplaceAll(cacheDir, folderName, ids); err != nil {
		slog.Error("FetchAndDownloadEmails: ReplaceAll failed", slog.String("folder", folderName), slog.Any("error", err))
	}

	return ids, nil
}

func (c *OutlookClient) moveSingleEmail(messageID, destFolderID string) error {
	bodyBytes, _ := json.Marshal(map[string]string{
		"destinationId": destFolderID,
	})
	resp, err := c.client.Post(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/move", messageID), "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("move failed: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *OutlookClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if err := c.init(); err != nil {
		return err
	}
	destFolderID, err := c.getFolderID(destLabelName)
	if err != nil {
		if err := c.EnsureLabelExists(destLabelName); err != nil {
			return err
		}
		destFolderID, err = c.getFolderID(destLabelName)
		if err != nil {
			return err
		}
	}
	for _, id := range messageIDs {
		if err := c.moveSingleEmail(id, destFolderID); err != nil {
			return err
		}
	}
	return nil
}

func (c *OutlookClient) copySingleEmail(messageID, destFolderID string) error {
	bodyBytes, _ := json.Marshal(map[string]string{
		"destinationId": destFolderID,
	})
	resp, err := c.client.Post(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/copy", messageID), "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("copy failed: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *OutlookClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if err := c.init(); err != nil {
		return err
	}
	destFolderID, err := c.getFolderID(destLabelName)
	if err != nil {
		if err := c.EnsureLabelExists(destLabelName); err != nil {
			return err
		}
		destFolderID, err = c.getFolderID(destLabelName)
		if err != nil {
			return err
		}
	}
	for _, id := range messageIDs {
		if err := c.copySingleEmail(id, destFolderID); err != nil {
			return err
		}
	}
	return nil
}

func (c *OutlookClient) MoveToInbox(messageIDs []string, sourceLabelName string) error {
	return c.MoveEmail(messageIDs, sourceLabelName, "Inbox")
}

func (c *OutlookClient) MoveAllSpam(destLabel string) error {
	if err := c.init(); err != nil {
		return err
	}
	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Junk Email"
	}
	if strings.EqualFold(spamFolder, destLabel) {
		return c.DeleteAllSpam()
	}

	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Printf("%s Outlook Junk folder is empty.\n", app.PrefixInfo)
		return nil
	}
	return c.MoveEmail(ids, spamFolder, destLabel)
}

func (c *OutlookClient) DeleteAllSpam() error {
	if err := c.init(); err != nil {
		return err
	}
	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Junk Email"
	}
	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Printf("%s Outlook Junk folder is already empty.\n", app.PrefixInfo)
		return nil
	}

	fmt.Printf("%s Permanently destroying %d message(s) in Outlook Junk folder...\n", app.PrefixInfo, len(ids))
	destroyedCount := 0
	for _, id := range ids {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s", id), nil)
		if err != nil {
			continue
		}
		resp, err := c.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
				destroyedCount++
			}
		}
	}
	fmt.Printf("%s Successfully purged %d spam emails permanently from Outlook server.\n", app.PrefixSuccess, destroyedCount)
	return nil
}

func (c *OutlookClient) MarkAsRead(messageIDs []string) error {
	if err := c.init(); err != nil {
		return err
	}
	for _, id := range messageIDs {
		bodyBytes, _ := json.Marshal(map[string]bool{
			"isRead": true,
		})
		req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s", id), bytes.NewBuffer(bodyBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	return nil
}

func (c *OutlookClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	if err := c.init(); err != nil {
		return err
	}

	targetFolderID, err := c.getFolderID(targetLabel)
	if err != nil {
		_ = c.EnsureLabelExists(targetLabel)
		targetFolderID, err = c.getFolderID(targetLabel)
		if err != nil {
			return err
		}
	}

	b64Content := base64.StdEncoding.EncodeToString(rawBytes)
	req, err := http.NewRequest(http.MethodPost, "https://graph.microsoft.com/v1.0/me/messages", strings.NewReader(b64Content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload raw email returned status %d: %s", resp.StatusCode, string(b))
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	return c.moveSingleEmail(res.ID, targetFolderID)
}

func (c *OutlookClient) SendEmail(rawBytes []byte) error {
	if err := c.init(); err != nil {
		return err
	}

	b64Content := base64.StdEncoding.EncodeToString(rawBytes)
	req, err := http.NewRequest(http.MethodPost, "https://graph.microsoft.com/v1.0/me/messages", strings.NewReader(b64Content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create draft for sending returned status %d: %s", resp.StatusCode, string(b))
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	sendURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/send", res.ID)
	reqSend, err := http.NewRequest(http.MethodPost, sendURL, nil)
	if err != nil {
		return err
	}

	respSend, err := c.client.Do(reqSend)
	if err != nil {
		return err
	}
	defer respSend.Body.Close()

	if respSend.StatusCode != http.StatusAccepted && respSend.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(respSend.Body)
		return fmt.Errorf("send draft returned status %d: %s", respSend.StatusCode, string(b))
	}

	return nil
}
