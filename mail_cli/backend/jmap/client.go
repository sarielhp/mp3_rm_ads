package jmap

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	jmapMail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

type JMAPClient struct {
	config        *cfg_g.Config
	account       cfg_acc.AccountConfig
	client        *jmap.Client
	accID         jmap.ID
	mailboxId     map[string]jmap.ID
	sentMailboxID jmap.ID
}

func NewJMAPClient(acc cfg_acc.AccountConfig, config *cfg_g.Config) mailclient.MailClient {
	return &JMAPClient{
		config:  config,
		account: acc,
	}
}

func (c *JMAPClient) init() error {
	if c.client != nil {
		return nil
	}

	sessionURL := c.account.SessionURL
	if sessionURL == "" {
		sessionURL = c.account.IMAPHost
	}
	if sessionURL == "" {
		return fmt.Errorf("no JMAP session URL is configured for account %s", c.account.Name)
	}

	client := &jmap.Client{
		SessionEndpoint: sessionURL,
	}
	client.WithAccessToken(c.account.Password)
	if err := client.Authenticate(); err != nil {
		return fmt.Errorf("JMAP authentication failed for %s: %w", c.account.Username, err)
	}

	c.client = client
	c.accID = client.Session.PrimaryAccounts[jmapMail.URI]

	if err := c.loadMailboxes(); err != nil {
		return err
	}

	return nil
}

func (c *JMAPClient) loadMailboxes() error {
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.accID,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to retrieve mailboxes: %w", err)
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

	c.mailboxId = make(map[string]jmap.ID)
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

	c.sentMailboxID = ""
	for _, mb := range mbs {
		fullPath := getFullPath(mb)
		c.mailboxId[strings.ToLower(fullPath)] = mb.ID
		c.mailboxId[fullPath] = mb.ID

		if strings.EqualFold(string(mb.Role), "sent") {
			c.sentMailboxID = mb.ID
		}
	}

	if c.sentMailboxID == "" {
		for _, mb := range mbs {
			nameLower := strings.ToLower(mb.Name)
			if nameLower == "sent" || nameLower == "sent items" || nameLower == "sent messages" {
				c.sentMailboxID = mb.ID
				break
			}
		}
	}

	return nil
}

