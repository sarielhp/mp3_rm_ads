package gmail

import (
	"fmt"
	"mail_cli/app"
	"mail_cli/uicommon"
	"sort"
	"strconv"
	"strings"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"
)

// listGmailLabelsREST fetches and prints all user and system labels on Gmail.
func listGmailLabelsREST(config *Config) error {
	systemLabels, userLabels, err := fetchGmailLabelsREST(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Fetching labels from Gmail...\n", app.PrefixInfo)

	fmt.Println()
	app.ColorBoldCyan.Println("======================================================================")
	app.ColorBoldCyan.Println("                           GMAIL SYSTEM LABELS                        ")
	app.ColorBoldCyan.Println("======================================================================")
	for _, l := range systemLabels {
		if config.HideZeroLabels && l.MessagesTotal == 0 {
			continue
		}
		fmt.Printf("  • %-25s  [Total: %-4d  Unread: %-4d]\n", l.Name, l.MessagesTotal, l.MessagesUnread)
	}

	var folderItems []LabelItem
	for _, l := range userLabels {
		folderItems = append(folderItems, LabelItem{
			Name:           l.Name,
			FullName:       l.Name,
			MessagesTotal:  l.MessagesTotal,
			MessagesUnread: l.MessagesUnread,
			IsLabel:        true,
		})
	}
	uicommon.PrintLabelTree("                            GMAIL USER LABELS                         ", folderItems, config.HideZeroLabels)

	return nil
}

// renameGmailLabelREST renames an existing Gmail label by name to a new name.
func renameGmailLabelREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	oldName := strings.TrimSpace(config.LabelsValOld)
	newName := strings.TrimSpace(config.LabelsValNew)

	fmt.Printf("%s Searching for label %q on Gmail...\n", app.PrefixInfo, oldName)

	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	var targetLabel *gmailapi.Label
	var destLabel *gmailapi.Label
	for _, l := range labelsRes.Labels {
		if strings.EqualFold(l.Name, oldName) {
			targetLabel = l
		}
		if strings.EqualFold(l.Name, newName) {
			destLabel = l
		}
	}

	if targetLabel == nil {
		return fmt.Errorf("label %q not found on Gmail", oldName)
	}

	if strings.EqualFold(targetLabel.Type, "system") {
		return fmt.Errorf("system label %q cannot be renamed", oldName)
	}

	if destLabel != nil {
		if targetLabel.Id == destLabel.Id {
			fmt.Printf("%s Label is already named %q (no-op).\n", app.PrefixSuccess, targetLabel.Name)
			return nil
		}

		fmt.Printf("%s Destination label %q already exists (ID: %s). Migrating emails from %q to %q...\n",
			app.PrefixInfo, destLabel.Name, destLabel.Id, targetLabel.Name, destLabel.Name)

		pageToken := ""
		totalMoved := 0
		for {
			req := srv.Users.Messages.List("me").LabelIds(targetLabel.Id).IncludeSpamTrash(true)
			if pageToken != "" {
				req = req.PageToken(pageToken)
			}
			res, err := req.Do()
			if err != nil {
				return fmt.Errorf("failed to list messages in label %q: %w", targetLabel.Name, err)
			}

			if len(res.Messages) > 0 {
				var ids []string
				for _, m := range res.Messages {
					ids = append(ids, m.Id)
				}

				removeLabels := []string{}
				if targetLabel.Id != "SENT" && !strings.EqualFold(targetLabel.Name, "SENT") {
					removeLabels = append(removeLabels, targetLabel.Id)
				}
				err = srv.Users.Messages.BatchModify("me", &gmailapi.BatchModifyMessagesRequest{
					Ids:            ids,
					AddLabelIds:    []string{destLabel.Id},
					RemoveLabelIds: removeLabels,
				}).Do()
				if err != nil {
					return fmt.Errorf("failed to move batch of messages: %w", err)
				}
				totalMoved += len(ids)
				fmt.Printf("%s Moved %d messages to %q...\n", app.PrefixInfo, len(ids), destLabel.Name)
			}

			pageToken = res.NextPageToken
			if pageToken == "" {
				break
			}
		}

		fmt.Printf("%s Successfully moved %d total messages from %q to %q.\n",
			app.PrefixSuccess, totalMoved, targetLabel.Name, destLabel.Name)

		fmt.Printf("%s Deleting empty label %q...\n", app.PrefixInfo, targetLabel.Name)
		err = srv.Users.Labels.Delete("me", targetLabel.Id).Do()
		if err != nil {
			return fmt.Errorf("failed to delete migrated label %q: %w", targetLabel.Name, err)
		}
		fmt.Printf("%s Successfully deleted label %q.\n", app.PrefixSuccess, targetLabel.Name)
		return nil
	}

	fmt.Printf("%s Found label %q (ID: %s, Type: %s). Renaming to %q...\n",
		app.PrefixInfo, targetLabel.Name, targetLabel.Id, targetLabel.Type, newName)

	_, err = srv.Users.Labels.Patch("me", targetLabel.Id, &gmailapi.Label{
		Name: newName,
	}).Do()
	if err != nil {
		return fmt.Errorf("failed to rename label: %w", err)
	}

	fmt.Printf("%s Successfully renamed label %q to %q on Gmail!\n",
		app.PrefixSuccess, targetLabel.Name, newName)
	fmt.Printf("%s Note: All messages previously labeled with %q are now automatically labeled with %q.\n",
		app.PrefixInfo, targetLabel.Name, newName)

	return nil
}

