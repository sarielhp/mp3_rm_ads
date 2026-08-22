package scan

import (
	"bytes"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
	"mail_cli/ui"
	"mime"
	"net/mail"
	"strings"
)

func Perform(client interface {
	mailclient.LabelReader
	mailclient.EmailFetcher
	mailclient.EmailWriter
	mailclient.SpamManager
	mailclient.RuleManager
	mailclient.ConfigProvider
	Validate() error
}, config *cfg_g.Config, labelPrefix string, moveSpam string, moveInbox string) (int, error) {
	movedCountTotal := 0
	matchedLabels, err := client.GetMatchingLabels(labelPrefix)
	if err != nil {
		return 0, err
	}
	if len(matchedLabels) == 0 {
		return 0, fmt.Errorf("no labels found matching prefix %q", labelPrefix)
	}

	var exactMatch string
	for _, l := range matchedLabels {
		if strings.EqualFold(l, labelPrefix) {
			exactMatch = l
			break
		}
	}
	if exactMatch != "" {
		matchedLabels = []string{exactMatch}
	}

	isMoveSpam := moveSpam == "true"

	if isMoveSpam {
		for _, l := range matchedLabels {
			if strings.EqualFold(l, "SPAM") {
				return 0, fmt.Errorf("the -m (move) flag is not allowed when scanning the system spam folder itself (%s)", l)
			}
		}
	}

	for _, matchedLabel := range matchedLabels {
		if config.ReadOnly {
			fmt.Printf("%s [READ-ONLY / DRY RUN] Scanning label: %s...\n", app.PrefixInfo, matchedLabel)
		} else if config.Verbose {
			fmt.Printf("%s Scanning label: %s...\n", app.PrefixInfo, matchedLabel)
		}

		cacheDirName := cfg_g.SanitizeLabelForCache(matchedLabel)

		downloadedIDs, err := client.FetchAndDownloadEmails(matchedLabel, cacheDirName)
		if err != nil {
			return movedCountTotal, fmt.Errorf("error downloading emails for label %s: %w", matchedLabel, err)
		}

		totalCountBeforePattern := 0
		if app.FlagScanPattern != "" {
			totalCountBeforePattern = len(downloadedIDs)
			var filteredIDs []string
			baseDir := client.Config().DownloadDir
			for _, id := range downloadedIDs {
				rawBytes, err := msg.Read(baseDir, id)
				if err != nil {
					continue
				}
				parsedEmail := email.ParseReader(bytes.NewReader(rawBytes), id, "")
				if parsedEmail == nil {
					continue
				}
				if email.MatchPattern(parsedEmail.Subject, app.FlagScanPattern) {
					filteredIDs = append(filteredIDs, id)
				}
			}
			downloadedIDs = filteredIDs
		}

		if len(downloadedIDs) == 0 {
			if app.FlagScanPattern != "" {
				fmt.Printf("%s No emails in label %s match pattern %q.\n", app.PrefixInfo, matchedLabel, app.FlagScanPattern)
			} else {
				fmt.Printf("%s No emails found in label %s.\n", app.PrefixInfo, matchedLabel)
			}
			continue
		}

		if app.FlagExplicitScanInbox && (strings.EqualFold(matchedLabel, "inbox") || strings.EqualFold(matchedLabel, config.ReceivedFolder)) {
			fmt.Printf("    %s Explicit 'scan inbox' detected. Clearing cached classifications for %d email(s) to force re-labeling...\n", app.PrefixInfo, len(downloadedIDs))
			for _, id := range downloadedIDs {
				_ = msg.ClearClassification(config.DownloadDir, id)
			}
		}

		origCount := len(downloadedIDs)
		downloadedIDs, err = client.CheckAndApplyRules(downloadedIDs, matchedLabel, cacheDirName)
		if err != nil {
			return movedCountTotal, fmt.Errorf("error applying rules for label %s: %w", matchedLabel, err)
		}
		movedCount := origCount - len(downloadedIDs)

		if len(downloadedIDs) == 0 {
			fmt.Printf("%s All emails in label %s were processed and labeled by rules.\n", app.PrefixSuccess, matchedLabel)
			if movedCount > 0 {
				fmt.Printf("Messages moved by rules: %d\n", movedCount)
			}
			continue
		}

		if moveInbox != "" {
			targetSender := strings.ToLower(strings.TrimSpace(moveInbox))
			var matchingIDs []string

			for _, id := range downloadedIDs {
				emailBytes, rErr := msg.Read(config.DownloadDir, id)
				if rErr != nil {
					continue
				}
				localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
				if errMail == nil {
					fromHeader := localEmail.Header.Get("From")
					sender := strings.ToLower(email.ParseEmailAddress(fromHeader))
					if sender == targetSender {
						matchingIDs = append(matchingIDs, id)
					}
				}
			}

			if len(matchingIDs) > 0 {
				if config.ReadOnly {
					fmt.Printf("[DRY-RUN] Would move %d message(s) to inbox\n", len(matchingIDs))
				} else {
					fmt.Printf("%s Found %d message(s) from '%s' in label %s. Moving to Inbox...\n", app.PrefixInfo, len(matchingIDs), moveInbox, matchedLabel)
					err = client.MoveToInbox(matchingIDs, matchedLabel)
					if err != nil {
						return movedCountTotal, fmt.Errorf("failed to move messages to inbox: %w", err)
					}
					fmt.Printf("%s Successfully moved %d message(s) to inbox.\n", app.PrefixSuccess, len(matchingIDs))
					movedCountTotal += len(matchingIDs)
				}

				movedMap := make(map[string]bool)
				for _, id := range matchingIDs {
					movedMap[id] = true
				}
				var filteredIDs []string
				for _, id := range downloadedIDs {
					if !movedMap[id] {
						filteredIDs = append(filteredIDs, id)
					}
				}
				downloadedIDs = filteredIDs

				if len(downloadedIDs) == 0 {
					continue
				}
			} else {
				if config.Verbose {
					fmt.Printf("%s No messages from '%s' found in label %s.\n", app.PrefixInfo, moveInbox, matchedLabel)
				}
			}
		}

		if moveSpam != "" && moveSpam != "true" {
			targetSender := strings.ToLower(strings.TrimSpace(moveSpam))
			var matchingIDs []string
			dec := new(mime.WordDecoder)

			for _, id := range downloadedIDs {
				emailBytes, rErr := msg.Read(config.DownloadDir, id)
				if rErr != nil {
					continue
				}
				localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
				if errMail == nil {
					fromHeader := localEmail.Header.Get("From")
					sender := strings.ToLower(email.ParseEmailAddress(fromHeader))
					if sender == targetSender {
						matchingIDs = append(matchingIDs, id)
					}
				}
			}

			if len(matchingIDs) == 1 {
				uniqueID := matchingIDs[0]

				subject := ""
				if data, rErr := msg.Read(config.DownloadDir, uniqueID); rErr == nil {
					if msg, errParse := mail.ReadMessage(bytes.NewReader(data)); errParse == nil {
						subject = email.DecodeHeader(dec, msg.Header.Get("Subject"))
					}
				}
				if subject == "" {
					subject = "(No Subject)"
				}

				if config.ReadOnly {
					fmt.Printf("[DRY-RUN] Would move message %s to spam\n", uniqueID)
				} else {
					fmt.Printf("%s Found unique message from '%s' (Subject: %q, ID: %s). Moving to Spam...\n", app.PrefixInfo, moveSpam, subject, uniqueID)
					err = client.ReportSpam([]string{uniqueID}, matchedLabel)
					if err != nil {
						return movedCountTotal, fmt.Errorf("failed to move unique message to spam: %w", err)
					}
					fmt.Printf("%s Successfully moved unique message to spam.\n", app.PrefixSuccess)
					movedCountTotal++
				}

				var filteredIDs []string
				for _, id := range downloadedIDs {
					if id != uniqueID {
						filteredIDs = append(filteredIDs, id)
					}
				}
				downloadedIDs = filteredIDs

				if len(downloadedIDs) == 0 {
					continue
				}
			} else if len(matchingIDs) > 1 {
				fmt.Printf("%s Found %d messages from '%s' in label %s (not unique, skipping move).\n", app.PrefixWarn, len(matchingIDs), moveSpam, matchedLabel)
			} else {
				if config.Verbose {
					fmt.Printf("%s No messages from '%s' found in label %s.\n", app.PrefixInfo, moveSpam, matchedLabel)
				}
			}
		}

		spamIDs, blacklistedIDs, scanResults, err := ScanEmails(downloadedIDs, config, cacheDirName)
		if err != nil {
			return movedCountTotal, fmt.Errorf("error scanning emails with Bogofilter for label %s: %w", matchedLabel, err)
		}

		isSpamFolder := strings.EqualFold(matchedLabel, config.SpamFolder) || strings.EqualFold(matchedLabel, "spam") || strings.EqualFold(matchedLabel, "[gmail]/spam")

		if isMoveSpam && len(spamIDs) > 0 {
			if config.ReadOnly {
				fmt.Printf("[DRY-RUN] Would move %d message(s) to %s\n", len(spamIDs), config.SpamFolder)
			} else {
				if config.Verbose {
					fmt.Printf("%s Reporting %d spam emails to mail server...\n", app.PrefixInfo, len(spamIDs))
				}
				err = client.ReportSpam(spamIDs, matchedLabel)
				if err != nil {
					return movedCountTotal, fmt.Errorf("error reporting spam to mail server for label %s: %w", matchedLabel, err)
				}
				fmt.Printf("%s Successfully reported and moved %d spam emails.\n", app.PrefixSuccess, len(spamIDs))
				movedCountTotal += len(spamIDs)
			}
		}

		if len(blacklistedIDs) > 0 && config.SpamLearn != "" {
			if config.ReadOnly {
				fmt.Printf("[DRY-RUN] Would move %d blacklisted message(s) from %s to %s\n", len(blacklistedIDs), matchedLabel, config.SpamLearn)
			} else {
				if config.Verbose {
					fmt.Printf("%s Moving %d blacklisted email(s) from %s to %s...\n", app.PrefixInfo, len(blacklistedIDs), matchedLabel, config.SpamLearn)
				}
				err = client.MoveEmail(blacklistedIDs, matchedLabel, config.SpamLearn)
				if err != nil {
					return movedCountTotal, fmt.Errorf("failed to move blacklisted emails from %s to %s: %w", matchedLabel, config.SpamLearn, err)
				}
				fmt.Printf("%s Moved %d blacklisted message(s) from %s to %s.\n", app.PrefixSuccess, len(blacklistedIDs), matchedLabel, config.SpamLearn)
				movedCountTotal += len(blacklistedIDs)
			}
		}

		folderTitle := fmt.Sprintf("Folder: %s:%s (scan).", config.GetSelectedAccountDisplayName(), matchedLabel)
		ui.PrintFolderSummary(folderTitle, "", downloadedIDs, spamIDs, scanResults, config, cacheDirName, movedCount, totalCountBeforePattern)

		if !isMoveSpam && len(spamIDs) > 0 && !isSpamFolder {
			fmt.Printf("%s Run the scan with the -m flag to report and move these messages as spam.\n", app.PrefixInfo)
		}

		if len(blacklistedIDs) > 0 && config.SpamLearn == "" {
			fmt.Printf("%s Warning: %d blacklisted email(s) detected but no SpamLearn folder configured.\n", app.PrefixWarn, len(blacklistedIDs))
		}

		if isSpamFolder && len(downloadedIDs) > len(spamIDs) {
			fmt.Printf("%s Run 'spam learn force' to train the classifier and mark these emails as spam.\n", app.PrefixInfo)
		}
	}
	return movedCountTotal, nil
}
