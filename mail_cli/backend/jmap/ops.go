package jmap

import (
	"bytes"
	"fmt"
	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	myme "mail_cli/email"
	"mime"
	"net/mail"
	"net/url"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
)

type JMAPFilterCondition struct {
	MatchFrom string `json:"matchFrom,omitempty"`
}

type JMAPFilterAction struct {
	AddMailboxIds []string `json:"addMailboxIds,omitempty"`
}

type JMAPFilter struct {
	ID         string                `json:"id,omitempty"`
	Name       string                `json:"name"`
	IsEnabled  bool                  `json:"isEnabled"`
	Conditions []JMAPFilterCondition `json:"conditions"`
	Actions    []JMAPFilterAction    `json:"actions"`
}

type JMAPFilterSetMethod struct {
	Account jmap.ID                `json:"accountId"`
	Create  map[string]*JMAPFilter `json:"create,omitempty"`
	Destroy []jmap.ID              `json:"destroy,omitempty"`
}

func (m *JMAPFilterSetMethod) Name() string         { return "Filter/set" }
func (m *JMAPFilterSetMethod) Requires() []jmap.URI { return nil }

type JMAPFilterSetResponse struct {
	AccountState string                    `json:"accountState,omitempty"`
	Created      map[string]*JMAPFilter    `json:"created,omitempty"`
	NotCreated   map[string]*jmap.SetError `json:"notCreated,omitempty"`
	NotDestroyed map[string]*jmap.SetError `json:"notDestroyed,omitempty"`
}

type JMAPFilterGetMethod struct {
	Account jmap.ID `json:"accountId"`
}

func (m *JMAPFilterGetMethod) Name() string         { return "Filter/get" }
func (m *JMAPFilterGetMethod) Requires() []jmap.URI { return nil }

type JMAPFilterGetResponse struct {
	AccountState string           `json:"accountState,omitempty"`
	List         []*JMAPFilterGet `json:"list,omitempty"`
}

type JMAPFilterGet struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsEnabled bool   `json:"isEnabled"`
}

func (c *JMAPClient) DeleteAllSpam() error {
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

	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: c.accID,
		Filter: &email.FilterCondition{
			InMailbox: spamID,
		},
		Limit: 1000,
	})

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	var queryResp *email.QueryResponse
	for _, inv := range resp.Responses {
		if qr, ok := inv.Args.(*email.QueryResponse); ok {
			queryResp = qr
		}
	}

	if queryResp == nil || len(queryResp.IDs) == 0 {
		fmt.Printf("%s JMAP Spam mailbox is already empty.\n", app.PrefixInfo)
		return nil
	}

	fmt.Printf("%s Permanently destroying %d message(s) in JMAP Spam folder...\n", app.PrefixInfo, len(queryResp.IDs))

	reqDestroy := &jmap.Request{}
	reqDestroy.Invoke(&email.Set{
		Account: c.accID,
		Destroy: queryResp.IDs,
	})

	respDestroy, err := c.client.Do(reqDestroy)
	if err != nil {
		return err
	}

	destroyedCount := 0
	for _, inv := range respDestroy.Responses {
		if r, ok := inv.Args.(*email.SetResponse); ok {
			destroyedCount = len(r.Destroyed)
			for id, errSet := range r.NotDestroyed {
				fmt.Printf("    %s Failed to destroy message %s: %s\n", app.PrefixError, id, safeStr(errSet.Description))
			}
		}
	}

	fmt.Printf("%s Successfully purged %d spam emails permanently from JMAP server.\n", app.PrefixSuccess, destroyedCount)
	return nil
}

