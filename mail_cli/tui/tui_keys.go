package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		m.done = true
		return m, tea.Quit
	}
	if s == "f1" {
		m.showHelp = !m.showHelp
		m.showDiag = false
		m.accountOverlayOpen = false
		m.treeOpen = false
		m.menuOpen = false
		m.showError = false
		return m, nil
	}
	if m.confirmSend {
		return m.kConfirmSend(k)
	}
	if m.menuOpen {
		return m.kMenu(k)
	}
	if m.showDiag {
		return m.kDiag(k)
	}
	if m.accountOverlayOpen {
		return m.kAccountOverlay(k)
	}
	if s == "ctrl+d" {
		m.showDiag = true
		m.showHelp = false
		m.treeOpen = false
		m.accountOverlayOpen = false
		m.diagVp.GotoBottom()
		return m, nil
	}
	if m.inSearch {
		return m.kSearch(k)
	}
	if m.treeOpen && m.inFolderSearch {
		return m.kFolder(k)
	}
	return m.kGlobalKey(k)
}

func (m *tuiModel) kDiag(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch s {
	case "ctrl+d", "esc", "q":
		m.showDiag = false
	case "up", "k":
		m.diagVp.ScrollUp(1)
	case "down", "j":
		m.diagVp.ScrollDown(1)
	case "pageup", "pgup":
		m.diagVp.ScrollUp(m.diagVp.Height)
	case "pagedown", "pgdn", "pgdown":
		m.diagVp.ScrollDown(m.diagVp.Height)
	case "home":
		m.diagVp.GotoTop()
	case "end":
		m.diagVp.GotoBottom()
	}
	return m, nil
}

func (m *tuiModel) kGlobalKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch s {
	case "q":
		if m.treeOpen {
			m.treeOpen = false
			return m, nil
		}
		if m.mode == ModeDetail {
			m.mode = ModeIndex
			return m, nil
		}
		if m.mode == ModeIndex && m.splitPreview {
			m.splitPreview = false
			return m, nil
		}
		m.done = true
		return m, tea.Quit
	case "A", "f3", "F3":
		m.openAccountOverlay()
		return m, nil
	case "[":
		return m.prevAccount()
	case "]":
		return m.nextAccount()
	case "f", "F":
		if m.currentF != "" {
			if m.treeOpen {
				m.treeOpen = false
			} else {
				m.treeOpen = true
				m.showGlobalTree = true
				m.buildEntries()
			}
		}
		return m, nil
	case "h", "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "tab":
		if m.treeOpen {
			return m.kFolder(k)
		}
		if m.mode == ModeDetail {
			m.mode = ModeIndex
			return m, nil
		}
		if m.mode == ModeIndex {
			if m.eIdx >= 0 && m.eIdx < len(m.emails) {
				em := m.emails[m.eIdx]
				if em.ThreadHasReplies || em.ThreadDepth > 0 {
					m.cycleEmailThreadSubtreeExpansion(em.ID, em.MessageID)
					return m, nil
				}
			}
			if m.searchQuery != "" && !m.inSearch {
				m.inSearch = true
				return m, nil
			}
		}
		return m, nil
	case "esc":
		if m.showError {
			m.showError = false
			m.err = nil
		} else if m.showHelp {
			m.showHelp = false
		} else if m.accountOverlayOpen {
			m.accountOverlayOpen = false
		} else if m.treeOpen {
			m.treeOpen = false
		} else if m.mode == ModeDetail {
			m.mode = ModeIndex
		} else if m.mode == ModeIndex {
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.rebuildVisibleEmails()
				if len(m.emails) > 0 {
					m.eIdx = min(m.eIdx, len(m.emails)-1)
					m.selectedID = m.emails[m.eIdx].ID
				}
				return m, nil
			}
			if m.splitPreview {
				m.splitPreview = false
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
		return m, nil
	default:
		if m.showHelp {
			return m, nil
		}
		if m.treeOpen {
			return m.kFolder(k)
		}
		switch m.mode {
		case ModeIndex:
			return m.kIndex(k)
		case ModeDetail:
			return m.kDetail(k)
		}
		return m, nil
	}
}

func (m *tuiModel) pageStep() int {
	if m.height > 2 {
		return m.height - 2
	}
	return 10
}

