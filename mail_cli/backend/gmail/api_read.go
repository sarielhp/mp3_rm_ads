package gmail

import gmailapi "google.golang.org/api/gmail/v1"

func markAsReadGmailREST(config *Config, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}
	return srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
		Ids:            messageIDs,
		RemoveLabelIds: []string{"UNREAD"},
	}).Do()
}
