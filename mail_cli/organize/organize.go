package organize

import (
	"fmt"
	"strings"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func All(config *cfg_g.Config, client interface {
	mailclient.LabelReader
	mailclient.EmailFetcher
	mailclient.EmailWriter
	mailclient.ConfigProvider
}, sourcePrefix string, targetFolder string) error {
	resolvedLabel, err := mailclient.ResolveLabel(client, sourcePrefix)
	if err != nil {
		return err
	}
	matchedLabels, err := client.GetMatchingLabels(resolvedLabel)
	if err != nil {
		return err
	}
	if len(matchedLabels) == 0 {
		return fmt.Errorf("no labels found matching prefix %q", resolvedLabel)
	}

	for _, matchedLabel := range matchedLabels {
		if strings.EqualFold(matchedLabel, targetFolder) {
			if config.Verbose {
				fmt.Printf("%s Skipping source label %s because it matches target archive folder %s.\n", app.PrefixInfo, matchedLabel, targetFolder)
			}
			continue
		}
		cacheDirName := cfg_g.SanitizeLabelForCache(matchedLabel)
		messageIDs, err := client.FetchAndDownloadEmails(matchedLabel, cacheDirName)
		if err != nil {
			return err
		}
		if len(messageIDs) == 0 {
			fmt.Printf("%s No messages found in label %s.\n", app.PrefixInfo, matchedLabel)
			continue
		}
		if config.ReadOnly {
			fmt.Printf("[DRY-RUN] Would archive %d message(s) to %s\n", len(messageIDs), targetFolder)
			continue
		}
		fmt.Printf("%s Moving %d message(s) from %s to %s on server...\n", app.PrefixInfo, len(messageIDs), matchedLabel, targetFolder)
		if err := client.MoveEmail(messageIDs, matchedLabel, targetFolder); err != nil {
			return fmt.Errorf("failed to move messages from %s: %w", matchedLabel, err)
		}
		for _, msgID := range messageIDs {
			cd := client.Config().DownloadDir
			_ = label.Move(cd, msgID, matchedLabel, targetFolder)
			_ = msg.ClearClassification(cd, msgID)
		}
		fmt.Printf("%s Successfully archived %d message(s) from %s to %s.\n", app.PrefixSuccess, len(messageIDs), matchedLabel, targetFolder)
	}
	return nil
}

func ResolveArchiveTarget(client interface {
	mailclient.LabelReader
	mailclient.BackendInfo
	mailclient.ConfigProvider
}) (string, error) {
	labels, err := client.GetMatchingLabels("")
	if err != nil {
		return "", err
	}
	for _, l := range labels {
		if strings.EqualFold(l, "archive") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.HasSuffix(strings.ToLower(l), "/archive") {
			return l, nil
		}
	}
	recFolder := client.InboxFolder()
	if recFolder != "" && !strings.EqualFold(recFolder, "inbox") {
		for _, l := range labels {
			if strings.EqualFold(l, recFolder) {
				return l, nil
			}
		}
		for _, l := range labels {
			if strings.HasSuffix(strings.ToLower(l), "/"+strings.ToLower(recFolder)) {
				return l, nil
			}
		}
	}
	for _, l := range labels {
		if strings.EqualFold(l, "received") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.HasSuffix(strings.ToLower(l), "/received") {
			return l, nil
		}
	}

	// Fallback for Gmail: if it's Gmail, we can use "Archive" even if it doesn't exist on the server
	isGmail := client.BackendType() == "gmail"
	if isGmail {
		return "Archive", nil
	}

	return "", fmt.Errorf("archive target folder not found on server (neither 'Archive' nor 'Received' exists)")
}

func ResolveTrashTarget(client mailclient.LabelReader) (string, error) {
	labels, err := client.GetMatchingLabels("")
	if err != nil {
		return "", err
	}
	for _, l := range labels {
		if strings.EqualFold(l, "trash") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.HasSuffix(strings.ToLower(l), "/trash") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.EqualFold(l, "deleted items") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.HasSuffix(strings.ToLower(l), "/deleted items") {
			return l, nil
		}
	}
	for _, l := range labels {
		if strings.EqualFold(l, "deleted") {
			return l, nil
		}
	}
	return "", fmt.Errorf("trash target folder not found on server (neither 'Trash' nor 'Deleted Items' exists)")
}

func ArchiveByID(config *cfg_g.Config, client interface {
	mailclient.EmailWriter
	mailclient.ConfigProvider
}, targetFolder string, targetID string) error {
	msgID, folderName, err := cache.FindCachedEmailByID(client.Config().DownloadDir, targetID)
	if err != nil {
		return err
	}

	if config.ReadOnly {
		fmt.Printf("[DRY-RUN] Would archive message %s to %s\n", targetID, targetFolder)
		return nil
	}

	fmt.Printf("%s Moving message %s (%s) to %s on server...\n", app.PrefixInfo, targetID, msgID, targetFolder)
	if err := client.MoveEmail([]string{msgID}, folderName, targetFolder); err != nil {
		return fmt.Errorf("failed to move message: %w", err)
	}

	cd := client.Config().DownloadDir
	_ = label.Move(cd, msgID, folderName, targetFolder)
	_ = msg.ClearClassification(cd, msgID)

	fmt.Printf("%s Successfully archived message %s to %s.\n", app.PrefixSuccess, targetID, targetFolder)
	return nil
}
