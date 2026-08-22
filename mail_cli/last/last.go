package last

import (
	"bytes"
	"fmt"
	"mime"
	"net/mail"
	"sort"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
	"mail_cli/ui"
	"mail_cli/uicommon"
)

// EmailWithFolder pairs an email with the folder it belongs to.
type EmailWithFolder struct {
	Email  uicommon.FolderEmail
	Folder string
}

// Perform fetches emails across the account, collects the last N received,
// stores them into a virtual mailbox named "last", and prints them showing each email's folder.
func Perform(client mailclient.MailClient, config *cfg_g.Config, n int) error {
	if n <= 0 {
		return fmt.Errorf("number of emails must be greater than 0")
	}

	fmt.Printf("%s Fetching last %d emails from server...\n", app.PrefixInfo, n)

	// 1. Try server-side account-wide fetch first
	var refs []cfg_acc.MessageFolderRef
	var err error
	if client != nil {
		refs, err = client.FetchLatestAccountEmails(n)
	}

	// 2. Fall back to folder-by-folder scan if account-wide query is not supported or returned nothing
	if err != nil || len(refs) == 0 {
		return performFolderByFolderFallback(client, config, n)
	}

	// 3. Build email list from message refs
	var allEmails []EmailWithFolder
	dec := new(mime.WordDecoder)
	for _, ref := range refs {
		eml := parseMessageData(config.DownloadDir, ref.MessageID, dec)
		allEmails = append(allEmails, EmailWithFolder{
			Folder: ref.Folder,
			Email:  eml,
		})
	}

	return finishAndDisplay(allEmails, config, n)
}

func parseMessageData(downloadDir, id string, dec *mime.WordDecoder) uicommon.FolderEmail {
	data, rErr := msg.Read(downloadDir, id)
	var subject, fromEmail, fromRaw string
	var emailDate time.Time
	var messageID, inReplyTo, references string

	if rErr != nil {
		subject = fmt.Sprintf("[Error reading cached message ID: %s]", id)
		fromEmail = "[Error]"
		fromRaw = "[Error]"
	} else {
		parsedMsg, errParse := mail.ReadMessage(bytes.NewReader(data))
		if errParse != nil {
			subject = fmt.Sprintf("[Error parsing cached message ID: %s]", id)
			fromEmail = "[Error]"
			fromRaw = "[Error]"
		} else {
			subject = email.DecodeHeader(dec, parsedMsg.Header.Get("Subject"))
			if subject == "" {
				subject = "(No Subject)"
			}
			fromEmail = email.ParseEmailAddress(parsedMsg.Header.Get("From"))
			if fromEmail == "" {
				fromEmail = "(Unknown Sender)"
			}
			fromRaw = strings.TrimSpace(parsedMsg.Header.Get("From"))
			if fromRaw == "" {
				fromRaw = "(Unknown Sender)"
			}
			dateStrHeader := parsedMsg.Header.Get("Date")
			if dateStrHeader != "" {
				if parsedDate, errD := mail.ParseDate(dateStrHeader); errD == nil {
					emailDate = parsedDate
				}
			}
			messageID = strings.TrimSpace(parsedMsg.Header.Get("Message-ID"))
			inReplyTo = strings.TrimSpace(parsedMsg.Header.Get("In-Reply-To"))
			references = strings.TrimSpace(parsedMsg.Header.Get("References"))
		}
	}

	if emailDate.IsZero() {
		if info, errInfo := msg.GetInfo(downloadDir, id); errInfo == nil && !info.Date.IsZero() {
			emailDate = info.Date
		} else {
			emailDate = time.Now()
		}
	}

	formattedDate := email.FormatEmailDate(emailDate)
	subject = uicommon.ReverseHebrewRuns(subject)

	isSpam := false
	isPolitical := false
	isBlacklisted := false
	if info, errInfo := msg.GetInfo(downloadDir, id); errInfo == nil {
		isSpam = info.IsSpam
		isPolitical = info.IsPolitical
		isBlacklisted = info.IsBlacklisted
	}

	return uicommon.FolderEmail{
		ID:            id,
		Subject:       subject,
		FromEmail:     fromEmail,
		FromRaw:       fromRaw,
		EmailDate:     emailDate,
		FormattedDate: formattedDate,
		IsSpam:        isSpam,
		IsPolitical:   isPolitical,
		IsBlacklisted: isBlacklisted,
		MessageID:     messageID,
		InReplyTo:     inReplyTo,
		References:    references,
	}
}

func performFolderByFolderFallback(client mailclient.MailClient, config *cfg_g.Config, n int) error {
	folders, err := client.GetLabelItems()
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}

	if len(folders) == 0 {
		fmt.Printf("%s No folders found in account.\n", app.PrefixInfo)
		return nil
	}

	var allEmails []EmailWithFolder
	seenMsgIDs := make(map[string]bool)
	dec := new(mime.WordDecoder)

	for _, f := range folders {
		folderName := f.FullName
		if folderName == "" {
			folderName = f.Name
		}
		if folderName == "" {
			continue
		}

		cacheDirName := cfg_g.SanitizeLabelForCache(folderName)
		downloadedIDs, fetchErr := client.FetchAndDownloadEmails(folderName, cacheDirName)
		if fetchErr != nil {
			if config.Verbose {
				fmt.Printf("%s Warning: failed to fetch emails from folder %q: %v\n", app.PrefixWarn, folderName, fetchErr)
			}
			continue
		}

		for _, id := range downloadedIDs {
			if seenMsgIDs[id] {
				continue
			}
			seenMsgIDs[id] = true

			eml := parseMessageData(config.DownloadDir, id, dec)
			allEmails = append(allEmails, EmailWithFolder{
				Folder: folderName,
				Email:  eml,
			})
		}
	}

	return finishAndDisplay(allEmails, config, n)
}

func finishAndDisplay(allEmails []EmailWithFolder, config *cfg_g.Config, n int) error {
	if len(allEmails) == 0 {
		fmt.Printf("%s No emails found.\n", app.PrefixInfo)
		return nil
	}

	// 1. Sort all emails by date descending (most recently received first)
	sort.Slice(allEmails, func(i, j int) bool {
		return allEmails[i].Email.EmailDate.After(allEmails[j].Email.EmailDate)
	})

	// 2. Keep only the top N
	if len(allEmails) > n {
		allEmails = allEmails[:n]
	}

	// 3. Reverse to chronological order (oldest to newest among the last N)
	sort.Slice(allEmails, func(i, j int) bool {
		return allEmails[i].Email.EmailDate.Before(allEmails[j].Email.EmailDate)
	})

	// 4. Save to VirtualMailbox "last"
	vm := &VirtualMailbox{
		Name:        "last",
		Description: fmt.Sprintf("Last %d emails received across all folders", len(allEmails)),
		MessageIDs:  make([]string, len(allEmails)),
		FolderMap:   make(map[string]string),
	}
	for i, ef := range allEmails {
		vm.MessageIDs[i] = ef.Email.ID
		vm.FolderMap[ef.Email.ID] = ef.Folder
	}
	_ = Save(config.DownloadDir, vm)

	// 5. Print results with folder names
	var emailsOnly []uicommon.FolderEmail
	folderMap := make(map[string]string)
	for _, ef := range allEmails {
		emailsOnly = append(emailsOnly, ef.Email)
		folderMap[ef.Email.ID] = ef.Folder
	}

	ui.PrintEmailsWithFolders(
		fmt.Sprintf("LAST %d EMAILS (ACROSS ALL FOLDERS)", len(allEmails)),
		emailsOnly,
		folderMap,
		config,
	)

	return nil
}
