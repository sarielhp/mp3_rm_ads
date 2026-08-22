package tui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/email"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) kIndexOpenDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.eIdx >= 0 && m.eIdx < len(m.emails) {
		em := m.emails[m.eIdx]
		key := em.MessageID
		if key == "" {
			key = em.ID
		}
		if em.ThreadHasReplies && !m.expandedThreads[key] {
			m.expandRecursively(key, em.ID)
			m.rebuildVisibleEmails()
			for i, e := range m.emails {
				if e.ID == m.selectedID {
					m.eIdx = i
					break
				}
			}
			return m, nil
		}
		m.mode = ModeDetail
		m.selectedID = em.ID
		var cmds []tea.Cmd
		if !em.IsRead {
			m.emails[m.eIdx].IsRead = true
			m.isRead[em.ID] = true
			if cmd := m.markAsReadCmd(em.ID); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		h := 20
		if m.height > 0 {
			h = max(5, m.height-3)
		}
		if m.width > 0 {
			m.vp.Width = max(20, m.width-2)
			m.vp.Height = h
		}
		m.vp.SetContent("")
		m.detailH = ""
		m.detail = "(Loading message...)"
		cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m *tuiModel) markAsReadCmd(msgID string) tea.Cmd {
	if m.isReadOnly() || m.client == nil || m.client.Config() == nil {
		return nil
	}
	downloadDir := m.client.Config().DownloadDir
	client := m.client
	return func() tea.Msg {
		_ = cache.MarkIDsRead(downloadDir, []string{msgID}, nil)
		_ = client.MarkAsRead([]string{msgID})
		return nil
	}
}

func (m *tuiModel) kIndexActions(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch {
	case s == "E":
		return m.archiveCurrentEmail()
	case s == "s":
		return m.spamCurrentEmail()
	case s == "U" || s == "u":
		return m.unspamCurrentEmail()
	case s == "d":
		return m.deleteCurrentEmail()
	case s == "/":
		if m.fuzzySearch {
			m.fuzzySearch = false
			m.fuzzyGlobal = false
		}
		m.inSearch = true
		m.searchQuery = ""
		m.rebuildVisibleEmails()
		return m, nil
	case s == "~":
		m.loadAllCachedEmails()
		m.fuzzySearch = true
		m.fuzzyGlobal = true
		m.inSearch = true
		m.searchQuery = ""
		m.rebuildVisibleEmails()
		if len(m.emails) > 0 {
			m.eIdx = 0
			m.selectedID = m.emails[0].ID
		}
		return m, nil
	case s == "!":
		m.fuzzySearch = true
		m.fuzzyGlobal = false
		m.inSearch = true
		m.searchQuery = ""
		m.rebuildVisibleEmails()
		if len(m.emails) > 0 {
			m.eIdx = 0
			m.selectedID = m.emails[0].ID
		}
		return m, nil
	case s == "r" || s == "g":
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			targetID := m.emails[m.eIdx].ID
			groupReply := (s == "g")
			return m, m.replyCmd(targetID, groupReply)
		}
	case s == "R":
		m.showError = false
		m.err = nil
		m.showLoad = "Downloading..."
		return m, m.loadEmailsCmd(m.currentF)
	case s == "m" || s == "alt" || s == "alt+m" || k.Alt:
		m.menuOpen = true
		m.menuCursor = 0
		return m, nil
	case s == "a" || s == "A" || s == "f3" || s == "F3" || k.Type == tea.KeyF3:
		m.openAccountOverlay()
		return m, nil
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		num := int(s[0] - '0')
		accounts := m.getAccounts()
		if num-1 < len(accounts) {
			return m.switchAccountByIndex(num - 1)
		}
		if s == "1" {
			slog.Info("DEBUG KEY 1 PRESSED")
			return m, m.cmdDebugDump()
		}
		return m, nil
	default:
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) currentEmailID() string {
	if m.mode == ModeIndex && m.eIdx >= 0 && m.eIdx < len(m.emails) {
		return m.emails[m.eIdx].ID
	}
	return m.selectedID
}

