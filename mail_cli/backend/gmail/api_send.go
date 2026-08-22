package gmail

import (
	"encoding/base64"
	"fmt"
	gmailapi "google.golang.org/api/gmail/v1"
)

func (c *GmailClient) SendEmail(rawBytes []byte) error {
	return sendGmailEmailREST(c.config, rawBytes)
}

func sendGmailEmailREST(config *Config, rawBytes []byte) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	rawStr := base64.URLEncoding.EncodeToString(rawBytes)
	gMsg := &gmailapi.Message{
		Raw: rawStr,
	}

	_, err = srv.Users.Messages.Send("me", gMsg).Do()
	if err != nil {
		return fmt.Errorf("failed to send raw email via Gmail REST API: %w", err)
	}
	return nil
}
