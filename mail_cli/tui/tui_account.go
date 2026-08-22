package tui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/cache"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/label"
	"mail_cli/mailclient"
	"mail_cli/uicommon"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) getAccounts() []cfg_acc.AccountConfig {
	if m.Config() != nil && len(m.Config().Accounts) > 0 {
		return m.Config().Accounts
	}
	if m.cfg != nil && len(m.cfg.Accounts) > 0 {
		return m.cfg.Accounts
	}
	return nil
}

func (m *tuiModel) currentAccountIndex() int {
	accounts := m.getAccounts()
	if len(accounts) == 0 {
		return 0
	}
	sel := ""
	if m.Config() != nil {
		sel = m.Config().SelectedAccount
	} else if m.cfg != nil {
		sel = m.cfg.SelectedAccount
	}
	if sel != "" {
		for i, a := range accounts {
			if strings.EqualFold(a.Name, sel) {
				return i
			}
		}
	}
	return 0
}

func (m *tuiModel) openAccountOverlay() {
	accounts := m.getAccounts()
	if len(accounts) == 0 {
		return
	}
	m.accountOverlayOpen = true
	m.treeOpen = false
	m.showHelp = false
	m.showDiag = false
	m.accountCursor = m.currentAccountIndex()
}

func (m *tuiModel) closeAccountOverlay() {
	m.accountOverlayOpen = false
}

func (m *tuiModel) prevAccount() (tea.Model, tea.Cmd) {
	accounts := m.getAccounts()
	if len(accounts) <= 1 {
		return m, nil
	}
	cur := m.currentAccountIndex()
	prev := (cur - 1 + len(accounts)) % len(accounts)
	return m.switchAccount(accounts[prev])
}

func (m *tuiModel) nextAccount() (tea.Model, tea.Cmd) {
	accounts := m.getAccounts()
	if len(accounts) <= 1 {
		return m, nil
	}
	cur := m.currentAccountIndex()
	next := (cur + 1) % len(accounts)
	return m.switchAccount(accounts[next])
}

func (m *tuiModel) switchAccountByIndex(idx int) (tea.Model, tea.Cmd) {
	accounts := m.getAccounts()
	if len(accounts) == 0 {
		m.accountOverlayOpen = false
		return m, nil
	}
	if idx < 0 || idx >= len(accounts) {
		m.accountOverlayOpen = false
		return m, nil
	}
	m.accountOverlayOpen = false
	return m.switchAccount(accounts[idx])
}

func (m *tuiModel) switchAccount(acc cfg_acc.AccountConfig) (tea.Model, tea.Cmd) {
	slog.Info("switchAccount called", slog.String("accountName", acc.Name), slog.String("displayName", acc.GetDisplayName()))

	if m.Config() != nil {
		m.Config().SelectedAccount = acc.Name
	}
	if m.cfg != nil {
		m.cfg.SelectedAccount = acc.Name
	}

	var newClient mailclient.MailClient
	var err error

	var cfgToUse *cfg_g.Config
	if m.cfg != nil {
		cfgToUse = m.cfg
	} else if m.Config() != nil {
		cfgToUse = m.Config()
	}

	if m.bk != nil && m.bk.GetClientForAccount != nil && cfgToUse != nil {
		newClient, err = m.bk.GetClientForAccount(cfgToUse, acc)
	} else if m.bk != nil && m.bk.GetClientForAccountIndex != nil && cfgToUse != nil {
		newClient, err = m.bk.GetClientForAccountIndex(cfgToUse, m.currentAccountIndex()+1)
	} else if m.bk != nil && m.bk.GetClientForSelected != nil && cfgToUse != nil {
		newClient, err = m.bk.GetClientForSelected(cfgToUse)
	}

	if err != nil {
		m.err = err
		m.showError = true
		return m, nil
	}

	if newClient != nil {
		m.client = newClient
	}

	if m.client != nil && m.client.Config() != nil {
		m.isRead = cache.LoadReadState(m.client.Config().DownloadDir)
		m.isReplied = cache.LoadRepliedState(m.client.Config().DownloadDir)
	}
	if m.isRead == nil {
		m.isRead = make(map[string]bool)
	}
	if m.isReplied == nil {
		m.isReplied = make(map[string]bool)
	}

	m.pendingEmails = make(map[string]PendingOpType)
	m.expandedThreads = make(map[string]bool)
	m.rawEmails = nil
	m.emails = nil
	m.eIdx = 0
	m.selectedID = ""
	m.indexStart = 0
	m.mode = ModeIndex
	m.detail = ""
	m.detailH = ""
	m.attachments = nil
	m.attachmentDrawer = false
	m.searchQuery = ""
	m.inSearch = false
	m.folderSearch = ""
	m.inFolderSearch = false
	m.treeOpen = false
	m.accountOverlayOpen = false
	m.showHelp = false
	m.showError = false
	m.err = nil
	m.showDiag = false
	m.menuOpen = false

	var cachedFolders []label.LabelItem
	if m.client != nil && m.client.Config() != nil {
		dir := m.client.Config().DownloadDir
		if dir != "" {
			cachePath := filepath.Join(dir, "labels_cache.json")
			if data, err := os.ReadFile(cachePath); err == nil {
				_ = json.Unmarshal(data, &cachedFolders)
			}
		}
	}

	targetFolder := "INBOX"
	if m.client != nil {
		targetFolder = m.client.InboxFolder()
	}

	if len(cachedFolders) > 0 {
		m.globalFolders = cachedFolders
		m.globalTree = uicommon.BuildLabelTree(cachedFolders)
		m.folders = cachedFolders
		m.tree = uicommon.BuildLabelTree(cachedFolders)
		m.buildEntries()
		resolved := label.ResolveFolder(cachedFolders, targetFolder, "INBOX")
		m.currentF = resolved
	} else {
		m.globalFolders = nil
		m.globalTree = nil
		m.folders = nil
		m.tree = nil
		m.entries = nil
		m.currentF = targetFolder
	}

	m.loadCachedEmailsForFolder(m.currentF)

	var cmds []tea.Cmd
	if m.client != nil {
		cmds = append(cmds, m.bgFetchFoldersCmd())
		if m.currentF != "" {
			if len(m.emails) == 0 {
				m.showLoad = "Downloading..."
			} else {
				m.showLoad = ""
			}
			cmds = append(cmds, m.loadEmailsCmd(m.currentF))
		}
	}
	cmds = append(cmds, tea.ClearScreen)

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) kAccountOverlay(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	accounts := m.getAccounts()
	s := k.String()
	switch {
	case s == "esc" || s == "q" || s == "a" || s == "A":
		m.accountOverlayOpen = false
		return m, nil
	case s == "up" || s == "k":
		if len(accounts) > 0 {
			m.accountCursor--
			if m.accountCursor < 0 {
				m.accountCursor = len(accounts) - 1
			}
		}
	case s == "down" || s == "j" || s == "tab":
		if len(accounts) > 0 {
			m.accountCursor++
			if m.accountCursor >= len(accounts) {
				m.accountCursor = 0
			}
		}
	case s == "enter" || s == " ":
		return m.switchAccountByIndex(m.accountCursor)
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		num := int(s[0] - '0')
		if num-1 < len(accounts) {
			return m.switchAccountByIndex(num - 1)
		}
	}
	return m, nil
}

