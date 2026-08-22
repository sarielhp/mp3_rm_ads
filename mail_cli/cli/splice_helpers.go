package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
)

// isMessageInDest checks whether a message already exists in the destination folder.
// Results are cached per folder in destCache. Callers must delete(destCache, destFolder)
// after uploading to a folder to force a refresh on the next check.
func isMessageInDest(destClient mailclient.MailClient, destFolder string, msgID string, srcEmail *email.Email, destCache map[string]map[string]bool) bool {
	idsMap, ok := destCache[destFolder]
	if !ok {
		cacheSubdir := cfg_g.SanitizeLabelForCache(destFolder)
		if cacheSubdir == "" {
			cacheSubdir = destFolder
		}
		targetIDs, err := destClient.FetchAndDownloadEmails(destFolder, cacheSubdir)
		idsMap = make(map[string]bool)
		if err == nil {
			for _, id := range targetIDs {
				idsMap[id] = true
			}
		} else {
			slog.Warn("isMessageInDest: failed to fetch destination folder", slog.String("folder", destFolder), slog.Any("error", err))
		}
		destCache[destFolder] = idsMap
	}
	if idsMap[msgID] {
		return true
	}
	if srcEmail != nil {
		destDir := destClient.Config().DownloadDir
		for eID := range idsMap {
			rawBytes, err := msg.Read(destDir, eID)
			if err != nil {
				slog.Warn("isMessageInDest: read error for cached email", slog.String("msgID", eID), slog.Any("error", err))
				continue
			}
			destEmail := email.ParseReader(bytes.NewReader(rawBytes), eID, "")
			if destEmail != nil {
				if srcEmail.MessageID != "" && destEmail.MessageID != "" && strings.EqualFold(srcEmail.MessageID, destEmail.MessageID) {
					return true
				}
				if srcEmail.Subject != "" && destEmail.Subject == srcEmail.Subject && srcEmail.EmailDate.Equal(destEmail.EmailDate) {
					return true
				}
			}
		}
	}
	return false
}

// confirmMessageInDest verifies a message exists on the destination server by fetching
// from the server (not cache) and checking by ID, Message-ID, or Subject+Date match.
// Retries with exponential backoff to account for server eventual consistency.
func confirmMessageInDest(destClient mailclient.MailClient, destFolder string, msgID string, srcEmail *email.Email) (bool, error) {
	cacheSubdir := cfg_g.SanitizeLabelForCache(destFolder)
	if cacheSubdir == "" {
		cacheSubdir = destFolder
	}

	delay := confirmRetryDelay
	for attempt := 1; attempt <= maxConfirmRetries; attempt++ {
		targetIDs, err := destClient.FetchAndDownloadEmails(destFolder, cacheSubdir)
		if err != nil {
			if attempt < maxConfirmRetries {
				slog.Debug("confirmMessageInDest: retry after fetch error", slog.Int("attempt", attempt), slog.Any("error", err))
				time.Sleep(delay)
				delay *= 2
				continue
			}
			return false, fmt.Errorf("failed to fetch destination folder %q from server after %d attempts: %w", destFolder, attempt, err)
		}

		destDir := destClient.Config().DownloadDir
		for _, eID := range targetIDs {
			if eID == msgID {
				return true, nil
			}
			if srcEmail != nil {
				rawBytes, err := msg.Read(destDir, eID)
				if err != nil {
					continue
				}
				destEmail := email.ParseReader(bytes.NewReader(rawBytes), eID, "")
				if destEmail != nil {
					if srcEmail.MessageID != "" && destEmail.MessageID != "" && strings.EqualFold(srcEmail.MessageID, destEmail.MessageID) {
						return true, nil
					}
					if srcEmail.Subject != "" && destEmail.Subject == srcEmail.Subject && srcEmail.EmailDate.Equal(destEmail.EmailDate) {
						return true, nil
					}
				}
			}
		}

		if attempt < maxConfirmRetries {
			slog.Debug("confirmMessageInDest: message not found yet, retrying", slog.Int("attempt", attempt), slog.String("msgID", msgID), slog.String("folder", destFolder))
			time.Sleep(delay)
			delay *= 2
		}
	}

	return false, fmt.Errorf("message %s not confirmed on server in destination folder %q after %d attempts", msgID, destFolder, maxConfirmRetries)
}