func (c *JMAPClient) MoveAllSpam(destLabel string) error {
	if err := c.init(); err != nil {
		return err
	}

	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Spam"
	}

	if strings.EqualFold(spamFolder, destLabel) {
		fmt.Printf("%s spam_learn folder is the same as the spam folder. Purging spam emails instead...\n", app.PrefixInfo)
		return c.DeleteAllSpam()
	}

	spamID, err := c.getMailboxID(spamFolder)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: c.accID,
		Filter: &email.FilterCondition{
			InMailbox: spamID,
		},
		Limit: 1000,
	})

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	var queryResp *email.QueryResponse
	for _, inv := range resp.Responses {
		if qr, ok := inv.Args.(*email.QueryResponse); ok {
			queryResp = qr
		}
	}

	if queryResp == nil || len(queryResp.IDs) == 0 {
		fmt.Printf("%s JMAP Spam mailbox is already empty.\n", app.PrefixInfo)
		return nil
	}

	var ids []string
	for _, id := range queryResp.IDs {
		ids = append(ids, string(id))
	}

	fmt.Printf("%s Moving %d message(s) from JMAP Spam folder to '%s'...\n", app.PrefixInfo, len(ids), destLabel)
	return c.MoveEmail(ids, spamFolder, destLabel)
}

type politicalEmailResult struct {
	id              string
	fromHeader      string
	subject         string
	score           float64
	triggered       []string
	listUnsubscribe string
	isBlacklisted   bool
	unsubSuccess    bool
}

func (c *JMAPClient) ShowPoliticalSpam(autoBlacklist bool) error {
	ids, err := c.fetchSpamEmails()
	if err != nil || len(ids) == 0 {
		return err
	}

	politicalIDs := c.detectPoliticalEmails(ids)
	if len(politicalIDs) == 0 {
		return nil
	}

	printPoliticalHeader()
	idsToDelete := c.processPoliticalEmails(politicalIDs, autoBlacklist)
	c.deleteUnsubscribed(idsToDelete)
	return nil
}

func (c *JMAPClient) fetchSpamEmails() ([]string, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Spam"
	}

	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		fmt.Printf("%s JMAP Spam folder is empty.\n", app.PrefixInfo)
		return nil, nil
	}

	return ids, nil
}

func (c *JMAPClient) detectPoliticalEmails(ids []string) []string {
	dec := new(mime.WordDecoder)
	var politicalIDs []string

	for _, id := range ids {
		emailBytes, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			continue
		}

		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail != nil {
			continue
		}

		subject := myme.DecodeHeader(dec, localEmail.Header.Get("Subject"))
		bodyStr, _ := gmail.ExtractPlainBodyText(localEmail)
		bodyStr = myme.StripHTML(bodyStr)
		if len(bodyStr) > 8192 {
			bodyStr = bodyStr[:8192]
		}

		isPolitical, _, _ := myme.DetectPolitical(subject, bodyStr)
		if isPolitical {
			politicalIDs = append(politicalIDs, id)
		}
	}

	return politicalIDs
}

func printPoliticalHeader() {
	fmt.Println()
	app.ColorBoldPurple.Println("======================================================================")
	app.ColorBoldPurple.Println("           POLITICAL DONATION EMAILS DETECTED IN JMAP SPAM FOLDER     ")
	app.ColorBoldPurple.Println("======================================================================")
}

func (c *JMAPClient) processPoliticalEmails(politicalIDs []string, autoBlacklist bool) []string {
	dec := new(mime.WordDecoder)
	var idsToDelete []string

	for i, id := range politicalIDs {
		res := c.processSinglePoliticalEmail(id, autoBlacklist, dec)
		if res == nil {
			continue
		}
		printEmailResult(i+1, res, c.config.Verbose)
		if res.unsubSuccess {
			idsToDelete = append(idsToDelete, id)
		}
	}

	return idsToDelete
}

func (c *JMAPClient) processSinglePoliticalEmail(id string, autoBlacklist bool, dec *mime.WordDecoder) *politicalEmailResult {
	emailBytes, rErr := msg.Read(c.config.DownloadDir, id)
	if rErr != nil {
		return nil
	}

	localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
	if errMail != nil {
		return nil
	}

	subject := myme.DecodeHeader(dec, localEmail.Header.Get("Subject"))
	fromHeader := myme.DecodeHeader(dec, localEmail.Header.Get("From"))
	bodyStr, _ := gmail.ExtractPlainBodyText(localEmail)
	bodyStr = myme.StripHTML(bodyStr)
	if len(bodyStr) > 8192 {
		bodyStr = bodyStr[:8192]
	}

	listUnsubscribe := localEmail.Header.Get("List-Unsubscribe")
	_, score, triggered := myme.DetectPolitical(subject, bodyStr)

	sender := myme.ParseEmailAddress(fromHeader)
	isBlacklisted := c.autoBlacklistSender(sender, listUnsubscribe, score, autoBlacklist)
	unsubSuccess := c.attemptUnsubscribe(listUnsubscribe, fromHeader)

	return &politicalEmailResult{
		id:              id,
		fromHeader:      fromHeader,
		subject:         subject,
		score:           score,
		triggered:       triggered,
		listUnsubscribe: listUnsubscribe,
		isBlacklisted:   isBlacklisted,
		unsubSuccess:    unsubSuccess,
	}
}