func (m *tuiModel) kFolder(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inFolderSearch {
		return m.kFolderSearch(k)
	}
	s := k.String()
	switch {
	case k.Type == tea.KeyUp || s == "k":
		m.cursor--
		if m.cursor < 0 {
			m.cursor = 0
		}
	case k.Type == tea.KeyDown || s == "j":
		m.cursor++
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
	case s == "pageup" || s == "pgup":
		m.cursor -= m.pageStep()
		if m.cursor < 0 {
			m.cursor = 0
		}
	case s == "pagedown" || s == "pgdown" || s == "pgdn":
		m.cursor += m.pageStep()
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
	case s == "home":
		m.cursor = 0
	case s == "end":
		m.cursor = len(m.entries) - 1
	case k.Type == tea.KeyRight || s == "+":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			if n := m.entries[m.cursor]; n.Node != nil {
				n.Node.Expanded = true
			}
		}
		m.buildEntries()
	case k.Type == tea.KeyLeft || s == "-":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			if n := m.entries[m.cursor]; n.Node != nil {
				n.Node.Expanded = false
			}
		}
		m.buildEntries()
	case s == " ":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			if n := m.entries[m.cursor]; n.Node != nil {
				n.Node.Expanded = !n.Node.Expanded
			}
		}
		m.buildEntries()
	case k.Type == tea.KeyTab || s == "tab":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			if n := m.entries[m.cursor]; n.Node != nil {
				targetName := n.Node.FullName
				m.cycleTreeSubtreeExpansion(n.Node)
				m.buildEntries()
				for i, e := range m.entries {
					if e.Node != nil && e.Node.FullName == targetName {
						m.cursor = i
						break
					}
				}
			}
		}
	case k.Type == tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			n := m.entries[m.cursor]
			if n.Node != nil {
				m.currentF = n.Node.FullName
				m.treeOpen = false
				m.mode = ModeIndex
				m.loadCachedEmailsForFolder(n.Node.FullName)
				if len(m.emails) == 0 {
					m.showLoad = "Downloading..."
				} else {
					m.showLoad = ""
				}
				return m, m.loadEmailsCmd(n.Node.FullName)
			}
		}
	case s == "/":
		m.inFolderSearch = true
		m.folderSearch = ""
		m.buildEntries()
	case s == "f" || s == "esc":
		m.treeOpen = false
	default:
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) kFolderSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch {
	case k.Type == tea.KeyEsc:
		m.inFolderSearch = false
		m.folderSearch = ""
		m.buildEntries()
	case k.Type == tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			n := m.entries[m.cursor]
			if n.Node != nil {
				m.currentF = n.Node.FullName
				m.treeOpen = false
				m.inFolderSearch = false
				m.folderSearch = ""
				m.mode = ModeIndex
				m.loadCachedEmailsForFolder(n.Node.FullName)
				if len(m.emails) == 0 {
					m.showLoad = "Downloading..."
				} else {
					m.showLoad = ""
				}
				return m, m.loadEmailsCmd(n.Node.FullName)
			}
		}
	case k.Type == tea.KeyUp:
		m.cursor--
		if m.cursor < 0 {
			m.cursor = 0
		}
	case k.Type == tea.KeyDown:
		m.cursor++
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
	case k.Type == tea.KeyBackspace || s == "backspace" || s == "ctrl+h":
		if len(m.folderSearch) > 0 {
			m.folderSearch = m.folderSearch[:len(m.folderSearch)-1]
			m.buildEntries()
		}
	case k.Type == tea.KeySpace:
		m.folderSearch += " "
		m.buildEntries()
	default:
		if len(k.Runes) > 0 {
			m.folderSearch += string(k.Runes)
			m.buildEntries()
		}
	}
	return m, nil
}