func renderAccountOverlay(m *tuiModel) string {
	t := m.theme.Theme()
	accounts := m.getAccounts()
	curIdx := m.currentAccountIndex()

	var lines []string
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Get(uicommon.ColorHighlightFg)))

	lines = append(lines, headerStyle.Render("       SWITCH ACCOUNT"))
	lines = append(lines, "")

	if len(accounts) == 0 {
		lines = append(lines, "  No configured accounts found.")
	} else {
		for i, acc := range accounts {
			numStr := fmt.Sprintf("[%d]", i+1)
			name := acc.GetDisplayName()
			accType := strings.ToUpper(acc.Type)
			if accType == "" {
				accType = "MAIL"
			}
			user := acc.Username

			var prefix string
			var rowStyle lipgloss.Style
			if i == m.accountCursor {
				prefix = "> "
				rowStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color(t.Get(uicommon.ColorHighlightFg)))
			} else {
				prefix = "  "
				rowStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(t.Get(uicommon.ColorFg)))
			}

			typeBadge := lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Get(uicommon.ColorDate))).
				Render(fmt.Sprintf("[%s]", accType))

			roBadge := ""
			if acc.ReadOnly || (m.Config() != nil && m.Config().ReadOnly) || (m.cfg != nil && m.cfg.ReadOnly) {
				roBadge = " " + lipgloss.NewStyle().
					Foreground(lipgloss.Color(t.Get(uicommon.ColorDate))).
					Render("[RO]")
			}

			activeBullet := ""
			if i == curIdx {
				activeBullet = " " + lipgloss.NewStyle().
					Foreground(lipgloss.Color(t.Get(uicommon.ColorHighlightFg))).
					Render("●")
			}

			var desc string
			if user != "" && user != name {
				desc = fmt.Sprintf("%-4s %-14s %s%s  (%s)%s", numStr, name, typeBadge, roBadge, user, activeBullet)
			} else {
				desc = fmt.Sprintf("%-4s %-14s %s%s%s", numStr, name, typeBadge, roBadge, activeBullet)
			}

			row := prefix + rowStyle.Render(desc)
			lines = append(lines, row)
		}
	}

	lines = append(lines, "")
	dialogWidth := min(64, max(42, m.width-4))
	innerWidth := dialogWidth - 4 // accounting for Padding(1, 2)

	helpLeft := "Enter/1-9: Select  │  j/k: Nav  │  Esc: Cancel"
	helpRight := "●=active"
	if innerWidth < len(helpLeft)+len(helpRight)+2 {
		helpLeft = "1-9: Select  │  j/k: Nav  │  Esc: Cancel"
	}

	var footer string
	if gap := innerWidth - len(helpLeft) - len(helpRight); gap > 0 {
		footer = helpLeft + strings.Repeat(" ", gap) + helpRight
	} else {
		footer = helpLeft + "  " + helpRight
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Get(uicommon.ColorDate)))
	lines = append(lines, helpStyle.Render(footer))

	content := strings.Join(lines, "\n")
	dialog := lipgloss.NewStyle().
		Width(dialogWidth).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Get(uicommon.ColorBorder))).
		Padding(1, 2).
		Render(content)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(max(10, m.height)).
		Align(lipgloss.Center, lipgloss.Center).
		Render(dialog)
}