// fixGmailLabelsREST checks for user labels of the form x/y where x does not exist,
// and fixes the hierarchy recursively from shallowest to deepest.
func fixGmailLabelsREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Audit started: Checking for missing parent labels on Gmail...\n", app.PrefixInfo)

	for {
		labelsRes, err := srv.Users.Labels.List("me").Do()
		if err != nil {
			return fmt.Errorf("failed to list Gmail labels: %w", err)
		}

		existing := make(map[string]bool)
		for _, l := range labelsRes.Labels {
			existing[l.Name] = true
		}

		type missingPrefixItem struct {
			Name  string
			Depth int
		}
		var missingPrefixes []missingPrefixItem
		seenMissing := make(map[string]bool)

		for _, l := range labelsRes.Labels {
			if strings.EqualFold(l.Type, "system") {
				continue
			}
			parts := strings.Split(l.Name, "/")
			if len(parts) <= 1 {
				continue
			}
			for i := 1; i < len(parts); i++ {
				prefix := strings.Join(parts[:i], "/")
				if !existing[prefix] {
					if !seenMissing[prefix] {
						seenMissing[prefix] = true
						missingPrefixes = append(missingPrefixes, missingPrefixItem{
							Name:  prefix,
							Depth: i,
						})
					}
				}
			}
		}

		if len(missingPrefixes) == 0 {
			fmt.Printf("%s No missing parent labels found. Hierarchy is completely clean!\n", app.PrefixSuccess)
			break
		}

		sort.Slice(missingPrefixes, func(i, j int) bool {
			if missingPrefixes[i].Depth != missingPrefixes[j].Depth {
				return missingPrefixes[i].Depth < missingPrefixes[j].Depth
			}
			return missingPrefixes[i].Name < missingPrefixes[j].Name
		})

		targetPrefix := missingPrefixes[0].Name
		fmt.Printf("%s Found missing parent label %q. Launching hierarchy fix...\n", app.PrefixInfo, targetPrefix)

		err = fixLabelHierarchy(srv, targetPrefix, labelsRes.Labels)
		if err != nil {
			return fmt.Errorf("failed to fix hierarchy for parent %q: %w", targetPrefix, err)
		}
	}

	return nil
}

