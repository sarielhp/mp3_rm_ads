package jmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/uicommon"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
)

func (c *JMAPClient) ListLabels() error {
	if err := c.init(); err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.accID,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	var mbs []*mailbox.Mailbox
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			mbs = r.List
		}
	}

	mbMap := make(map[jmap.ID]*mailbox.Mailbox)
	for _, mb := range mbs {
		mbMap[mb.ID] = mb
	}

	var getFullPath func(mb *mailbox.Mailbox) string
	getFullPath = func(mb *mailbox.Mailbox) string {
		if mb.ParentID == "" {
			return mb.Name
		}
		parent, exists := mbMap[mb.ParentID]
		if !exists {
			return mb.Name
		}
		return getFullPath(parent) + "/" + mb.Name
	}

	var folderItems []cfg_acc.LabelItem
	for _, mb := range mbs {
		folderItems = append(folderItems, cfg_acc.LabelItem{
			Name:           mb.Name,
			FullName:       getFullPath(mb),
			MessagesTotal:  int64(mb.TotalEmails),
			MessagesUnread: int64(mb.UnreadEmails),
			IsLabel:        true,
		})
	}

	uicommon.PrintLabelTree("                          JMAP MAILBOXES LIST                         ", folderItems, c.config.HideZeroLabels)
	return nil
}

func (c *JMAPClient) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.accID,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	var mbs []*mailbox.Mailbox
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			mbs = r.List
		}
	}

	mbMap := make(map[jmap.ID]*mailbox.Mailbox)
	for _, mb := range mbs {
		mbMap[mb.ID] = mb
	}

	var getFullPath func(mb *mailbox.Mailbox) string
	getFullPath = func(mb *mailbox.Mailbox) string {
		if mb.ParentID == "" {
			return mb.Name
		}
		parent, exists := mbMap[mb.ParentID]
		if !exists {
			return mb.Name
		}
		return getFullPath(parent) + "/" + mb.Name
	}

	var items []cfg_acc.LabelItem
	for _, mb := range mbs {
		items = append(items, cfg_acc.LabelItem{
			Name:           mb.Name,
			FullName:       getFullPath(mb),
			MessagesTotal:  int64(mb.TotalEmails),
			MessagesUnread: int64(mb.UnreadEmails),
			IsLabel:        true,
		})
	}

	return items, nil
}

func (c *JMAPClient) RenameLabel(oldName, newName string) error {
	if err := c.init(); err != nil {
		return err
	}

	mbID, err := c.getMailboxID(oldName)
	if err != nil {
		return err
	}

	parts := strings.Split(newName, "/")
	localName := parts[len(parts)-1]

	var parentID jmap.ID
	if len(parts) > 1 {
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		parentID, err = c.getOrCreateMailbox(parentPath)
		if err != nil {
			return err
		}
	}

	req := &jmap.Request{}
	patch := jmap.Patch{
		"name": localName,
	}
	if len(parts) > 1 {
		patch["parentId"] = string(parentID)
	} else {
		patch["parentId"] = nil
	}

	req.Invoke(&mailbox.Set{
		Account: c.accID,
		Update: map[jmap.ID]jmap.Patch{
			mbID: patch,
		},
	})

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			if errSet, failed := r.NotUpdated[mbID]; failed {
				return fmt.Errorf("failed to rename JMAP mailbox: %s", safeStr(errSet.Description))
			}
		}
	}

	fmt.Printf("%s Successfully renamed JMAP mailbox %q to %q.\n", app.PrefixSuccess, oldName, newName)
	return c.loadMailboxes()
}

func (c *JMAPClient) FixLabels() error {
	fmt.Printf("%s JMAP mailboxes hierarchy is managed automatically by the JMAP server.\n", app.PrefixInfo)
	return nil
}

func (c *JMAPClient) DeleteLabel(name string) error {
	if err := c.init(); err != nil {
		return err
	}

	mbID, err := c.getMailboxID(name)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: c.accID,
		Destroy: []jmap.ID{mbID},
	})

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			if errSet, failed := r.NotDestroyed[mbID]; failed {
				return fmt.Errorf("failed to delete JMAP mailbox: %s", safeStr(errSet.Description))
			}
		}
	}

	fmt.Printf("%s Successfully deleted JMAP mailbox %q.\n", app.PrefixSuccess, name)
	return c.loadMailboxes()
}

