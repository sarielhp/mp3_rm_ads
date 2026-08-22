package tui

import (
	"bytes"
	"fmt"
	"log/slog"
	"mail_cli/backend/gmail"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/scan"
	"mail_cli/spam"
	"mail_cli/uicommon"
	"mime"
	"net/mail"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) Config() *cfg_g.Config {
	if m.client != nil {
		return m.client.Config()
	}
	return nil
}

func (m *tuiModel) isReadOnly() bool {
	if m.Config() != nil && m.Config().ReadOnly {
		return true
	}
	if m.cfg != nil && m.cfg.ReadOnly {
		return true
	}
	accounts := m.getAccounts()
	idx := m.currentAccountIndex()
	if idx >= 0 && idx < len(accounts) {
		return accounts[idx].ReadOnly
	}
	return false
}

func (m *tuiModel) Init() tea.Cmd {
	m.lastTime = time.Now()
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Tick(time.Second*10, func(time.Time) tea.Msg { return timeTickMsg{} }))
	cmds = append(cmds, m.bgFetchFoldersCmd())
	if m.currentF != "" {
		if len(m.emails) == 0 {
			m.showLoad = "Downloading..."
		}
		cmds = append(cmds, m.loadEmailsCmd(m.currentF))
	} else {
		m.showLoad = "Downloading..."
		cmds = append(cmds, func() tea.Msg {
			items, err := m.client.GetLabelItems()
			if err != nil {
				return errMsg{err}
			}
			return foldersLoadedMsg{folders: items}
		})
	}
	return tea.Batch(cmds...)
}

func (m *tuiModel) bgFetchFoldersCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.GetLabelItems()
		if err != nil {
			return errMsg{err}
		}
		return foldersLoadedMsg{folders: items}
	}
}

func (m *tuiModel) loadEmailsCmd(folder string) tea.Cmd {
	return func() tea.Msg {
		cacheDir := m.client.Config().DownloadDir
		slog.Info("loadEmailsCmd: calling FetchAndDownloadEmails", slog.String("folder", folder), slog.String("cacheDir", cacheDir))
		ids, err := m.client.FetchAndDownloadEmails(folder, "")
		if err != nil {
			slog.Error("loadEmailsCmd: FetchAndDownloadEmails failed", slog.String("folder", folder), slog.Any("error", err))
			return errMsg{err}
		}
		m.isRead = cache.LoadReadState(cacheDir)
		m.isReplied = cache.LoadRepliedState(cacheDir)
		var msgs []uicommon.FolderEmail
		var classifyIDs []string
		for _, id := range ids {
			data, err := msg.Read(cacheDir, id)
			if err != nil {
				continue
			}
			if em := email.ParseReader(bytes.NewReader(data), id, ""); em != nil {
				em.IsRead = m.isRead[id]
				em.IsReplied = m.isReplied[id]
				needsClassify := true
				if info, err := msg.GetInfo(cacheDir, id); err == nil {
					em.IsSpam = info.IsSpam
					em.IsPolitical = info.IsPolitical
					em.IsBlacklisted = info.IsBlacklisted
					needsClassify = !info.Classified
				}
				if needsClassify {
					classifyIDs = append(classifyIDs, id)
				}
				msgs = append(msgs, *em)
			}
		}
		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].EmailDate.After(msgs[j].EmailDate)
		})
		return emailsLoadedMsg{emails: msgs, classifyIDs: classifyIDs}
	}
}

func (m *tuiModel) bgClassifyCmd(ids []string) tea.Cmd {
	return func() tea.Msg {
		if len(ids) == 0 {
			return classifyDoneMsg{}
		}
		slog.Info("Background classification starting", slog.Int("count", len(ids)))
		_, _, _, err := scan.ScanEmails(ids, m.client.Config(), "")
		if err != nil {
			slog.Error("Background classification failed", slog.Any("error", err))
		} else {
			slog.Info("Background classification complete", slog.Int("count", len(ids)))
		}
		return classifyDoneMsg{}
	}
}

