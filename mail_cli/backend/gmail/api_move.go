package gmail

import (
	"encoding/base64"
	"fmt"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

func ensureFolderGmail(config *Config, folderName string) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	parts := strings.Split(folderName, "/")
	for _, part := range parts {
		_ = part
		labelsRes, err := srv.Users.Labels.List("me").Do()
		if err != nil {
			return fmt.Errorf("failed to list Gmail labels: %w", err)
		}
		found := false
		for _, l := range labelsRes.Labels {
			if l.Name == folderName {
				found = true
				break
			}
		}
		if !found {
			_, err := srv.Users.Labels.Create("me", &gmailapi.Label{
				Name:                  folderName,
				LabelListVisibility:   "labelShow",
				MessageListVisibility: "show",
			}).Do()
			if err != nil {
				return fmt.Errorf("failed to create label %q: %w", folderName, err)
			}
		}
	}
	return nil
}

func (c *GmailClient) EnsureLabelExists(name string) error {
	return ensureFolderGmail(c.config, name)
}

func (c *GmailClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	srv, err := GetGmailService(c.config)
	if err != nil {
		return err
	}

	// Ensure the label exists
	if err := c.EnsureLabelExists(targetLabel); err != nil {
		return err
	}

	// Resolve target label ID
	destLabelID := ""
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err == nil {
		for _, l := range labelsRes.Labels {
			if strings.EqualFold(l.Name, targetLabel) {
				destLabelID = l.Id
				break
			}
		}
	}

	if destLabelID == "" {
		return fmt.Errorf("target label %q not found on Gmail server", targetLabel)
	}

	msg := &gmailapi.Message{
		Raw:      base64.URLEncoding.EncodeToString(rawBytes),
		LabelIds: []string{destLabelID},
	}

	_, err = srv.Users.Messages.Insert("me", msg).Do()
	return err
}