// verifySourceMessage checks that a message still exists in the source folder
// on the server before we delete it. This prevents data loss when two processes
// operate on the same folder concurrently.
func verifySourceMessage(client mailclient.MailClient, srcFolder string, msgID string, baseDir string) (bool, error) {
	cacheSubdir := cfg_g.SanitizeLabelForCache(srcFolder)
	if cacheSubdir == "" {
		cacheSubdir = srcFolder
	}
	ids, err := client.FetchAndDownloadEmails(srcFolder, cacheSubdir)
	if err != nil {
		return false, fmt.Errorf("failed to verify source: %w", err)
	}
	for _, id := range ids {
		if id == msgID {
			return true, nil
		}
	}
	// Also check locally — the message might have been moved already by a previous splice
	exists, _ := msg.Exists(baseDir, msgID)
	return exists, nil
}

func ValidateSpliceArgs(folder string, numMessages int, allowKeep bool) error {
	if !allowKeep && (strings.EqualFold(folder, "keep") || strings.HasPrefix(strings.ToLower(folder), "keep/")) {
		return fmt.Errorf("source folder %q must not be \"keep\" or start with \"keep/\" unless --allow is specified", folder)
	}
	if numMessages <= 0 {
		return fmt.Errorf("flag -n must be at least 1")
	}
	return nil
}

func resolveSingleLabel(client mailclient.MailClient, prefix string) (string, error) {
	resolvedName := prefix
	if strings.EqualFold(resolvedName, "inbox") {
		resolvedName = client.InboxFolder()
	}

	directMatches, err := client.GetMatchingLabels(resolvedName)
	if err == nil && len(directMatches) > 0 {
		for _, l := range directMatches {
			if strings.EqualFold(l, resolvedName) {
				return l, nil
			}
		}
		if len(directMatches) == 1 {
			return directMatches[0], nil
		}
	}

	allLabels, err := client.GetMatchingLabels("")
	if err != nil {
		return "", err
	}

	var candidates []string
	rLower := strings.ToLower(resolvedName)
	suffixMatch := "/" + rLower

	for _, l := range allLabels {
		lLower := strings.ToLower(l)
		if lLower == rLower {
			return l, nil
		}
		if strings.HasSuffix(lLower, suffixMatch) || strings.HasPrefix(lLower, rLower) {
			candidates = append(candidates, l)
		}
	}

	resolved := ResolveUniqueMatch(resolvedName, candidates)
	if resolved != "" {
		return resolved, nil
	}

	if len(candidates) > 1 {
		return "", fmt.Errorf("label %q is ambiguous on account (matches: %s)", prefix, strings.Join(candidates, ", "))
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	for _, l := range allLabels {
		if strings.Contains(strings.ToLower(l), rLower) {
			candidates = append(candidates, l)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("label %q is ambiguous on account (matches: %s)", prefix, strings.Join(candidates, ", "))
	}

	return "", fmt.Errorf("no labels found matching %q", prefix)
}

func isSameAccount(c1, c2 mailclient.MailClient) bool {
	if c1 == nil || c2 == nil {
		return false
	}
	if c1 == c2 {
		return true
	}
	// Use backend type + config identity rather than pointer comparison,
	// since CheckingMailClient wrappers may wrap the same backend.
	if c1.BackendType() != c2.BackendType() {
		return false
	}
	cfg1 := c1.Config()
	cfg2 := c2.Config()
	if cfg1 == nil || cfg2 == nil {
		return false
	}
	if cfg1.SelectedAccount != "" && cfg2.SelectedAccount != "" {
		return strings.EqualFold(cfg1.SelectedAccount, cfg2.SelectedAccount)
	}
	return c1 == c2
}