func (m *tuiModel) loadDetailCmd(id string, eIdx int) tea.Cmd {
	return func() tea.Msg {
		cfg := m.client.Config()
		cd := cfg.DownloadDir
		data, err := msg.Read(cd, id)
		if err != nil {
			return detailMsg{headers: "(Not cached)", body: "(Email not found locally)"}
		}
		msg, err := mail.ReadMessage(bytes.NewReader(data))
		if err != nil {
			return detailMsg{headers: "(Parse error)", body: err.Error()}
		}
		dec := new(mime.WordDecoder)
		subj := uicommon.DecDecode(dec, msg.Header.Get("Subject"))
		frm := uicommon.DecDecode(dec, msg.Header.Get("From"))
		dS := msg.Header.Get("Date")
		hdrs := fmt.Sprintf("From: %s\nSubject: %s\nDate: %s", frm, subj, dS)
		body, _ := gmail.ExtractPlainBodyText(msg)
		wrap := uicommon.WrapText(body, 72)
		atts, _ := email.ExtractAttachments(data)
		return detailMsg{headers: hdrs, body: wrap, attachments: atts}
	}
}

func (m *tuiModel) cmdArchive(id string) tea.Cmd {
	return func() tea.Msg {
		cfg := m.client.Config()
		cd := cfg.DownloadDir
		tgt, err := m.bk.ResolveArchiveTarget(m.client)
		if err != nil {
			return errMsg{err: err}
		}
		if err := m.client.MoveEmail([]string{id}, m.currentF, tgt); err != nil {
			return errMsg{err: err}
		}
		if err := label.Move(cd, id, m.currentF, tgt); err != nil {
			return errMsg{err: err}
		}
		_ = msg.ClearClassification(cd, id)
		return archiveDoneMsg{id: id}
	}
}

func (m *tuiModel) cmdSpam(id string) tea.Cmd {
	return func() tea.Msg {
		cfg := m.client.Config()
		cd := cfg.DownloadDir
		_, err := msg.Read(cd, id)
		if err != nil {
			return spamDoneMsg{id: id}
		}
		_ = m.client.ReportSpam([]string{id}, m.currentF)
		_ = label.Move(cd, id, m.currentF, cfg.SpamFolder)
		return spamDoneMsg{id: id}
	}
}

func (m *tuiModel) cmdUnspam(id string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		cfg := m.client.Config()
		err := spam.UnspamByID(m.client, cfg, id)
		elapsed := time.Since(startTime)
		if elapsed < time.Second {
			time.Sleep(time.Second - elapsed)
		}
		if err != nil {
			return errMsg{err: err}
		}
		return unspamDoneMsg{id: id}
	}
}

func (m *tuiModel) cmdDelete(id string) tea.Cmd {
	return func() tea.Msg {
		cfg := m.client.Config()
		cd := cfg.DownloadDir
		tgt, err := m.bk.ResolveTrashTarget(m.client)
		if err == nil {
			_ = m.client.MoveEmail([]string{id}, m.currentF, tgt)
			_ = label.Move(cd, id, m.currentF, tgt)
		}
		_ = msg.Delete(cd, id)
		return deleteDoneMsg{id: id}
	}
}

func (m *tuiModel) loadCachedEmailsForFolder(folder string) {
	if m.client == nil {
		slog.Warn("loadCachedEmailsForFolder: client is nil")
		return
	}
	if m.client.Config() == nil {
		slog.Warn("loadCachedEmailsForFolder: client config is nil")
		return
	}
	cacheDir := m.client.Config().DownloadDir
	slog.Info("loadCachedEmailsForFolder starting", slog.String("folder", folder), slog.String("cacheDir", cacheDir))

	if !label.HasStructure(cacheDir) {
		slog.Info("loadCachedEmailsForFolder: cache structure not found, skipping")
		return
	}

	ids, err := label.IDs(cacheDir, folder)
	if err != nil {
		slog.Info("loadCachedEmailsForFolder: no folder index", slog.String("folder", folder))
		return
	}
	var cachedEmails []uicommon.FolderEmail
	for _, id := range ids {
		data, err := msg.Read(cacheDir, id)
		if err != nil {
			continue
		}
		if em := email.ParseReader(bytes.NewReader(data), id, ""); em != nil {
			em.IsRead = m.isRead[id]
			em.IsReplied = m.isReplied[id]
			if info, err := msg.GetInfo(cacheDir, id); err == nil {
				em.IsSpam = info.IsSpam
				em.IsPolitical = info.IsPolitical
				em.IsBlacklisted = info.IsBlacklisted
			}
			cachedEmails = append(cachedEmails, *em)
		}
	}

	prevSelectedID := m.selectedID
	prevFolder := m.currentF

	m.rawEmails = cachedEmails
	m.rebuildVisibleEmails()

	if prevFolder == folder && prevSelectedID != "" {
		found := false
		for i, e := range m.emails {
			if e.ID == prevSelectedID {
				m.eIdx = i
				m.selectedID = prevSelectedID
				found = true
				break
			}
		}
		if !found {
			if len(m.emails) > 0 {
				m.eIdx = min(m.eIdx, len(m.emails)-1)
				m.selectedID = m.emails[m.eIdx].ID
			} else {
				m.eIdx = 0
				m.selectedID = ""
			}
		}
	} else {
		m.folderJustChanged = true
		if len(m.emails) > 0 {
			m.eIdx = len(m.emails) - 1
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.eIdx = 0
			m.selectedID = ""
		}
	}
	m.adjustScrollOffset()
}