func (c *JMAPClient) getMailboxID(name string) (jmap.ID, error) {
	if err := c.init(); err != nil {
		return "", err
	}

	lowerName := strings.ToLower(name)
	priorityKeys := c.resolvePriorityKeys(lowerName)

	for _, k := range priorityKeys {
		if id, ok := c.mailboxId[k]; ok {
			slog.Info("getMailboxID resolved", slog.String("lookupName", name), slog.String("matchedKey", k), slog.String("returnedID", string(id)))

			var collisions []string
			for mapKey := range c.mailboxId {
				if mapKey != k && (mapKey == lowerName || strings.HasSuffix(mapKey, "/"+lowerName)) {
					collisions = append(collisions, fmt.Sprintf("%q", mapKey))
				}
			}
			if len(collisions) > 0 {
				slog.Warn("getMailboxID key collision detected (selected key may be wrong)",
					slog.String("selectedKey", k),
					slog.Any("otherKeysMatched", collisions))
			}

			return id, nil
		}
	}

	if err := c.loadMailboxes(); err == nil {
		priorityKeys = c.resolvePriorityKeys(lowerName)
		for _, k := range priorityKeys {
			if id, ok := c.mailboxId[k]; ok {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("JMAP mailbox not found: %s", name)
}

func (c *JMAPClient) resolvePriorityKeys(lowerName string) []string {
	inboxName := c.InboxFolder()
	if strings.ToLower(lowerName) == "inbox" || strings.EqualFold(lowerName, inboxName) {
		for _, exactKey := range []string{inboxName, "inbox", "Inbox", "INBOX"} {
			if _, ok := c.mailboxId[exactKey]; ok {
				return []string{exactKey}
			}
		}
		var inboxKeys []string
		for k := range c.mailboxId {
			if k == strings.ToLower(inboxName) || k == "inbox" || strings.HasPrefix(k, strings.ToLower(inboxName)+"/") || strings.HasPrefix(k, "inbox/") {
				inboxKeys = append(inboxKeys, k)
			}
		}
		sort.Strings(inboxKeys)
		return inboxKeys
	}

	switch strings.ToLower(lowerName) {
	case "spam":
		for _, exactKey := range []string{"spam", "Spam", "SPAM"} {
			if _, ok := c.mailboxId[exactKey]; ok {
				return []string{exactKey}
			}
		}
		var spamKeys []string
		for k := range c.mailboxId {
			if k == lowerName || strings.HasPrefix(k, "spam/") {
				spamKeys = append(spamKeys, k)
			}
		}
		sort.Strings(spamKeys)
		return spamKeys
	default:
		return []string{lowerName}
	}
}

func (c *JMAPClient) getOrCreateMailbox(name string) (jmap.ID, error) {
	id, err := c.getMailboxID(name)
	if err == nil {
		return id, nil
	}

	parts := strings.Split(name, "/")
	var parentID jmap.ID

	for i := 0; i < len(parts); i++ {
		currentPath := strings.Join(parts[:i+1], "/")
		id, err = c.getMailboxID(currentPath)
		if err != nil {
			localName := parts[i]
			reqCreate := &jmap.Request{}
			mbCreate := &mailbox.Mailbox{
				Name: localName,
			}
			if parentID != "" {
				mbCreate.ParentID = parentID
			}

			reqCreate.Invoke(&mailbox.Set{
				Account: c.accID,
				Create: map[jmap.ID]*mailbox.Mailbox{
					"new-mb": mbCreate,
				},
			})

			respCreate, errDo := c.client.Do(reqCreate)
			if errDo != nil {
				return "", errDo
			}

			var createdID jmap.ID
			for _, inv := range respCreate.Responses {
				if r, ok := inv.Args.(*mailbox.SetResponse); ok {
					if errSet, failed := r.NotCreated["new-mb"]; failed {
						return "", fmt.Errorf("failed to create mailbox %s: %s", currentPath, safeStr(errSet.Description))
					}
					for _, mb := range r.Created {
						createdID = mb.ID
					}
				}
			}
			if createdID == "" {
				return "", fmt.Errorf("failed to retrieve ID of created mailbox: %s", currentPath)
			}

			id = createdID
			if errRefresh := c.loadMailboxes(); errRefresh != nil {
				return "", errRefresh
			}
		}
		parentID = id
	}

	return parentID, nil
}

func (c *JMAPClient) EnsureLabelExists(name string) error {
	_, err := c.getOrCreateMailbox(name)
	return err
}

func (c *JMAPClient) MarkAsRead(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if err := c.init(); err != nil {
		return err
	}
	updates := make(map[jmap.ID]jmap.Patch)
	for _, id := range messageIDs {
		updates[jmap.ID(id)] = jmap.Patch{"isUnread": false}
	}
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.accID,
		Update:  updates,
	})
	_, err := c.client.Do(req)
	return err
}

func (c *JMAPClient) downloadBlob(accountId, blobId, name, contentType string) ([]byte, error) {
	urlStr := c.client.Session.DownloadURL
	urlStr = strings.ReplaceAll(urlStr, "{accountId}", url.PathEscape(accountId))
	urlStr = strings.ReplaceAll(urlStr, "{blobId}", url.PathEscape(blobId))
	urlStr = strings.ReplaceAll(urlStr, "{name}", url.PathEscape(name))
	urlStr = strings.ReplaceAll(urlStr, "{type}", url.QueryEscape(contentType))

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (c *JMAPClient) getIdentityID() (jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&identity.Get{
		Account: c.accID,
	})
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}

	var identityID jmap.ID
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*identity.GetResponse); ok {
			for _, ident := range r.List {
				if strings.EqualFold(ident.Email, c.account.Username) {
					identityID = ident.ID
					break
				}
			}
			if identityID == "" && len(r.List) > 0 {
				identityID = r.List[0].ID
			}
		}
	}

	if identityID == "" {
		return "", fmt.Errorf("no identity found for username %s", c.account.Username)
	}
	return identityID, nil
}

func (c *JMAPClient) Validate() error {
	if err := cfg_g.ValidateAccountParams(c.account); err != nil {
		return err
	}
	return c.init()
}

func (c *JMAPClient) GetMatchingLabels(prefix string) ([]string, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	var matched []string
	prefixLower := strings.ToLower(prefix)
	for name := range c.mailboxId {
		if strings.HasPrefix(strings.ToLower(name), prefixLower) {
			matched = append(matched, name)
		}
	}

	uniqueMap := make(map[string]bool)
	var result []string
	for _, m := range matched {
		canonical := m
		for origName := range c.mailboxId {
			if strings.EqualFold(origName, m) && origName != strings.ToLower(origName) {
				canonical = origName
				break
			}
		}
		if !uniqueMap[canonical] {
			uniqueMap[canonical] = true
			result = append(result, canonical)
		}
	}

	sort.Strings(result)
	return result, nil
}

