package gmail

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
)

func fetchAndDownloadEmailsREST(config *Config, folderName string, cacheSubdir string) ([]string, error) {
	srv, err := GetGmailService(config)
	if err != nil {
		return nil, err
	}

	labelID, errLabel := resolveLabelIDByName(srv, config, folderName)
	if errLabel != nil {
		labelID = ""
	}

	if config.Verbose {
		fmt.Printf("%s Searching for messages in folder %q (labelID: %s)...\n", app.PrefixInfo, folderName, labelID)
	}

	var allMsgs []*gmailapi.Message
	pageToken := ""
	limit := config.Limit

	for {
		call := srv.Users.Messages.List("me").IncludeSpamTrash(true)
		if labelID != "" {
			call = call.LabelIds(labelID)
		} else {
			call = call.Q(fmt.Sprintf("label:%q", folderName))
		}
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages: %w", err)
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

	var allIDs []string
	var missingIDs []string

	downloadDir := config.DownloadDir
	if err := os.MkdirAll(filepath.Join(downloadDir, "messages"), 0700); err != nil {
		return nil, fmt.Errorf("failed to create messages cache directory: %w", err)
	}

	for _, m := range allMsgs {
		allIDs = append(allIDs, m.Id)
		exists, err := msg.Exists(downloadDir, m.Id)
		if err != nil || !exists {
			missingIDs = append(missingIDs, m.Id)
		}
	}

	readIDs := cache.LoadReadState(downloadDir)
	fetchUnreadIDs := func() ([]string, error) {
		var unreadIDs []string
		unreadPageToken := ""
		for {
			call := srv.Users.Messages.List("me").Q("is:unread").IncludeSpamTrash(true)
			if labelID != "" {
				call = call.LabelIds(labelID)
			} else {
				call = call.Q(fmt.Sprintf("label:%q is:unread", folderName))
			}
			if unreadPageToken != "" {
				call = call.PageToken(unreadPageToken)
			}
			res, err := call.Do()
			if err != nil {
				return nil, err
			}
			for _, m := range res.Messages {
				unreadIDs = append(unreadIDs, m.Id)
			}
			if len(unreadIDs) >= limit || res.NextPageToken == "" {
				break
			}
			unreadPageToken = res.NextPageToken
		}
		return unreadIDs, nil
	}
	unreadIDs, err := fetchUnreadIDs()
	if err != nil {
		slog.Error("Failed to fetch unread message IDs from server", slog.Any("error", err))
	} else {
		unreadSet := make(map[string]bool, len(unreadIDs))
		for _, id := range unreadIDs {
			unreadSet[id] = true
		}
		for _, id := range allIDs {
			readIDs[id] = !unreadSet[id]
		}
		if err := cache.SaveReadState(downloadDir, readIDs); err != nil {
			slog.Error("Failed to save read state after server sync", slog.Any("error", err))
		}
	}

	if len(missingIDs) > 0 {
		slog.Info("Downloading new emails to local cache", slog.Int("count", len(missingIDs)), slog.String("folder", folderName))

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
		var mu sync.Mutex
		count := 0
		total := len(missingIDs)

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

					mu.Lock()
					count++
					if err := msg.Store(downloadDir, msgID, rawBytes, time.Now()); err != nil {
						slog.Error("Failed to store message", slog.String("msgID", msgID), slog.Any("error", err))
					}
					_ = count
					_ = total
					mu.Unlock()

					slog.Info("Cached email locally", slog.String("message_id", msgID), slog.String("folder", folderName))
				}
			}()
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				return nil, err
			}
		}
	}

	if err := label.ReplaceAll(downloadDir, folderName, allIDs); err != nil {
		slog.Error("Failed to save folder index", slog.Any("error", err))
	}

	return allIDs, nil
}

func ensureLabelHierarchyREST(srv *gmailapi.Service, labelName string, labelNameToID map[string]string, labelIDToName map[string]string) (string, error) {
	var parts []string
	for _, p := range strings.Split(labelName, "/") {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid empty label name: %q", labelName)
	}

	var currentPath []string
	var targetLabelID string

	for _, part := range parts {
		currentPath = append(currentPath, part)
		prefixLabel := strings.Join(currentPath, "/")
		lowerPrefix := strings.ToLower(prefixLabel)

		if id, ok := labelNameToID[lowerPrefix]; ok {
			targetLabelID = id
			if labelIDToName[id] != prefixLabel {
				prefixLabel = labelIDToName[id]
				currentPath = strings.Split(prefixLabel, "/")
			}
		} else {
			newL, err := srv.Users.Labels.Create("me", &gmailapi.Label{
				Name:                  prefixLabel,
				LabelListVisibility:   "labelShow",
				MessageListVisibility: "show",
			}).Do()
			if err != nil {
				return "", fmt.Errorf("failed to create label %q: %w", prefixLabel, err)
			}
			targetLabelID = newL.Id
			labelNameToID[lowerPrefix] = targetLabelID
			labelIDToName[targetLabelID] = prefixLabel
			fmt.Printf("%s Created new Gmail label: %q\n", app.PrefixSuccess, prefixLabel)
		}
	}

	return targetLabelID, nil
}

func rewriteRuleLabelCasing(config *Config, labelToAdd string) string {
	srv, err := GetGmailService(config)
	if err != nil {
		return labelToAdd
	}

	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return labelToAdd
	}

	gmailLabels := make(map[string]string)
	for _, l := range labelsRes.Labels {
		gmailLabels[strings.ToLower(l.Name)] = l.Name
	}

	parts := strings.Split(labelToAdd, "/")
	rewritten := false
	for i := 1; i <= len(parts); i++ {
		prefix := strings.Join(parts[:i], "/")
		lowerPrefix := strings.ToLower(prefix)
		if exactName, ok := gmailLabels[lowerPrefix]; ok {
			exactParts := strings.Split(exactName, "/")
			for idx, val := range exactParts {
				if idx >= len(parts) {
					break
				}
				if parts[idx] != val {
					parts[idx] = val
					rewritten = true
				}
			}
		}
	}

	if rewritten {
		newLabel := strings.Join(parts, "/")
		if newLabel != labelToAdd {
			fmt.Printf("%s Casing mismatch detected: Rewriting label %q to %q to match existing Gmail folders.\n",
				app.PrefixInfo, labelToAdd, newLabel)
			return newLabel
		}
	}

	return labelToAdd
}
