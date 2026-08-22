package jmap

import (
	"bytes"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	myme "mail_cli/email"
	"mime"
	"net/mail"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
)

// forEachNotUpdated iterates resp.Responses for *email.SetResponse and calls fn
// for each entry in NotUpdated.
func forEachNotUpdated(resp *jmap.Response, fn func(id jmap.ID, errSet *jmap.SetError)) {
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.SetResponse); ok {
			for id, errSet := range r.NotUpdated {
				fn(id, errSet)
			}
		}
	}
}

type candidateInfo struct {
	rule    cfg_acc.Rule
	sender  string
	subject string
}

func (c *JMAPClient) CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error) {
	if len(messageIDs) == 0 {
		return messageIDs, nil
	}
	if err := c.init(); err != nil {
		return nil, err
	}

	candidates, matchedIDs, remainingIDs := c.loadCandidates(messageIDs)
	if len(matchedIDs) == 0 {
		return remainingIDs, nil
	}

	currentMailboxIDs, err := c.fetchCurrentMailboxIDs(toJmapIDs(matchedIDs))
	if err != nil {
		remainingIDs = append(remainingIDs, matchedIDs...)
		return remainingIDs, err
	}

	return c.applyRuleMoves(matchedIDs, candidates, currentMailboxIDs, remainingIDs), nil
}

func (c *JMAPClient) loadCandidates(messageIDs []string) (map[string]candidateInfo, []string, []string) {
	dec := new(mime.WordDecoder)
	candidates := make(map[string]candidateInfo)
	var matchedIDs, remainingIDs []string

	for _, id := range messageIDs {
		data, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			remainingIDs = append(remainingIDs, id)
			continue
		}
		parsed, errParse := mail.ReadMessage(bytes.NewReader(data))
		if errParse != nil {
			remainingIDs = append(remainingIDs, id)
			continue
		}
		fromHeader := parsed.Header.Get("From")
		sender := myme.ParseEmailAddress(fromHeader)
		subject := myme.DecodeHeader(dec, parsed.Header.Get("Subject"))

		if matchedRule := cfg_acc.MatchRules(c.config.Rules, sender, subject); matchedRule != nil {
			candidates[id] = candidateInfo{rule: *matchedRule, sender: sender, subject: subject}
			matchedIDs = append(matchedIDs, id)
		} else {
			remainingIDs = append(remainingIDs, id)
		}
	}
	return candidates, matchedIDs, remainingIDs
}

func toJmapIDs(ids []string) []jmap.ID {
	out := make([]jmap.ID, len(ids))
	for i, id := range ids {
		out[i] = jmap.ID(id)
	}
	return out
}

func (c *JMAPClient) fetchCurrentMailboxIDs(jmapIDs []jmap.ID) (map[string]map[string]bool, error) {
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:    c.accID,
		IDs:        jmapIDs,
		Properties: []string{"mailboxIds"},
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	var getResp *email.GetResponse
	for _, inv := range resp.Responses {
		if gr, ok := inv.Args.(*email.GetResponse); ok {
			getResp = gr
		}
	}

	out := make(map[string]map[string]bool)
	if getResp == nil {
		return out, nil
	}
	for _, em := range getResp.List {
		mbs := make(map[string]bool, len(em.MailboxIDs))
		for mbID := range em.MailboxIDs {
			mbs[string(mbID)] = true
		}
		out[string(em.ID)] = mbs
	}
	return out, nil
}

func (c *JMAPClient) applyRuleMoves(matchedIDs []string, candidates map[string]candidateInfo, currentMailboxIDs map[string]map[string]bool, remainingIDs []string) []string {
	for _, id := range matchedIDs {
		remainingIDs = c.applySingleRuleMove(id, candidates[id], currentMailboxIDs[id], remainingIDs)
	}
	return remainingIDs
}