func (m *tuiModel) archiveCurrentEmail() (tea.Model, tea.Cmd) {
	targetID := m.currentEmailID()
	if targetID == "" {
		return m, nil
	}
	if m.isReadOnly() {
		m.err = fmt.Errorf("🔒 Read-Only Mode: Action simulated (no server changes)")
		m.showError = true
		return m, nil
	}
	m.removeEmailByID(targetID)
	m.addPending(targetID, PendingArchive)
	m.mode = ModeIndex
	return m, m.cmdArchive(targetID)
}

func (m *tuiModel) spamCurrentEmail() (tea.Model, tea.Cmd) {
	targetID := m.currentEmailID()
	if targetID == "" {
		return m, nil
	}
	if m.isReadOnly() {
		m.err = fmt.Errorf("🔒 Read-Only Mode: Action simulated (no server changes)")
		m.showError = true
		return m, nil
	}
	m.removeEmailByID(targetID)
	m.addPending(targetID, PendingSpam)
	m.mode = ModeIndex
	return m, m.cmdSpam(targetID)
}

func (m *tuiModel) unspamCurrentEmail() (tea.Model, tea.Cmd) {
	targetID := m.currentEmailID()
	if targetID == "" {
		return m, nil
	}
	if m.isReadOnly() {
		m.err = fmt.Errorf("🔒 Read-Only Mode: Action simulated (no server changes)")
		m.showError = true
		return m, nil
	}
	if !strings.EqualFold(m.currentF, "inbox") {
		m.removeEmailByID(targetID)
	}
	m.addPending(targetID, PendingUnspam)
	m.mode = ModeIndex
	m.showLoad = "Unspamming..."
	return m, m.cmdUnspam(targetID)
}

func (m *tuiModel) deleteCurrentEmail() (tea.Model, tea.Cmd) {
	targetID := m.currentEmailID()
	if targetID == "" {
		return m, nil
	}
	if m.isReadOnly() {
		m.err = fmt.Errorf("🔒 Read-Only Mode: Action simulated (no server changes)")
		m.showError = true
		return m, nil
	}
	m.removeEmailByID(targetID)
	m.addPending(targetID, PendingDelete)
	m.mode = ModeIndex
	return m, m.cmdDelete(targetID)
}

