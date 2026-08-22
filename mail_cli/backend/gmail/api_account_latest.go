package gmail

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"

	gmailapi "google.golang.org/api/gmail/v1"
)

// fetchLatestAccountEmailsREST fetches the latest N emails across the entire Gmail account in a single query.
func fetchLatestAccountEmailsREST(config *Config, limit int) ([]cfg_acc.MessageFolderRef, error) {
	srv, err := GetGmailService(config)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	// 1. Get label mapping (id -> name)
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}
	idToName := make(map[string]string)
	for _, l := range labelsRes.Labels {
		idToName[l.Id] = l.Name
	}

	// 2. Query messages across all folders (omitting labelIds queries entire mailbox)
	var allMsgs []*gmailapi.Message
	pageToken := ""
	for {
		call := srv.Users.Messages.List("me").IncludeSpamTrash(true).MaxResults(int64(limit))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list account messages: %w", err)
		}
		allMsgs = append(allMsgs, res.Messages...)
		if len(allMsgs) >= limit || res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	if len(allMsgs) == 0 {
		return nil, nil
	}
	if len(allMsgs) > limit {
		allMsgs = allMsgs[:limit]
	}

	downloadDir := config.DownloadDir
	_ = os.MkdirAll(filepath.Join(downloadDir, "messages"), 0700)

	// 3. For each message, determine missing IDs to download and resolve labels
	var missingIDs []string
	for _, m := range allMsgs {
		exists, err := msg.Exists(downloadDir, m.Id)
		if err != nil || !exists {
			missingIDs = append(missingIDs, m.Id)
		}
	}

	// 4. Download any missing message bodies into cache in parallel
	if len(missingIDs) > 0 {
		numWorkers := 8
		if numWorkers > len(missingIDs) {
			numWorkers = len(missingIDs)
		}
		taskChan := make(chan string, len(missingIDs))
		for _, id := range missingIDs {
			taskChan <- id
		}
		close(taskChan)

		errChan := make(chan error, len(missingIDs))
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for msgID := range taskChan {
					resMsg, err := srv.Users.Messages.Get("me", msgID).Format("raw").Do()
					if err != nil {
						errChan <- fmt.Errorf("failed to download message %s: %w", msgID, err)
						continue
					}
					rawBytes, err := DecodeGmailRaw(resMsg.Raw)
					if err != nil {
						errChan <- fmt.Errorf("failed to decode raw message %s: %w", msgID, err)
						continue
					}
					if err := msg.Store(downloadDir, msgID, rawBytes, time.Now()); err != nil {
						slog.Error("Failed to store message", slog.String("msgID", msgID), slog.Any("error", err))
					}
				}
			}()
		}
		wg.Wait()
		close(errChan)
		for err := range errChan {
			if err != nil {
				slog.Warn("Failed to download email", slog.String("error", err.Error()))
			}
		}
	}

	// 5. Build MessageFolderRef results
	// Non-folder Gmail pseudo-labels to skip
	ignoredGmailLabels := map[string]bool{
		"UNREAD":    true,
		"IMPORTANT": true,
		"CHAT":      true,
		"STARRED":   true,
	}

	var refs []cfg_acc.MessageFolderRef
	for _, m := range allMsgs {
		folder := "INBOX"
		meta, err := srv.Users.Messages.Get("me", m.Id).Format("minimal").Do()
		if err == nil && len(meta.LabelIds) > 0 {
			var candidateFolders []string
			for _, lid := range meta.LabelIds {
				name := idToName[lid]
				if name == "" {
					name = lid
				}
				if ignoredGmailLabels[name] || strings.HasPrefix(name, "CATEGORY_") {
					continue
				}
				candidateFolders = append(candidateFolders, name)
			}

			// Prioritize user-created labels over generic system labels like INBOX/SENT/TRASH
			selected := ""
			for _, c := range candidateFolders {
				if !gmailSystemLabels[c] {
					selected = c
					break
				}
			}
			if selected == "" && len(candidateFolders) > 0 {
				selected = candidateFolders[0]
			}
			if selected != "" {
				folder = selected
			}
		}
		refs = append(refs, cfg_acc.MessageFolderRef{
			MessageID: m.Id,
			Folder:    folder,
		})
	}

	return refs, nil
}