func (c *JMAPClient) applySingleRuleMove(id string, candidate candidateInfo, mbs map[string]bool, remainingIDs []string) []string {
	targetID, err := c.getOrCreateMailbox(candidate.rule.Label)
	if err != nil {
		if c.config.Verbose {
			fmt.Printf("    %s JMAP Rule: Failed to resolve/create mailbox %q: %v\n", app.PrefixWarn, candidate.rule.Label, err)
		}
		return append(remainingIDs, id)
	}

	if len(mbs) == 1 && mbs[string(targetID)] {
		if c.config.Verbose {
			fmt.Printf("%s JMAP Rule match has 0 effect (already exclusively in %q) for message %s. Keeping message as-is.\n", app.PrefixInfo, candidate.rule.Label, id)
		}
		return append(remainingIDs, id)
	}

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.accID,
		Update: map[jmap.ID]jmap.Patch{
			jmap.ID(id): {"mailboxIds": map[string]bool{string(targetID): true}},
		},
	})
	resp, err := c.client.Do(req)
	if err != nil {
		if c.config.Verbose {
			fmt.Printf("    %s JMAP Rule: Move failed for message %s: %v\n", app.PrefixWarn, id, err)
		}
		return append(remainingIDs, id)
	}

	isSuccess := true
	forEachNotUpdated(resp, func(failedID jmap.ID, errSet *jmap.SetError) {
		if failedID == jmap.ID(id) {
			isSuccess = false
			if c.config.Verbose {
				fmt.Printf("    %s JMAP Rule: Move failed: %s\n", app.PrefixWarn, safeStr(errSet.Description))
			}
		}
	})

	if isSuccess {
		ruleDesc := candidate.rule.Sender
		if candidate.rule.Subject != "" {
			ruleDesc = fmt.Sprintf("subject: %s*", candidate.rule.Subject)
		}
		fmt.Printf("%s JMAP Rule Applied: Message %s (%q) matching '%s' moved to %q\n", app.PrefixSuccess, id, candidate.subject, ruleDesc, candidate.rule.Label)
		return remainingIDs
	}
	return append(remainingIDs, id)
}

// setMailboxOperation is the shared helper for MoveEmail and CopyEmail.
// buildPatch receives the resolved destID and returns the patch to apply.
func (c *JMAPClient) setMailboxOperation(messageIDs []string, destLabelName string, buildPatch func(jmap.ID) jmap.Patch) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if err := c.init(); err != nil {
		return err
	}

	destID, err := c.getOrCreateMailbox(destLabelName)
	if err != nil {
		return err
	}

	updates := make(map[jmap.ID]jmap.Patch, len(messageIDs))
	patch := buildPatch(destID)
	for _, id := range messageIDs {
		updates[jmap.ID(id)] = patch
	}

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.accID,
		Update:  updates,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	var opErr error
	forEachNotUpdated(resp, func(id jmap.ID, errSet *jmap.SetError) {
		msg := fmt.Sprintf("failed to move message %s to JMAP mailbox %s: %s (type: %s)", id, destLabelName, safeStr(errSet.Description), errSet.Type)
		fmt.Printf("    %s %s\n", app.PrefixError, msg)
		if opErr == nil {
			opErr = fmt.Errorf("%s", msg)
		}
	})
	return opErr
}

func (c *JMAPClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if strings.EqualFold(sourceLabelName, destLabelName) {
		if c.config.Verbose {
			fmt.Printf("    [Debug] MoveEmail: source and destination are the same (%q). Skipping.\n", destLabelName)
		}
		return nil
	}
	return c.setMailboxOperation(messageIDs, destLabelName, func(destID jmap.ID) jmap.Patch {
		return jmap.Patch{"mailboxIds": map[string]bool{string(destID): true}}
	})
}

func (c *JMAPClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return c.setMailboxOperation(messageIDs, destLabelName, func(destID jmap.ID) jmap.Patch {
		return jmap.Patch{"mailboxIds/" + string(destID): true}
	})
}