func (m *tuiModel) kDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.attachmentDrawer {
		return m.kAttachmentDrawer(k)
	}
	if m.inDetailSearch {
		return m.kDetailSearch(k)
	}
	s := k.String()
	switch {
	case k.Type == tea.KeyUp || s == "k":
		m.vp.ScrollUp(1)
	case k.Type == tea.KeyDown || s == "j":
		m.vp.ScrollDown(1)
	case s == "pageup" || s == "pgup":
		m.vp.ScrollUp(m.vp.Height)
	case s == "pagedown" || s == "pgdown" || s == "pgdn":
		m.vp.ScrollDown(m.vp.Height)
	case s == "home":
		m.vp.GotoTop()
	case s == "end":
		m.vp.GotoBottom()
	case k.Type == tea.KeyLeft || s == "left":
		if len(m.emails) > 0 {
			m.eIdx--
			if m.eIdx < 0 {
				m.eIdx = 0
			}
			m.selectedID = m.emails[m.eIdx].ID
			var cmds []tea.Cmd
			if !m.emails[m.eIdx].IsRead {
				m.emails[m.eIdx].IsRead = true
				m.isRead[m.selectedID] = true
				if cmd := m.markAsReadCmd(m.selectedID); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.vp.GotoTop()
			m.vp.SetContent("")
			m.detailH = ""
			m.detail = "(Loading message...)"
			cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
			return m, tea.Batch(cmds...)
		}
	case k.Type == tea.KeyRight || s == "right":
		if len(m.emails) > 0 {
			m.eIdx++
			if m.eIdx >= len(m.emails) {
				m.eIdx = len(m.emails) - 1
			}
			m.selectedID = m.emails[m.eIdx].ID
			var cmds []tea.Cmd
			if !m.emails[m.eIdx].IsRead {
				m.emails[m.eIdx].IsRead = true
				m.isRead[m.selectedID] = true
				if cmd := m.markAsReadCmd(m.selectedID); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.vp.GotoTop()
			m.vp.SetContent("")
			m.detailH = ""
			m.detail = "(Loading message...)"
			cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
			return m, tea.Batch(cmds...)
		}
	case s == "E":
		return m.archiveCurrentEmail()
	case s == "s":
		return m.spamCurrentEmail()
	case s == "U" || s == "u":
		return m.unspamCurrentEmail()
	case s == "d":
		return m.deleteCurrentEmail()
	case s == "r" || s == "g":
		targetID := m.selectedID
		groupReply := (s == "g")
		return m, m.replyCmd(targetID, groupReply)
	case s == "A":
		m.openAccountOverlay()
		return m, nil
	case s == "a":
		if len(m.attachments) > 0 {
			m.attachmentDrawer = true
			m.attCursor = 0
			m.saveStatus = ""
			return m, nil
		}
		m.openAccountOverlay()
		return m, nil
	case s == "m" || s == "alt" || s == "alt+m" || k.Alt:
		m.menuOpen = true
		m.menuCursor = 0
		return m, nil
	case s == "/":
		m.inDetailSearch = true
		m.detailSearch = ""
		return m, nil
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		num := int(s[0] - '0')
		accounts := m.getAccounts()
		if num-1 < len(accounts) {
			return m.switchAccountByIndex(num - 1)
		}
		if s == "1" {
			return m, m.cmdDebugDump()
		}
		return m, nil
	default:
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) kSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch {
	case k.Type == tea.KeyEsc:
		m.inSearch = false
		m.searchQuery = ""
		if m.fuzzySearch {
			m.fuzzySearch = false
			if m.fuzzyGlobal && m.savedRawEmails != nil {
				m.rawEmails = m.savedRawEmails
				m.savedRawEmails = nil
				m.currentF = m.savedCurrentF
			}
			m.fuzzyGlobal = false
		}
		m.rebuildVisibleEmails()
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.eIdx = 0
			m.selectedID = ""
		}
	case k.Type == tea.KeyTab || s == "tab" || k.Type == tea.KeyEnter:
		m.inSearch = false
		if len(m.emails) > 0 {
			m.eIdx = 0
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.eIdx = 0
			m.selectedID = ""
		}
		return m, nil
	case k.Type == tea.KeyBackspace || s == "backspace" || s == "ctrl+h":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.rebuildVisibleEmails()
			if len(m.emails) > 0 {
				m.eIdx = min(m.eIdx, len(m.emails)-1)
				m.selectedID = m.emails[m.eIdx].ID
			} else {
				m.eIdx = 0
				m.selectedID = ""
			}
		}
	case k.Type == tea.KeySpace:
		m.searchQuery += " "
		m.rebuildVisibleEmails()
		if len(m.emails) > 0 {
			m.eIdx = min(m.eIdx, len(m.emails)-1)
			m.selectedID = m.emails[m.eIdx].ID
		} else {
			m.eIdx = 0
			m.selectedID = ""
		}
	default:
		if len(k.Runes) > 0 {
			m.searchQuery += string(k.Runes)
			m.rebuildVisibleEmails()
			if len(m.emails) > 0 {
				m.eIdx = min(m.eIdx, len(m.emails)-1)
				m.selectedID = m.emails[m.eIdx].ID
			} else {
				m.eIdx = 0
				m.selectedID = ""
			}
		}
	}
	return m, nil
}

