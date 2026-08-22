package tui

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"mail_cli/cache"
	"mail_cli/label"
	"mail_cli/uicommon"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) adjustScrollOffset() {
	availH := m.height - 2
	if m.showError {
		availH--
	}
	numLines := availH
	if m.splitPreview && len(m.emails) > 0 {
		numLines = max(3, (availH-1)/2)
	}
	if numLines <= 0 {
		numLines = 1
	}
	oldStart := m.indexStart
	m.indexStart = max(0, min(m.indexStart, len(m.emails)-numLines))
	if m.eIdx < m.indexStart {
		m.indexStart = m.eIdx
	} else if m.eIdx >= m.indexStart+numLines {
		m.indexStart = m.eIdx - numLines + 1
	}
	var emailDetails []string
	for _, em := range m.emails {
		emailDetails = append(emailDetails, fmt.Sprintf("%s(read=%t):%q", em.ID, em.IsRead, em.Subject))
	}
	slog.Info("adjustScrollOffset completed",
		slog.Int("height", m.height),
		slog.Int("numLines", numLines),
		slog.Int("emailsCount", len(m.emails)),
		slog.Any("emails", emailDetails),
		slog.Int("eIdx", m.eIdx),
		slog.Int("oldStart", oldStart),
		slog.Int("newStart", m.indexStart),
		slog.Int("mode", int(m.mode)),
	)
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevMode := m.mode
	model, cmd := m.updateInner(msg)
	if tm, ok := model.(*tuiModel); ok {
		tm.adjustScrollOffset()
		if prevMode == ModeDetail && tm.mode == ModeIndex {
			if cmd == nil {
				cmd = tea.ClearScreen
			} else {
				cmd = tea.Batch(cmd, tea.ClearScreen)
			}
		}
	}
	return model, cmd
}