func (c *JMAPClient) ReportSpam(messageIDs []string, sourceLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if err := c.init(); err != nil {
		return err
	}

	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Spam"
	}

	spamID, err := c.getMailboxID(spamFolder)
	if err != nil {
		return err
	}

	updates := make(map[jmap.ID]jmap.Patch, len(messageIDs))
	for _, id := range messageIDs {
		updates[jmap.ID(id)] = jmap.Patch{
			"mailboxIds": map[string]bool{string(spamID): true},
		}
	}

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.accID,
		Update:  updates,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	forEachNotUpdated(resp, func(id jmap.ID, errSet *jmap.SetError) {
		fmt.Printf("    %s Failed to move message %s to JMAP Spam: %s (type: %s)\n", app.PrefixError, id, safeStr(errSet.Description), errSet.Type)
	})
	return nil
}

func (c *JMAPClient) MoveToInbox(messageIDs []string, sourceLabelName string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	inboxName := c.InboxFolder()
	if strings.EqualFold(sourceLabelName, inboxName) {
		if c.config.Verbose {
			fmt.Printf("    [Debug] MoveToInbox: message already in Inbox. Skipping.\n")
		}
		return nil
	}
	if err := c.init(); err != nil {
		return err
	}

	inboxID, err := c.getMailboxID(inboxName)
	if err != nil {
		return err
	}

	unspamID, errUnspam := c.resolveUnspamMailbox()
	if errUnspam != nil {
		fmt.Printf("    %s Warning: failed to get or create JMAP mailbox %q: %v\n", app.PrefixWarn, c.config.UnspamLearn, errUnspam)
	}

	updates := make(map[jmap.ID]jmap.Patch, len(messageIDs))
	for _, id := range messageIDs {
		mailboxIds := map[string]bool{string(inboxID): true}
		if unspamID != "" {
			mailboxIds[string(unspamID)] = true
		}
		updates[jmap.ID(id)] = jmap.Patch{"mailboxIds": mailboxIds}
	}

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.accID,
		Update:  updates,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	forEachNotUpdated(resp, func(id jmap.ID, errSet *jmap.SetError) {
		fmt.Printf("    %s Failed to move message %s to JMAP Inbox: %s (type: %s)\n", app.PrefixError, id, safeStr(errSet.Description), errSet.Type)
	})
	return nil
}

func (c *JMAPClient) resolveUnspamMailbox() (jmap.ID, error) {
	if c.config.UnspamLearn == "" {
		return "", nil
	}
	return c.getOrCreateMailbox(c.config.UnspamLearn)
}

func (c *JMAPClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	if err := c.init(); err != nil {
		return err
	}
	if err := c.EnsureLabelExists(targetLabel); err != nil {
		return err
	}

	destMailboxID, err := c.getMailboxID(targetLabel)
	if err != nil {
		return err
	}

	uploadResp, err := c.client.Upload(c.accID, bytes.NewReader(rawBytes))
	if err != nil {
		return fmt.Errorf("failed to upload email blob to JMAP: %w", err)
	}

	req := &jmap.Request{}
	req.Invoke(&email.Import{
		Account: c.accID,
		Emails: map[string]*email.EmailImport{
			"import1": {
				BlobID:     uploadResp.ID,
				MailboxIDs: map[jmap.ID]bool{destMailboxID: true},
			},
		},
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed JMAP import request: %w", err)
	}

	var importErr error
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.ImportResponse); ok {
			if len(r.NotCreated) > 0 {
				for id, jErr := range r.NotCreated {
					if jErr.Type == "alreadyExists" {
						continue
					}
					desc := ""
					if jErr.Description != nil {
						desc = *jErr.Description
					}
					importErr = fmt.Errorf("failed to import email %s: %s (%s)", id, jErr.Type, desc)
					break
				}
			}
		}
	}
	return importErr
}

func (c *JMAPClient) resolveFallbackMailbox() jmap.ID {
	for _, name := range []string{"inbox", "inbox folder"} {
		if id, exists := c.mailboxId[name]; exists {
			return id
		}
	}
	for _, id := range c.mailboxId {
		return id
	}
	return ""
}