func (m *tuiModel) kIndex(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.folderJustChanged = false
	s := k.String()
	switch {
	case m.splitPreview && (k.Type == tea.KeyUp || s == "up"):
		m.vp.ScrollUp(1)
	case m.splitPreview && (k.Type == tea.KeyDown || s == "down"):
		m.vp.ScrollDown(1)
	case m.splitPreview && (s == "pageup" || s == "pgup"):
		m.vp.ScrollUp(m.vp.Height)
	case m.splitPreview && (s == "pagedown" || s == "pgdown" || s == "pgdn"):
		m.vp.ScrollDown(m.vp.Height)
	case m.splitPreview && (k.Type == tea.KeyLeft || s == "left" || s == "k"):
		if m.eIdx > 0 {
			m.eIdx--
			m.selectedID = m.emails[m.eIdx].ID
			m.adjustScrollOffset()
			m.detailVpDirty = true
			m.vp.GotoTop()
			var cmds []tea.Cmd
			if !m.emails[m.eIdx].IsRead {
				m.emails[m.eIdx].IsRead = true
				m.isRead[m.emails[m.eIdx].ID] = true
				if cmd := m.markAsReadCmd(m.emails[m.eIdx].ID); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.detailH = ""
			m.detail = "(Loading preview...)"
			cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
			return m, tea.Batch(cmds...)
		}
	case m.splitPreview && (k.Type == tea.KeyRight || s == "right" || s == "j"):
		if m.eIdx < len(m.emails)-1 {
			m.eIdx++
			m.selectedID = m.emails[m.eIdx].ID
			m.adjustScrollOffset()
			m.detailVpDirty = true
			m.vp.GotoTop()
			var cmds []tea.Cmd
			if !m.emails[m.eIdx].IsRead {
				m.emails[m.eIdx].IsRead = true
				m.isRead[m.emails[m.eIdx].ID] = true
				if cmd := m.markAsReadCmd(m.emails[m.eIdx].ID); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.detailH = ""
			m.detail = "(Loading preview...)"
			cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
			return m, tea.Batch(cmds...)
		}
	case !m.splitPreview && (k.Type == tea.KeyUp || s == "k" || s == "up"):
		m.eIdx--
		if m.eIdx < 0 {
			m.eIdx = 0
		}
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			m.selectedID = m.emails[m.eIdx].ID
		}
	case !m.splitPreview && (k.Type == tea.KeyDown || s == "j" || s == "down"):
		m.eIdx++
		if m.eIdx >= len(m.emails) {
			m.eIdx = len(m.emails) - 1
		}
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			m.selectedID = m.emails[m.eIdx].ID
		}
	case !m.splitPreview && (s == "pageup" || s == "pgup"):
		m.eIdx -= m.pageStep()
		if m.eIdx < 0 {
			m.eIdx = 0
		}
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			m.selectedID = m.emails[m.eIdx].ID
		}
	case !m.splitPreview && (s == "pagedown" || s == "pgdown" || s == "pgdn"):
		m.eIdx += m.pageStep()
		if m.eIdx >= len(m.emails) {
			m.eIdx = len(m.emails) - 1
		}
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			m.selectedID = m.emails[m.eIdx].ID
		}
	case s == "home":
		m.eIdx = 0
		if len(m.emails) > 0 {
			m.selectedID = m.emails[0].ID
		}
		if m.splitPreview {
			m.vp.GotoTop()
		}
	case s == "end":
		if len(m.emails) > 0 {
			m.eIdx = len(m.emails) - 1
			m.selectedID = m.emails[m.eIdx].ID
		}
		if m.splitPreview {
			m.vp.GotoBottom()
		}
	case k.Type == tea.KeyEnter || s == "e":
		return m.kIndexOpenDetail(k)
	case s == " ":
		if len(m.emails) == 0 {
			return m, nil
		}
		m.splitPreview = !m.splitPreview
		if m.splitPreview && m.eIdx >= 0 && m.eIdx < len(m.emails) {
			m.selectedID = m.emails[m.eIdx].ID
			m.detailVpDirty = true
			m.vp.GotoTop()
			var cmds []tea.Cmd
			if !m.emails[m.eIdx].IsRead {
				m.emails[m.eIdx].IsRead = true
				m.isRead[m.emails[m.eIdx].ID] = true
				if cmd := m.markAsReadCmd(m.emails[m.eIdx].ID); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.detailH = ""
			m.detail = "(Loading preview...)"
			cmds = append(cmds, m.loadDetailCmd(m.selectedID, m.eIdx))
			return m, tea.Batch(cmds...)
		}
		return m, nil
	default:
		return m.kIndexActions(k)
	}
	return m, nil
}
