package jmap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"mail_cli/cache"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
)

// FetchLatestAccountEmails queries Fastmail/JMAP account-wide for the latest N emails.
func (c *JMAPClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	// Invert mailboxId to map jmap.ID -> canonical folder name
	idToName := make(map[jmap.ID]string)
	for name, id := range c.mailboxId {
		idToName[id] = name
	}

	// 1. Query emails across the entire account sorted by receivedAt descending
	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: c.accID,
		Sort: []*email.SortComparator{
			{
				Property:    "receivedAt",
				IsAscending: false,
			},
		},
		Limit: uint64(limit),
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
		return nil, nil
	}

	// 2. Fetch email metadata including mailboxIds and keywords
	reqGet := &jmap.Request{}
	reqGet.Invoke(&email.Get{
		Account:    c.accID,
		IDs:        queryResp.IDs,
		Properties: []string{"id", "blobId", "mailboxIds", "keywords"},
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

	if getResp == nil || len(getResp.List) == 0 {
		return nil, nil
	}

	cacheDir := c.config.DownloadDir
	_ = os.MkdirAll(filepath.Join(cacheDir, "messages"), 0700)

	// 3. Update read state
	readIDs := cache.LoadReadState(cacheDir)
	for _, em := range getResp.List {
		readIDs[string(em.ID)] = em.Keywords["$seen"]
	}
	_ = cache.SaveReadState(cacheDir, readIDs)

	// 4. Download any missing bodies in parallel
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
					continue
				}
				emlPath := filepath.Join(dir, idStr+".eml")
				_ = os.WriteFile(emlPath, emailBytes, 0600)
				_ = msg.Store(cacheDir, idStr, emailBytes, now)
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

	// 5. Build results with folder names
	var refs []cfg_acc.MessageFolderRef
	for _, em := range getResp.List {
		idStr := string(em.ID)
		folder := "Inbox"
		for mbID := range em.MailboxIDs {
			if name, ok := idToName[mbID]; ok && name != "" {
				folder = name
				break
			}
		}
		refs = append(refs, cfg_acc.MessageFolderRef{
			MessageID: idStr,
			Folder:    folder,
		})
	}

	return refs, nil
}