func (c *JMAPClient) ExportRules() error {
	if err := c.init(); err != nil {
		return err
	}

	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(c.config)
	if err != nil {
		return err
	}

	if len(targetAcc.Rules) == 0 {
		fmt.Printf("%s No rules found to export for account %s.\n", app.PrefixWarn, targetAcc.Name)
		return nil
	}

	var rulesToExport []cfg_acc.Rule
	if c.config.ForceLearn {
		fmt.Printf("%s Force mode: will export all rules (server-side filters not supported by this JMAP server).\n", app.PrefixInfo)
		for _, rule := range targetAcc.Rules {
			if !rule.Internal {
				rulesToExport = append(rulesToExport, rule)
			}
		}
	} else {
		for _, rule := range targetAcc.Rules {
			if rule.Internal {
				continue
			}
			if !rule.Exported {
				rulesToExport = append(rulesToExport, rule)
			}
		}
		if len(rulesToExport) == 0 {
			fmt.Printf("%s All rules for account %s are already exported.\n", app.PrefixInfo, targetAcc.Name)
			return nil
		}
		fmt.Printf("%s Exporting %d unexported rule(s)...\n", app.PrefixInfo, len(rulesToExport))
	}

	fmt.Printf("%s Note: Server-side filter creation is not supported by this JMAP server.\n", app.PrefixWarn)
	fmt.Printf("%s Rules will be marked as exported locally in config.\n\n", app.PrefixWarn)
	for _, r := range rulesToExport {
		fmt.Printf("  - %s -> %s\n", r.Sender, r.Label)
	}

	for i := range targetAcc.Rules {
		if !targetAcc.Rules[i].Internal {
			targetAcc.Rules[i].Exported = true
		}
	}

	if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
		return err
	}

	for i := range c.config.Rules {
		if !c.config.Rules[i].Internal {
			c.config.Rules[i].Exported = true
		}
	}

	fmt.Printf("%s Marked %d rule(s) as exported locally for account %s.\n", app.PrefixSuccess, len(rulesToExport), targetAcc.Name)
	return nil
}

func (c *JMAPClient) ListFilters() error {
	_, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(c.config)
	if err != nil {
		return err
	}

	fmt.Println("======================================================================")
	fmt.Printf("                SERVER FILTERS FOR ACCOUNT: %s\n", strings.ToUpper(targetAcc.Name))
	fmt.Println("======================================================================")
	fmt.Println("  Note: JMAP servers (including FastMail) do not support server-side")
	fmt.Println("  mail filters. The following are local routing rules.")
	fmt.Println()

	if len(targetAcc.Rules) == 0 {
		fmt.Println("  No rules configured.")
	} else {
		for i, rule := range targetAcc.Rules {
			status := " [unexported]"
			if rule.Exported {
				status = " [exported]"
			}
			fmt.Printf("  [%d] %s%s\n", i+1, rule.Sender, status)
			fmt.Printf("      -> %s\n", rule.Label)
		}
	}

	fmt.Println("======================================================================")
	return nil
}

func (c *JMAPClient) LearnSpam() error {
	if err := c.init(); err != nil {
		return err
	}

	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Spam"
	}

	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		fmt.Printf("%s JMAP Spam folder is empty.\n", app.PrefixInfo)
		return nil
	}

	total := len(ids)

	if err := app.RunPreFlightCheck(); err != nil {
		return err
	}

	trainedUidsPath := filepath.Join(c.config.DownloadDir, "trained_uids.json")
	trainedUids := make(map[string]bool)
	if !c.config.ForceLearn {
		if data, errRead := os.ReadFile(trainedUidsPath); errRead == nil {
			_ = json.Unmarshal(data, &trainedUids)
		}
	}

	successCount := 0
	alreadyLearnedCount := 0
	ignoredCount := 0

	for i, id := range ids {
		if trainedUids[id] {
			alreadyLearnedCount++
			continue
		}

		data, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			ignoredCount++
			continue
		}
		emailBytes := data

		cmd := exec.Command("bogofilter", "-s")
		cmd.Stdin = bytes.NewReader(emailBytes)
		errRun := cmd.Run()
		if errRun == nil {
			successCount++
			trainedUids[id] = true
			_ = msg.ClearClassification(c.config.DownloadDir, id)
		} else {
			if c.config.Verbose {
				fmt.Printf("    %s Bogofilter training failed for ID %s: %v\n", app.PrefixWarn, id, errRun)
			}
		}

		uicommon.DrawProgressBar(i+1, total, app.PrefixInfo+" Training classifier...")
	}

	if tBytes, errMarshal := json.Marshal(trainedUids); errMarshal == nil {
		_ = os.WriteFile(trainedUidsPath, tBytes, 0600)
	}

	fmt.Printf("%s Successfully trained Bogofilter on ", app.PrefixSuccess)
	app.ColorBoldGreen.Printf("%d", successCount)
	fmt.Printf(" new spam message(s). ")
	app.ColorBoldYellow.Printf("%d", alreadyLearnedCount)
	fmt.Printf(" already trained. ")
	app.ColorBoldRed.Printf("%d", ignoredCount)
	fmt.Println(" ignored.")
	return nil
}
