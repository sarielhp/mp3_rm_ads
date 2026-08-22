package gmail

import (
	"encoding/base64"
	"fmt"
	"mail_cli/app"
	"net/url"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

// reportSpamToGmailREST reports the emails as spam by adding the SPAM label and removing the source folder label.
func reportSpamToGmailREST(messageIDs []string, config *Config, sourceLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	sourceLabelID, err := resolveLabelIDByName(srv, config, sourceLabelName)
	if err != nil {
		return err
	}

	removeLabels := []string{}
	if sourceLabelID != "" && !strings.EqualFold(sourceLabelID, "SENT") {
		removeLabels = append(removeLabels, sourceLabelID)
	}

	err = srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
		Ids:            messageIDs,
		AddLabelIds:    []string{"SPAM"},
		RemoveLabelIds: removeLabels,
	}).Do()
	if err != nil {
		return fmt.Errorf("failed to batch report spam: %w", err)
	}

	return nil
}

// moveToInboxGmailREST moves the specified messages back to the INBOX by adding the INBOX label and removing the source label.
func moveToInboxGmailREST(messageIDs []string, config *Config, sourceLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	if strings.EqualFold(sourceLabelName, "inbox") {
		if config.Verbose {
			fmt.Printf("    [Debug] MoveToInbox: message already in Inbox. Skipping.\n")
		}
		return nil
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	sourceLabelID, err := resolveLabelIDByName(srv, config, sourceLabelName)
	if err != nil {
		return err
	}
	removeLabels := []string{}
	if sourceLabelID != "" && sourceLabelID != "INBOX" && !strings.EqualFold(sourceLabelID, "SENT") {
		removeLabels = append(removeLabels, sourceLabelID)
	}
	removeLabels = append(removeLabels, "SPAM")

	addLabels := []string{"INBOX"}
	if config.UnspamLearn != "" {
		labelsRes, listErr := srv.Users.Labels.List("me").Do()
		if listErr == nil {
			labelNameToID := make(map[string]string)
			labelIDToName := make(map[string]string)
			for _, l := range labelsRes.Labels {
				labelNameToID[strings.ToLower(l.Name)] = l.Id
				labelIDToName[l.Id] = l.Name
			}
			unspamLabelID, errEnsure := ensureLabelHierarchyREST(srv, config.UnspamLearn, labelNameToID, labelIDToName)
			if errEnsure == nil {
				addLabels = append(addLabels, unspamLabelID)
			} else {
				fmt.Printf("    %s Warning: failed to ensure Gmail label %q: %v\n", app.PrefixWarn, config.UnspamLearn, errEnsure)
			}
		}
	}

	err = srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
		Ids:            messageIDs,
		AddLabelIds:    addLabels,
		RemoveLabelIds: removeLabels,
	}).Do()
	if err != nil {
		return fmt.Errorf("failed to batch move to inbox: %w", err)
	}

	return nil
}