func (m *tuiModel) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.mode == ModeDetail {
			m.vp.Width = max(20, m.width-2)
			m.vp.Height = max(5, m.height-3)
		} else {
			m.vp.Width = max(20, m.width-2)
			m.vp.Height = max(5, m.height-2)
		}
		return m, nil

	case tea.KeyMsg:
		return m.key(msg)

	case foldersLoadedMsg:
		folders := msg.folders
		gFolders := append([]label.LabelItem{}, folders...)
		sort.Slice(gFolders, func(i, j int) bool {
			return strings.ToLower(gFolders[i].FullName) < strings.ToLower(gFolders[j].FullName)
		})
		m.globalFolders = gFolders
		newGlobalTree := uicommon.BuildLabelTree(gFolders)
		TransferExpandedStates(m.globalTree, newGlobalTree)
		m.globalTree = newGlobalTree
		if m.lpr != "" {
			lower := strings.ToLower(m.lpr)
			var filtered []label.LabelItem
			for _, f := range folders {
				if strings.Contains(strings.ToLower(f.FullName), lower) {
					filtered = append(filtered, f)
				}
			}
			folders = filtered
			m.lpr = ""
		}
		folders = append([]label.LabelItem{}, folders...)
		sort.Slice(folders, func(i, j int) bool {
			return strings.ToLower(folders[i].FullName) < strings.ToLower(folders[j].FullName)
		})
		m.folders = folders
		newTree := uicommon.BuildLabelTree(folders)
		TransferExpandedStates(m.tree, newTree)
		m.tree = newTree
		m.buildEntries()
		targetFolder := "INBOX"
		if m.client != nil {
			targetFolder = m.client.InboxFolder()
		}
		resolved := label.ResolveFolder(folders, m.currentF, targetFolder)
		slog.Info("Update foldersLoadedMsg alignment", slog.String("oldCurrentF", m.currentF), slog.String("resolved", resolved), slog.String("targetFolder", targetFolder))
		if resolved != m.currentF {
			m.currentF = resolved
			m.selectedID = ""
			m.loadCachedEmailsForFolder(m.currentF)
			if len(m.emails) == 0 {
				m.showLoad = "Downloading..."
			} else {
				m.showLoad = ""
			}
			return m, m.loadEmailsCmd(m.currentF)
		}
		return m, nil

	case emailsLoadedMsg:
		slog.Info("emailsLoadedMsg replacing rawEmails", slog.Int("incomingEmails", len(msg.emails)), slog.Int("prevRawEmails", len(m.rawEmails)))
		var filtered []uicommon.FolderEmail
		for _, e := range msg.emails {
			if _, pending := m.pendingEmails[e.ID]; !pending {
				filtered = append(filtered, e)
			}
		}
		m.rawEmails = filtered
		m.rebuildVisibleEmails()
		if len(m.emails) > 0 {
			found := false
			if !m.folderJustChanged && m.selectedID != "" {
				for i, e := range m.emails {
					if e.ID == m.selectedID {
						m.eIdx = i
						found = true
						break
					}
				}
			}
			m.folderJustChanged = false
			if !found {
				m.eIdx = len(m.emails) - 1
				m.selectedID = m.emails[m.eIdx].ID
				m.adjustScrollOffset()
			} else {
				m.indexStart = max(0, min(m.indexStart, len(m.emails)-1))
			}
		} else {
			m.folderJustChanged = false
			m.eIdx = 0
			m.selectedID = ""
			m.indexStart = 0
		}
		m.showLoad = ""
		slog.Info("emailsLoadedMsg DONE", slog.String("currentF", m.currentF), slog.Int("visibleEmails", len(m.emails)))
		if len(msg.classifyIDs) > 0 {
			return m, m.bgClassifyCmd(msg.classifyIDs)
		}
		return m, nil

	case classifyDoneMsg:
		slog.Info("classifyDoneMsg: reloading folder display after background classification")
		m.loadCachedEmailsForFolder(m.currentF)
		return m, nil

	case detailMsg:
		m.detailH = msg.headers
		m.detail = msg.body
		m.attachments = msg.attachments
		m.attachmentDrawer = false
		m.attCursor = 0
		m.saveStatus = ""
		m.detailVpDirty = true
		return m, nil

	case doneMsg:
		m.showLoad = ""
		for i, e := range m.emails {
			if e.ID == msg.id {
				m.emails = append(m.emails[:i], m.emails[i+1:]...)
				break
			}
		}
		m.removePending(msg.id)
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.selectedID = ""
		}
		return m, m.loadEmailsCmd(m.currentF)

	case spamDoneMsg:
		m.showLoad = ""
		for i, e := range m.emails {
			if e.ID == msg.id {
				m.emails = append(m.emails[:i], m.emails[i+1:]...)
				break
			}
		}
		m.removePending(msg.id)
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.selectedID = ""
		}
		return m, nil
	case unspamDoneMsg:
		m.showLoad = ""
		for i, e := range m.rawEmails {
			if e.ID == msg.id {
				m.rawEmails[i].IsSpam = false
				m.rawEmails[i].IsPolitical = false
				m.rawEmails[i].IsBlacklisted = false
			}
		}
		for i, e := range m.emails {
			if e.ID == msg.id {
				m.emails[i].IsSpam = false
				m.emails[i].IsPolitical = false
				m.emails[i].IsBlacklisted = false
			}
		}
		if !strings.EqualFold(m.currentF, "inbox") {
			for i, e := range m.emails {
				if e.ID == msg.id {
					m.emails = append(m.emails[:i], m.emails[i+1:]...)
					break
				}
			}
		}
		m.removePending(msg.id)
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.selectedID = ""
		}
		return m, nil
	case archiveDoneMsg:
		m.showLoad = ""
		for i, e := range m.emails {
			if e.ID == msg.id {
				m.emails = append(m.emails[:i], m.emails[i+1:]...)
				break
			}
		}
		m.removePending(msg.id)
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.selectedID = ""
		}
		return m, nil

	case deleteDoneMsg:
		m.showLoad = ""
		for i, e := range m.emails {
			if e.ID == msg.id {
				m.emails = append(m.emails[:i], m.emails[i+1:]...)
				break
			}
		}
		m.removePending(msg.id)
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.selectedID = ""
		}
		return m, nil

	case replyEditorFinishedMsg:
		return m.handleReplyEditorFinished(msg)

	case emailSentMsg:
		m.showLoad = ""
		if msg.err != nil {
			m.err = msg.err
			m.showError = true
		} else {
			m.err = nil
			m.showError = false
			if msg.targetID != "" {
				m.isReplied[msg.targetID] = true
				if m.client != nil && m.client.Config() != nil {
					_ = cache.MarkIDsReplied(m.client.Config().DownloadDir, []string{msg.targetID}, m.isReplied)
				}
				for i := range m.rawEmails {
					if m.rawEmails[i].ID == msg.targetID {
						m.rawEmails[i].IsReplied = true
					}
				}
				for i := range m.emails {
					if m.emails[i].ID == msg.targetID {
						m.emails[i].IsReplied = true
					}
				}
			}
		}
		return m, nil

	case errMsg:
		m.showLoad = ""
		m.err = msg.err
		m.showError = true
		return m, nil

	case timeTickMsg:
		m.lastTime = time.Now()
		m.cleanupIsReadMap()
		return m, tea.Tick(time.Second*10, func(time.Time) tea.Msg { return timeTickMsg{} })

	default:
		return m, nil
	}
}