// fixLabelHierarchy handles renaming P_d/* to temp names, creating P_d, and renaming them back.
func fixLabelHierarchy(srv *gmailapi.Service, missingParent string, allLabels []*gmailapi.Label) error {
	parts := strings.Split(missingParent, "/")
	last := parts[len(parts)-1]
	parentPath := strings.Join(parts[:len(parts)-1], "/")

	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	var tmpPrefix string
	if parentPath == "" {
		tmpPrefix = last + "_tmp_" + timestamp
	} else {
		tmpPrefix = parentPath + "/" + last + "_tmp_" + timestamp
	}

	var matchingLabels []*gmailapi.Label
	for _, l := range allLabels {
		if strings.HasPrefix(l.Name, missingParent+"/") {
			matchingLabels = append(matchingLabels, l)
		}
	}

	type RenameRestore struct {
		Id      string
		TmpName string
		OldName string
	}
	var restoreList []RenameRestore

	if len(matchingLabels) > 0 {
		sort.Slice(matchingLabels, func(i, j int) bool {
			return len(matchingLabels[i].Name) > len(matchingLabels[j].Name)
		})

		fmt.Printf("%s Temporarily renaming %d child labels under %q to %q...\n", app.PrefixInfo, len(matchingLabels), missingParent, tmpPrefix)
		for _, l := range matchingLabels {
			suffix := strings.TrimPrefix(l.Name, missingParent+"/")
			tmpName := tmpPrefix + "/" + suffix

			_, err := srv.Users.Labels.Patch("me", l.Id, &gmailapi.Label{
				Name: tmpName,
			}).Do()
			if err != nil {
				return fmt.Errorf("failed to rename %q to %q: %w", l.Name, tmpName, err)
			}
			restoreList = append(restoreList, RenameRestore{
				Id:      l.Id,
				TmpName: tmpName,
				OldName: l.Name,
			})
		}
	}

	fmt.Printf("%s Creating parent label %q...\n", app.PrefixInfo, missingParent)
	_, err := srv.Users.Labels.Create("me", &gmailapi.Label{
		Name:                  missingParent,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}).Do()
	if err != nil {
		return fmt.Errorf("failed to create label %q: %w", missingParent, err)
	}
	fmt.Printf("%s Parent label %q created successfully.\n", app.PrefixSuccess, missingParent)

	if len(restoreList) > 0 {
		sort.Slice(restoreList, func(i, j int) bool {
			return len(restoreList[i].OldName) < len(restoreList[j].OldName)
		})

		fmt.Printf("%s Restoring %d child labels back under prefix %q...\n", app.PrefixInfo, len(restoreList), missingParent)
		for _, r := range restoreList {
			_, err := srv.Users.Labels.Patch("me", r.Id, &gmailapi.Label{
				Name: r.OldName,
			}).Do()
			if err != nil {
				return fmt.Errorf("failed to restore label from %q to %q: %w", r.TmpName, r.OldName, err)
			}
		}
		fmt.Printf("%s All child labels restored under %q.\n", app.PrefixSuccess, missingParent)
	}

	return nil
}

// deleteGmailLabelREST deletes an existing custom user label on Gmail by name.
func deleteGmailLabelREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	labelName := strings.TrimSpace(config.LabelsValDel)
	if labelName == "" {
		return fmt.Errorf("label name to delete cannot be empty")
	}

	fmt.Printf("%s Searching for label %q on Gmail...\n", app.PrefixInfo, labelName)

	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	var targetLabel *gmailapi.Label
	for _, l := range labelsRes.Labels {
		if strings.EqualFold(l.Name, labelName) {
			targetLabel = l
			break
		}
	}

	if targetLabel == nil {
		return fmt.Errorf("label %q not found on Gmail", labelName)
	}

	if strings.EqualFold(targetLabel.Type, "system") {
		return fmt.Errorf("system label %q cannot be deleted", labelName)
	}

	fmt.Printf("%s Found label %q (ID: %s, Type: %s). Deleting...\n",
		app.PrefixInfo, targetLabel.Name, targetLabel.Id, targetLabel.Type)

	err = srv.Users.Labels.Delete("me", targetLabel.Id).Do()
	if err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}

	fmt.Printf("%s Successfully deleted label %q from Gmail!\n", app.PrefixSuccess, targetLabel.Name)
	return nil
}