// deleteAllSpamInGmailREST permanently deletes all messages in the Gmail Spam folder forever.
func deleteAllSpamInGmailREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Fetching all spam email IDs to delete permanently...\n", app.PrefixInfo)

	totalDeleted := 0
	batchSize := 1000
	pageToken := ""
	for {
		call := srv.Users.Messages.List("me").Q("label:SPAM").IncludeSpamTrash(true)
		if pageToken != "" {
			call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return fmt.Errorf("failed to list spam for deletion: %w", err)
		}

		if len(res.Messages) > 0 {
			var ids []string
			for _, m := range res.Messages {
				ids = append(ids, m.Id)
			}
			for i := 0; i < len(ids); i += batchSize {
				end := i + batchSize
				if end > len(ids) {
					end = len(ids)
				}
				err = srv.Users.Messages.BatchDelete("me", &gmailapi.BatchDeleteMessagesRequest{
					Ids: ids[i:end],
				}).Do()
				if err != nil {
					return fmt.Errorf("failed to batch delete spam: %w", err)
				}
			}
			totalDeleted += len(ids)
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	if totalDeleted == 0 {
		fmt.Printf("%s Gmail Spam folder is already empty!\n", app.PrefixSuccess)
		return nil
	}

	fmt.Printf("%s Successfully deleted and expunged ", app.PrefixSuccess)
	app.ColorBoldGreen.Printf("%d", totalDeleted)
	fmt.Println(" emails from Gmail Spam folder forever!")
	return nil
}

// executeMailtoUnsubscribeREST sends the mailto unsubscribe email using the Gmail REST API (Send).
func executeMailtoUnsubscribeREST(srv *gmailapi.Service, config *Config, mailtoLink string) error {
	u, err := url.Parse(mailtoLink)
	if err != nil {
		return err
	}

	toEmail := u.Path
	if toEmail == "" {
		toEmail = u.Opaque
	}

	queryParams := u.Query()
	subject := queryParams.Get("subject")
	if subject == "" {
		subject = "Unsubscribe"
	}
	body := queryParams.Get("body")
	if body == "" {
		body = "Unsubscribe me from this mailing list."
	}

	rawMessage := []byte("To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	gMsg := &gmailapi.Message{
		Raw: base64.URLEncoding.EncodeToString(rawMessage),
	}

	_, err = srv.Users.Messages.Send("me", gMsg).Do()
	return err
}

// moveEmailGmailREST moves emails from a source label to a destination label.
func moveEmailGmailREST(messageIDs []string, config *Config, sourceLabelName string, destLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if strings.EqualFold(sourceLabelName, destLabelName) {
		if config.Verbose {
			fmt.Printf("    [Debug] MoveEmail: source and destination are the same (%q). Skipping.\n", destLabelName)
		}
		return nil
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	// Resolve source label ID
	sourceLabelID, err := resolveLabelIDByName(srv, config, sourceLabelName)
	if err != nil {
		return fmt.Errorf("failed to resolve source label %q: %w", sourceLabelName, err)
	}

	// Resolve destination label ID
	destLabelID, err := resolveLabelIDByName(srv, config, destLabelName)
	if err != nil {
		if strings.EqualFold(destLabelName, "archive") {
			// Special case for Gmail: if target is "archive" and it doesn't exist,
			// we archive by removing the source label (e.g. INBOX) without adding any label.
			destLabelID = ""
		} else {
			return err
		}
	}

	removeLabels := []string{}
	if sourceLabelID != "" && sourceLabelID != destLabelID && !strings.EqualFold(sourceLabelID, "SENT") && !strings.EqualFold(sourceLabelName, "SENT") {
		removeLabels = append(removeLabels, sourceLabelID)
	}

	if len(removeLabels) == 0 && destLabelID == "" {
		return nil
	}

	req := &gmailapi.BatchModifyMessagesRequest{
		Ids:            messageIDs,
		RemoveLabelIds: removeLabels,
	}
	if destLabelID != "" {
		req.AddLabelIds = []string{destLabelID}
	}

	err = srv.Users.Messages.BatchModify("me", req).Do()
	if err != nil {
		return fmt.Errorf("failed to batch move emails: %w", err)
	}

	return nil
}

func copyEmailGmailREST(config *Config, messageIDs []string, sourceLabelName string, destLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}
	destLabelID, err := resolveLabelIDByName(srv, config, destLabelName)
	if err != nil {
		return err
	}
	if destLabelID == "" {
		return nil
	}
	req := &gmailapi.BatchModifyMessagesRequest{
		Ids:         messageIDs,
		AddLabelIds: []string{destLabelID},
	}
	err = srv.Users.Messages.BatchModify("me", req).Do()
	if err != nil {
		return fmt.Errorf("failed to copy messages from %s to %s: %w", sourceLabelName, destLabelName, err)
	}
	return nil
}

// moveAllSpamInGmailREST moves all messages in the Gmail Spam folder to a destination label.
func moveAllSpamInGmailREST(config *Config, destLabel string) error {
	spamFolder := config.SpamFolder
	if spamFolder == "" {
		spamFolder = "SPAM"
	}
	if strings.EqualFold(spamFolder, destLabel) || strings.EqualFold("SPAM", destLabel) || strings.EqualFold("[Gmail]/Spam", destLabel) {
		fmt.Printf("%s spam_learn folder is the same as the spam folder. Purging spam emails instead...\n", app.PrefixInfo)
		return deleteAllSpamInGmailREST(config)
	}

	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Ensuring Gmail label %q exists...\n", app.PrefixInfo, destLabel)
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	labelNameToID := make(map[string]string)
	labelIDToName := make(map[string]string)
	for _, l := range labelsRes.Labels {
		labelNameToID[strings.ToLower(l.Name)] = l.Id
		labelIDToName[l.Id] = l.Name
	}

	destLabelID, errLabel := ensureLabelHierarchyREST(srv, destLabel, labelNameToID, labelIDToName)
	if errLabel != nil {
		return fmt.Errorf("failed to ensure label %q: %w", destLabel, errLabel)
	}

	fmt.Printf("%s Fetching and moving spam emails to %q...\n", app.PrefixInfo, destLabel)

	totalMoved := 0
	batchSize := 1000
	pageToken := ""
	for {
		call := srv.Users.Messages.List("me").Q("label:SPAM").IncludeSpamTrash(true)
		if pageToken != "" {
			call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return fmt.Errorf("failed to list spam for moving: %w", err)
		}

		if len(res.Messages) > 0 {
			var ids []string
			for _, m := range res.Messages {
				ids = append(ids, m.Id)
			}
			for i := 0; i < len(ids); i += batchSize {
				end := i + batchSize
				if end > len(ids) {
					end = len(ids)
				}
				err = srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
					Ids:            ids[i:end],
					AddLabelIds:    []string{destLabelID},
					RemoveLabelIds: []string{"SPAM"},
				}).Do()
				if err != nil {
					return fmt.Errorf("failed to batch move spam to %q: %w", destLabel, err)
				}
			}
			totalMoved += len(ids)
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	if totalMoved == 0 {
		fmt.Printf("%s Gmail Spam folder is already empty!\n", app.PrefixSuccess)
		return nil
	}

	fmt.Printf("%s Successfully moved %d spam emails to %q on Gmail server.\n", app.PrefixSuccess, totalMoved, destLabel)
	return nil
}