func (c *JMAPClient) autoBlacklistSender(sender, listUnsubscribe string, score float64, autoBlacklist bool) bool {
	if !autoBlacklist || !myme.IsSafeToAutoBlacklist(sender, listUnsubscribe, score) {
		return false
	}

	if err := cfg_g.AutoBlacklistInternal(c.config, sender); err != nil {
		if c.config.Verbose {
			fmt.Printf("             %s Failed to auto-blacklist sender: %v\n", app.PrefixError, err)
		}
		return false
	}

	return true
}

func (c *JMAPClient) attemptUnsubscribe(listUnsubscribe, fromHeader string) bool {
	if listUnsubscribe == "" || !c.config.AutoUnsubscribe {
		return false
	}

	mailto, httpLink := gmail.ParseListUnsubscribe(listUnsubscribe)
	unsubSuccess := false

	if mailto != "" {
		if c.config.Verbose {
			fmt.Printf("             %s Attempting mailto unsubscribe...\n", app.PrefixInfo)
		}
		errUnsub := c.SendMail(mailto, "Unsubscribe", []byte("Please unsubscribe me."))
		gmail.LogUnsubscription(c.config, fromHeader, "mailto", mailto, errUnsub)
		if errUnsub != nil {
			if c.config.Verbose {
				fmt.Printf("             %s Mailto unsub failed: %v\n", app.PrefixError, errUnsub)
			}
		} else {
			if c.config.Verbose {
				fmt.Printf("             %s Mailto unsub sent successfully!\n", app.PrefixSuccess)
			}
			unsubSuccess = true
		}
	}

	if httpLink != "" {
		if c.config.Verbose {
			fmt.Printf("             %s Attempting HTTP unsubscribe...\n", app.PrefixInfo)
		}
		errUnsub := gmail.ExecuteHTTPUnsubscribe(httpLink)
		gmail.LogUnsubscription(c.config, fromHeader, "http", httpLink, errUnsub)
		if errUnsub != nil {
			if c.config.Verbose {
				fmt.Printf("             %s HTTP unsub failed: %v\n", app.PrefixError, errUnsub)
			}
		} else {
			if c.config.Verbose {
				fmt.Printf("             %s HTTP unsub requested successfully!\n", app.PrefixSuccess)
			}
			unsubSuccess = true
		}
	}

	return unsubSuccess
}

func printEmailResult(polCount int, res *politicalEmailResult, verbose bool) {
	if verbose {
		fmt.Printf("[%d] ID: ", polCount)
		app.ColorBold.Println(res.id)
		fmt.Printf("    From:    ")
		app.ColorCyan.Println(res.fromHeader)
		fmt.Printf("    Subject: ")
		app.ColorBold.Println(res.subject)

		scorePrinter := app.ColorYellow
		if res.score >= 20.0 {
			scorePrinter = app.ColorRed
		}
		fmt.Printf("    Score:   ")
		scorePrinter.Printf("%.1f", res.score)
		fmt.Println("/10.0")

		fmt.Printf("    Keys:    ")
		app.ColorYellow.Println(strings.Join(res.triggered, ", "))

		if res.listUnsubscribe != "" {
			fmt.Printf("    Unsub:   Found header: %s\n", res.listUnsubscribe)
			if res.unsubSuccess {
				fmt.Printf("             %s Marked email for deletion from JMAP Spam folder.\n", app.PrefixSuccess)
			}
		}
		fmt.Println("----------------------------------------------------------------------")
	} else {
		var actions []string
		if res.unsubSuccess {
			actions = append(actions, "Unsubscribed")
		}
		if res.isBlacklisted {
			actions = append(actions, "Blacklisted")
		}
		actionStr := ""
		if len(actions) > 0 {
			actionStr = " [" + strings.Join(actions, " & ") + "]"
		}
		scorePrinter := app.ColorYellow
		if res.score >= 20.0 {
			scorePrinter = app.ColorRed
		}
		scoreStr := scorePrinter.Sprintf("%.1f", res.score)
		fmt.Printf("  - %d. [%s/10.0] From: %s | Subj: %s%s\n", polCount, scoreStr, res.fromHeader, res.subject, actionStr)
	}
}