// FetchAndDownloadEmails downloads emails from a JMAP mailbox.
func (c *JMAPClient) FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	mbID, err := c.getMailboxID(folderName)
	if err != nil {
		return nil, err
	}

	slog.Info("FetchAndDownloadEmails: querying mailbox", slog.String("folder", folderName), slog.String("mailboxId", string(mbID)))

	idToName := make(map[jmap.ID]string)
	for name, id := range c.mailboxId {
		idToName[id] = name
	}
	if n, ok := idToName[mbID]; ok {
		slog.Info("FetchAndDownloadEmails mailbox resolved to", slog.String("mailboxId", string(mbID)), slog.String("mailboxName", n))
	} else {
		slog.Warn("FetchAndDownloadEmails mailbox name not found", slog.String("mailboxId", string(mbID)))
	}

	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: c.accID,
		Filter: &email.FilterCondition{
			InMailbox: mbID,
		},
		Limit: uint64(c.config.Limit),
	})

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	var queryResp *email.QueryResponse
	for _, inv := range resp.Responses {
		if qr, ok := inv.Args.(*email.QueryResponse); ok {
			queryResp = qr
		}
	}

	if queryResp == nil || len(queryResp.IDs) == 0 {
		slog.Info("FetchAndDownloadEmails: JMAP query returned no results", slog.String("folder", folderName), slog.String("mailboxId", string(mbID)))
		return nil, nil
	}

	slog.Info("FetchAndDownloadEmails: JMAP query result", slog.String("folder", folderName), slog.String("mailboxId", string(mbID)), slog.Int("returnedIds", len(queryResp.IDs)), slog.Any("idList", queryResp.IDs))

	reqGet := &jmap.Request{}
	reqGet.Invoke(&email.Get{
		Account:    c.accID,
		IDs:        queryResp.IDs,
		Properties: []string{"id", "blobId", "keywords"},
	})

	respGet, err := c.client.Do(reqGet)
	if err != nil {
		return nil, err
	}

	var getResp *email.GetResponse
	for _, inv := range respGet.Responses {
		if gr, ok := inv.Args.(*email.GetResponse); ok {
			getResp = gr
		}
	}

	if getResp == nil {
		return nil, fmt.Errorf("failed to retrieve email details")
	}

	var ids []string
	for _, em := range getResp.List {
		idStr := string(em.ID)
		ids = append(ids, idStr)
	}

	cacheDir := c.config.DownloadDir

	readIDs := cache.LoadReadState(cacheDir)
	for _, em := range getResp.List {
		idStr := string(em.ID)
		readIDs[idStr] = em.Keywords["$seen"]
	}
	if err := cache.SaveReadState(cacheDir, readIDs); err != nil {
		slog.Error("Failed to save read state after JMAP sync", slog.Any("error", err))
		return nil, fmt.Errorf("failed to save read state: %w", err)
	}

	_ = os.MkdirAll(filepath.Join(cacheDir, "messages"), 0700)

	if len(ids) == 0 {
		return nil, nil
	}

	if len(getResp.List) > 0 {
		slog.Info("Refreshing JMAP email cache", slog.Int("count", len(ids)), slog.String("folder", folderName))

		numWorkers := 5
		if numWorkers > len(getResp.List) {
			numWorkers = len(getResp.List)
		}

		taskChan := make(chan *email.Email, len(getResp.List))
		for _, em := range getResp.List {
			taskChan <- em
		}
		close(taskChan)

		errChan := make(chan error, len(getResp.List))
		var wg sync.WaitGroup
		var mu sync.Mutex
		count := 0

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range taskChan {
					idStr := string(item.ID)
					if exists, err := msg.Exists(cacheDir, idStr); err == nil && exists {
						continue
					}
					emailBytes, err := c.downloadBlob(string(c.accID), string(item.BlobID), idStr+".eml", "message/rfc822")
					if err != nil {
						errChan <- fmt.Errorf("failed to download message %s: %w", idStr, err)
						continue
					}

					now := time.Now()
					dateStr := now.Format("2006/01/02")
					dir := filepath.Join(cacheDir, "messages", dateStr)
					if err := os.MkdirAll(dir, 0700); err != nil {
						errChan <- fmt.Errorf("failed to create message dir: %w", err)
						continue
					}
					emlPath := filepath.Join(dir, idStr+".eml")
					if err := os.WriteFile(emlPath, emailBytes, 0600); err != nil {
						errChan <- fmt.Errorf("failed to write message %s to cache: %w", idStr, err)
						continue
					}

					mu.Lock()
					count++
					if err := msg.Store(cacheDir, idStr, emailBytes, now); err != nil {
						slog.Error("Failed to store message", slog.String("msgID", idStr), slog.Any("error", err))
					}
					mu.Unlock()

					slog.Info("Cached JMAP email locally", slog.String("message_id", idStr), slog.String("folder", folderName))
				}
			}()
		}
		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				slog.Warn("Failed to download JMAP email", slog.String("error", err.Error()))
			}
		}
	}

	if err := label.ReplaceAll(cacheDir, folderName, ids); err != nil {
		slog.Error("Failed to save folder index", slog.Any("error", err))
	}

	return ids, nil
}

func (c *JMAPClient) Config() *cfg_g.Config {
	return c.config
}

func (c *JMAPClient) RewriteRuleLabelCasing(label string) string {
	if err := c.init(); err != nil {
		return label
	}

	jmapLabels := make(map[string]string)
	for path := range c.mailboxId {
		jmapLabels[strings.ToLower(path)] = path
	}

	parts := strings.Split(label, "/")
	for i := 1; i <= len(parts); i++ {
		prefix := strings.Join(parts[:i], "/")
		lowerPrefix := strings.ToLower(prefix)
		if exactName, ok := jmapLabels[lowerPrefix]; ok {
			exactParts := strings.Split(exactName, "/")
			for idx, val := range exactParts {
				if idx >= len(parts) {
					break
				}
				if parts[idx] != val {
					parts[idx] = val
				}
			}
		}
	}
	return strings.Join(parts, "/")
}

func (c *JMAPClient) InboxFolder() string {
	return "Inbox"
}

func (c *JMAPClient) BackendType() string { return "jmap" }