func (c *JMAPClient) SendEmail(rawBytes []byte) error {
	if err := c.init(); err != nil {
		return err
	}

	createdEmailID, err := c.uploadAndImportEmail(rawBytes)
	if err != nil {
		return err
	}

	identityID, err := c.getIdentityID()
	if err != nil {
		return err
	}

	rcptTo, err := c.parseRecipients(rawBytes)
	if err != nil {
		return err
	}

	return c.submitEmail(createdEmailID, identityID, rcptTo)
}

func (c *JMAPClient) uploadAndImportEmail(rawBytes []byte) (jmap.ID, error) {
	uploadResp, err := c.client.Upload(c.accID, bytes.NewReader(rawBytes))
	if err != nil {
		return "", fmt.Errorf("failed JMAP upload: %w", err)
	}

	targetMailboxID := c.sentMailboxID
	if targetMailboxID == "" {
		targetMailboxID = c.resolveFallbackMailbox()
	}
	if targetMailboxID == "" {
		return "", fmt.Errorf("failed to submit email: no mailboxes available on JMAP server")
	}

	req := &jmap.Request{}
	req.Invoke(&email.Import{
		Account: c.accID,
		Emails: map[string]*email.EmailImport{
			"import1": {
				BlobID:     uploadResp.ID,
				MailboxIDs: map[jmap.ID]bool{targetMailboxID: true},
			},
		},
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed JMAP import request: %w", err)
	}

	var createdEmailID jmap.ID
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.ImportResponse); ok {
			if created, ok := r.Created["import1"]; ok {
				createdEmailID = created.ID
			}
			if len(r.NotCreated) > 0 {
				for id, jErr := range r.NotCreated {
					desc := ""
					if jErr.Description != nil {
						desc = *jErr.Description
					}
					return "", fmt.Errorf("failed to import email %s: %s (%s)", id, jErr.Type, desc)
				}
			}
		}
	}
	if createdEmailID == "" {
		return "", fmt.Errorf("failed to retrieve created email ID from JMAP import")
	}
	return createdEmailID, nil
}

func (c *JMAPClient) parseRecipients(rawBytes []byte) ([]*emailsubmission.Address, error) {
	parsedMsg, err := mail.ReadMessage(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email headers for submission: %w", err)
	}

	var rcptTo []*emailsubmission.Address
	seen := make(map[string]bool)
	for _, hdrName := range []string{"To", "Cc"} {
		hdrVal := parsedMsg.Header.Get(hdrName)
		if hdrVal == "" {
			continue
		}
		addrs, err := mail.ParseAddressList(hdrVal)
		if err == nil {
			for _, addr := range addrs {
				addrLower := strings.ToLower(addr.Address)
				if !seen[addrLower] {
					seen[addrLower] = true
					rcptTo = append(rcptTo, &emailsubmission.Address{Email: addr.Address})
				}
			}
		}
	}
	return rcptTo, nil
}

func (c *JMAPClient) submitEmail(createdEmailID, identityID jmap.ID, rcptTo []*emailsubmission.Address) error {
	req := &jmap.Request{}
	req.Invoke(&emailsubmission.Set{
		Account: c.accID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"sub1": {
				EmailID:    createdEmailID,
				IdentityID: identityID,
				Envelope: &emailsubmission.Envelope{
					MailFrom: &emailsubmission.Address{Email: c.account.Username},
					RcptTo:   rcptTo,
				},
			},
		},
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed JMAP email submission: %w", err)
	}

	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*emailsubmission.SetResponse); ok {
			if len(r.NotCreated) > 0 {
				for id, jErr := range r.NotCreated {
					desc := ""
					if jErr.Description != nil {
						desc = *jErr.Description
					}
					return fmt.Errorf("failed to submit email %s: %s (%s)", id, jErr.Type, desc)
				}
			}
		}
	}
	return nil
}