func (c *JMAPClient) deleteUnsubscribed(idsToDelete []string) {
	if len(idsToDelete) == 0 {
		return
	}

	fmt.Printf("\n%s Automatically deleting %d unsubscribed political spam email(s) from JMAP server...\n", app.PrefixInfo, len(idsToDelete))
	reqDestroy := &jmap.Request{}
	var destroyIDs []jmap.ID
	for _, id := range idsToDelete {
		destroyIDs = append(destroyIDs, jmap.ID(id))
	}
	reqDestroy.Invoke(&email.Set{
		Account: c.accID,
		Destroy: destroyIDs,
	})
	_, _ = c.client.Do(reqDestroy)
}

func (c *JMAPClient) SendMail(to string, subject string, body []byte) error {
	if err := c.init(); err != nil {
		return err
	}

	toEmail := to
	mailSubject := subject
	mailBody := body

	if strings.HasPrefix(strings.ToLower(to), "mailto:") {
		u, err := url.Parse(to)
		if err == nil {
			toEmail = u.Path
			if toEmail == "" {
				toEmail = u.Opaque
			}
			queryParams := u.Query()
			if s := queryParams.Get("subject"); s != "" {
				mailSubject = s
			}
			if b := queryParams.Get("body"); b != "" {
				mailBody = []byte(b)
			}
		}
	}

	draftsID, err := c.getMailboxID("Drafts")
	if err != nil {
		draftsID, err = c.getMailboxID(c.InboxFolder())
		if err != nil {
			return fmt.Errorf("cannot find drafts or inbox mailbox: %w", err)
		}
	}

	reqEmail := &jmap.Request{}
	reqEmail.Invoke(&email.Set{
		Account: c.accID,
		Create: map[jmap.ID]*email.Email{
			"draft1": {
				Subject:    mailSubject,
				MailboxIDs: map[jmap.ID]bool{draftsID: true},
				BodyValues: map[string]*email.BodyValue{
					"body1": {Value: string(mailBody)},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body1", Type: "text/plain"},
				},
			},
		},
	})

	respEmail, err := c.client.Do(reqEmail)
	if err != nil {
		return err
	}

	var createdEmailID jmap.ID
	for _, inv := range respEmail.Responses {
		if r, ok := inv.Args.(*email.SetResponse); ok {
			if errSet, failed := r.NotCreated["draft1"]; failed {
				return fmt.Errorf("failed to create JMAP draft email: %s", safeStr(errSet.Description))
			}
			for _, em := range r.Created {
				createdEmailID = em.ID
			}
		}
	}

	if createdEmailID == "" {
		return fmt.Errorf("failed to retrieve created draft email ID")
	}

	identID, err := c.getIdentityID()
	if err != nil {
		return err
	}

	reqSub := &jmap.Request{}
	reqSub.Invoke(&emailsubmission.Set{
		Account: c.accID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"sub1": {
				IdentityID: identID,
				EmailID:    createdEmailID,
				Envelope: &emailsubmission.Envelope{
					MailFrom: &emailsubmission.Address{Email: c.account.Username},
					RcptTo:   []*emailsubmission.Address{{Email: toEmail}},
				},
			},
		},
	})

	respSub, err := c.client.Do(reqSub)
	if err != nil {
		return err
	}

	for _, inv := range respSub.Responses {
		if r, ok := inv.Args.(*emailsubmission.SetResponse); ok {
			if errSet, failed := r.NotCreated["sub1"]; failed {
				return fmt.Errorf("JMAP EmailSubmission failed: %s", safeStr(errSet.Description))
			}
		}
	}

	return nil
}