func (m *tuiModel) removeEmailByID(id string) {
	newRaw := make([]uicommon.FolderEmail, 0, len(m.rawEmails))
	for _, e := range m.rawEmails {
		if e.ID != id {
			newRaw = append(newRaw, e)
		}
	}
	m.rawEmails = newRaw
	m.rebuildVisibleEmails()
	if len(m.emails) > 0 {
		m.eIdx = min(m.eIdx, len(m.emails)-1)
		m.selectedID = m.emails[m.eIdx].ID
	} else {
		m.eIdx = 0
		m.selectedID = ""
	}
	m.adjustScrollOffset()
}

func (m *tuiModel) cmdDebugDump() tea.Cmd {
	return func() tea.Msg {
		slog.Info("=== DEBUG DUMP START ===")
		slog.Info("State",
			slog.String("currentFolder", m.currentF),
			slog.Int("visibleEmails", len(m.emails)),
			slog.Int("rawEmails", len(m.rawEmails)),
			slog.Int("selectedIndex", m.eIdx),
			slog.String("selectedID", m.selectedID),
		)
		cd := m.client.Config().DownloadDir
		ids, err := label.IDs(cd, m.currentF)
		if err != nil {
			slog.Info("Debug dump: no folder index", slog.Any("error", err))
		} else {
			slog.Info("Cached message IDs from folder index", slog.Int("count", len(ids)), slog.Any("ids", ids))
		}
		slog.Info("=== DEBUG DUMP END ===")
		return doneMsg{id: "debug"}
	}
}

func (m *tuiModel) cleanupIsReadMap() {
	if len(m.isRead) > 10000 {
		slog.Info("Cleaning up isRead map", slog.Int("currentSize", len(m.isRead)))
		ids := make(map[string]bool)
		for _, e := range m.rawEmails {
			ids[e.ID] = true
		}
		for id := range m.isRead {
			if !ids[id] {
				delete(m.isRead, id)
			}
		}
		slog.Info("Cleaned up isRead map", slog.Int("newSize", len(m.isRead)))
	}
}

func (m *tuiModel) loadAllCachedEmails() {
	if m.client == nil || m.client.Config() == nil {
		return
	}
	cacheDir := m.client.Config().DownloadDir
	if !label.HasStructure(cacheDir) {
		return
	}

	var allEmails []uicommon.FolderEmail
	if len(m.folders) == 0 {
		return
	}

	for _, f := range m.folders {
		ids, err := label.IDs(cacheDir, f.FullName)
		if err != nil {
			continue
		}
		for _, id := range ids {
			data, err := msg.Read(cacheDir, id)
			if err != nil {
				continue
			}
			if em := email.ParseReader(bytes.NewReader(data), id, ""); em != nil {
				em.IsRead = m.isRead[id]
				em.IsReplied = m.isReplied[id]
				if info, err := msg.GetInfo(cacheDir, id); err == nil {
					em.IsSpam = info.IsSpam
					em.IsPolitical = info.IsPolitical
					em.IsBlacklisted = info.IsBlacklisted
				}
				allEmails = append(allEmails, *em)
			}
		}
	}

	sort.Slice(allEmails, func(i, j int) bool {
		return allEmails[i].EmailDate.After(allEmails[j].EmailDate)
	})

	m.savedRawEmails = m.rawEmails
	m.savedCurrentF = m.currentF
	m.rawEmails = allEmails
}