func (m *tuiModel) kDetailSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch {
	case k.Type == tea.KeyEsc:
		m.inDetailSearch = false
		m.detailSearch = ""
		m.detailVpDirty = true
		return m, nil
	case k.Type == tea.KeyEnter || s == "tab":
		m.inDetailSearch = false
		m.detailVpDirty = true
		return m, nil
	case k.Type == tea.KeyBackspace || s == "backspace" || s == "ctrl+h":
		if len(m.detailSearch) > 0 {
			m.detailSearch = m.detailSearch[:len(m.detailSearch)-1]
			m.detailVpDirty = true
		}
	case k.Type == tea.KeySpace:
		m.detailSearch += " "
		m.detailVpDirty = true
	default:
		if len(k.Runes) > 0 {
			m.detailSearch += string(k.Runes)
			m.detailVpDirty = true
		}
	}
	return m, nil
}

func (m *tuiModel) kAttachmentDrawer(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if len(m.attachments) == 0 {
		m.attachmentDrawer = false
		return m, nil
	}
	switch s {
	case "a", "esc", "q":
		m.attachmentDrawer = false
		m.saveStatus = ""
		return m, nil
	case "up", "k":
		m.attCursor--
		if m.attCursor < 0 {
			m.attCursor = 0
		}
	case "down", "j":
		m.attCursor++
		if m.attCursor >= len(m.attachments) {
			m.attCursor = len(m.attachments) - 1
		}
	case "enter", "o":
		att := m.attachments[m.attCursor]
		tmpFile, err := writeTempAttachment(att)
		if err == nil {
			go openSystemFile(tmpFile)
			m.saveStatus = "Opened: " + att.Filename
		} else {
			m.saveStatus = "Error: " + err.Error()
		}
		m.saveStatusTime = time.Now()
	case "s":
		att := m.attachments[m.attCursor]
		home, err := os.UserHomeDir()
		if err == nil {
			destDir := filepath.Join(home, "Downloads")
			_ = os.MkdirAll(destDir, 0755)
			destPath := filepath.Join(destDir, att.Filename)
			err = os.WriteFile(destPath, att.Data, 0644)
			if err == nil {
				m.saveStatus = "Saved to " + destPath
			} else {
				m.saveStatus = "Error saving: " + err.Error()
			}
		} else {
			m.saveStatus = "Error finding home: " + err.Error()
		}
		m.saveStatusTime = time.Now()
	}
	return m, nil
}

func writeTempAttachment(att email.Attachment) (string, error) {
	tmpDir, err := app.GetTempDir()
	if err != nil {
		return "", err
	}
	tmpPath := filepath.Join(tmpDir, att.Filename)
	err = os.WriteFile(tmpPath, att.Data, 0600)
	return tmpPath, err
}

func openSystemFile(path string) {
	cmd := exec.Command("xdg-open", path)
	_ = cmd.Run()
}

func (m *tuiModel) kMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch s {
	case "up", "k":
		m.menuCursor--
		if m.menuCursor < 0 {
			m.menuCursor = len(m.menuItems) - 1
		}
	case "down", "j":
		m.menuCursor++
		if m.menuCursor >= len(m.menuItems) {
			m.menuCursor = 0
		}
	case "enter":
		m.menuOpen = false
		if m.menuCursor < len(m.menuItems) {
			switch m.menuItems[m.menuCursor] {
			case "Move to Spam":
				var targetID string
				if m.mode == ModeIndex && m.eIdx >= 0 && m.eIdx < len(m.emails) {
					targetID = m.emails[m.eIdx].ID
				} else if m.mode == ModeDetail {
					targetID = m.selectedID
				}
				if targetID != "" {
					m.removeEmailByID(targetID)
					m.addPending(targetID, PendingSpam)
					m.mode = ModeIndex
					return m, m.cmdSpam(targetID)
				}
			case "Switch Account":
				m.openAccountOverlay()
				return m, nil
			case "Exit":
				m.done = true
				return m, tea.Quit
			}
		}
	case "esc", "q", "m", "alt", "alt+m":
		m.menuOpen = false
	default:
		if k.Alt {
			m.menuOpen = false
		}
	}
	return m, nil
}
