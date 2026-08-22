package tui

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mail_cli/cache"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/label"
	"mail_cli/mailclient"
	"mail_cli/uicommon"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const ModeIndex int = 0
const ModeDetail = 1

// Header column widths for email row rendering
const (
	HeaderColIndexWidth   = 4  // Index column width
	HeaderColDateWidth    = 8  // Date column width
	HeaderColFromWidth    = 20 // From column width
	HeaderColSubjectWidth = 2  // Subject column width
	HeaderColSpamWidth    = 2  // Spam indicator width
	HeaderColReadWidth    = 2  // Read indicator width
	HeaderColPadding      = 2  // Padding between columns
)

const (
	HeaderColTotal = HeaderColIndexWidth + HeaderColDateWidth + HeaderColFromWidth + HeaderColSubjectWidth + HeaderColSpamWidth + HeaderColReadWidth + (HeaderColPadding * 5)
)

type PendingOpType int

const (
	PendingSpam    PendingOpType = 1
	PendingArchive PendingOpType = 2
	PendingDelete  PendingOpType = 3
	PendingUnspam  PendingOpType = 4
)

type Backend struct {
	RunPreFlightCheck        func(cfg *cfg_g.Config) error
	GetClientForSelected     func(cfg *cfg_g.Config) (mailclient.MailClient, error)
	GetClientForAccount      func(cfg *cfg_g.Config, acc cfg_acc.AccountConfig) (mailclient.MailClient, error)
	GetClientForAccountIndex func(cfg *cfg_g.Config, index int) (mailclient.MailClient, error)
	InitTuiLogger            func(configDir string)
	CloseTuiLogger           func()
	ResolveArchiveTarget     func(client mailclient.MailClient) (string, error)
	ResolveTrashTarget       func(client mailclient.MailClient) (string, error)
	SanitizeLabelForCache    func(label string) string
}

type timeTickMsg struct{}

type errMsg struct {
	err error
}

type foldersLoadedMsg struct {
	folders []label.LabelItem
}

type emailsLoadedMsg struct {
	emails      []uicommon.FolderEmail
	classifyIDs []string
}

type detailMsg struct {
	headers     string
	body        string
	attachments []email.Attachment
}

type doneMsg struct {
	id string
}

type spamDoneMsg struct {
	id string
}

type archiveDoneMsg struct {
	id string
}

type deleteDoneMsg struct {
	id string
}

type unspamDoneMsg struct {
	id string
}

type classifyDoneMsg struct{}

func (m *tuiModel) isPending(id string) bool {
	_, ok := m.pendingEmails[id]
	return ok
}

func (m *tuiModel) addPending(id string, op PendingOpType) {
	m.pendingEmails[id] = op
}

func (m *tuiModel) removePending(id string) {
	delete(m.pendingEmails, id)
}

type tuiModel struct {
	mode               int
	folders            []label.LabelItem
	emails             []uicommon.FolderEmail
	rawEmails          []uicommon.FolderEmail
	expandedThreads    map[string]bool
	eIdx               int
	tree               []*uicommon.LabelTreeNode
	entries            []uicommon.TreeEntry
	cursor             int
	currentF           string
	treeOpen           bool
	showHelp           bool
	showLoad           string
	vp                 viewport.Model
	done               bool
	width              int
	height             int
	lpr                string
	selectedID         string
	client             mailclient.MailClient
	pendingEmails      map[string]PendingOpType
	detailH            string
	detail             string
	selectedRow        uicommon.ColoredString
	detailVpDirty      bool
	theme              *uicommon.ThemeManager
	lastTime           time.Time
	folderSearch       string
	inFolderSearch     bool
	inSearch           bool
	searchQuery        string
	fuzzySearch        bool
	fuzzyGlobal        bool
	savedRawEmails     []uicommon.FolderEmail
	savedCurrentF      string
	detailSearch       string
	inDetailSearch     bool
	splitPreview       bool
	indexStart         int
	err                error
	globalFolders      []label.LabelItem
	globalTree         []*uicommon.LabelTreeNode
	showGlobalTree     bool
	folderJustChanged  bool
	isRead             map[string]bool
	isReplied          map[string]bool
	bk                 *Backend
	showError          bool
	showDiag           bool
	diagVp             viewport.Model
	attachments        []email.Attachment
	attachmentDrawer   bool
	attCursor          int
	saveStatus         string
	saveStatusTime     time.Time
	menuOpen           bool
	menuCursor         int
	menuItems          []string
	accountOverlayOpen bool
	accountCursor      int
	cfg                *cfg_g.Config
	confirmSend        bool
	confirmSendBytes   []byte
	replyTargetID      string
}

func TransferExpandedStates(oldNodes, newNodes []*uicommon.LabelTreeNode) {
	oldStates := make(map[string]bool)
	var collect func(nodes []*uicommon.LabelTreeNode)
	collect = func(nodes []*uicommon.LabelTreeNode) {
		for _, n := range nodes {
			oldStates[n.FullName] = n.Expanded
			collect(n.Children)
		}
	}
	collect(oldNodes)
	var apply func(nodes []*uicommon.LabelTreeNode)
	apply = func(nodes []*uicommon.LabelTreeNode) {
		for _, n := range nodes {
			if exp, ok := oldStates[n.FullName]; ok {
				n.Expanded = exp
			}
			apply(n.Children)
		}
	}
	apply(newNodes)
}

func NewTuiModel(cl mailclient.MailClient, lp string, bk *Backend) *tuiModel {
	m := &tuiModel{
		mode:            ModeIndex,
		client:          cl,
		lpr:             lp,
		theme:           uicommon.NewThemeManager(),
		expandedThreads: make(map[string]bool),
		pendingEmails:   make(map[string]PendingOpType),
		showGlobalTree:  true,
		bk:              bk,
		menuItems:       []string{"Move to Spam", "Switch Account", "Exit"},
	}
	if cl != nil && cl.Config() != nil {
		m.isRead = cache.LoadReadState(cl.Config().DownloadDir)
		m.isReplied = cache.LoadRepliedState(cl.Config().DownloadDir)
	}
	if m.isRead == nil {
		m.isRead = make(map[string]bool)
	}
	if m.isReplied == nil {
		m.isReplied = make(map[string]bool)
	}
	m.vp = viewport.New(70, 20)
	m.diagVp = viewport.New(70, 20)
	targetFolder := "INBOX"
	if cl != nil {
		targetFolder = cl.InboxFolder()
	}
	if lp != "" {
		targetFolder = lp
	}
	slog.Info("newTuiModel initializing", slog.String("labelPrefix", lp), slog.String("targetFolderDefault", targetFolder))

	var cachedFolders []label.LabelItem
	if cl != nil && cl.Config() != nil {
		dir := cl.Config().DownloadDir
		if dir != "" {
			cachePath := filepath.Join(dir, "labels_cache.json")
			slog.Info("newTuiModel loading cached folders", slog.String("path", cachePath))
			if data, err := os.ReadFile(cachePath); err == nil {
				if err := json.Unmarshal(data, &cachedFolders); err != nil {
					slog.Error("newTuiModel failed to unmarshal cached folders", slog.Any("error", err))
				} else {
					var names []string
					for _, f := range cachedFolders {
						names = append(names, f.FullName)
					}
					slog.Info("newTuiModel loaded cached folders successfully", slog.Int("count", len(cachedFolders)), slog.Any("folderNames", names))
				}
			} else {
				slog.Warn("newTuiModel failed to read cached folders file", slog.Any("error", err))
			}
		}
	}
	if len(cachedFolders) > 0 {
		m.globalFolders = cachedFolders
		m.globalTree = uicommon.BuildLabelTree(cachedFolders)
		if lp != "" {
			lower := strings.ToLower(lp)
			var filtered []label.LabelItem
			for _, f := range cachedFolders {
				if strings.Contains(strings.ToLower(f.FullName), lower) {
					filtered = append(filtered, f)
				}
			}
			slog.Info("newTuiModel filtered cached folders by prefix", slog.Int("filteredCount", len(filtered)), slog.String("prefix", lp))
			cachedFolders = filtered
		}
	}
	if len(cachedFolders) > 0 {
		m.folders = cachedFolders
		m.tree = uicommon.BuildLabelTree(cachedFolders)
		m.buildEntries()
		resolved := label.ResolveFolder(cachedFolders, targetFolder, "INBOX")
		slog.Info("newTuiModel resolved target folder from cache", slog.String("resolvedFolder", resolved), slog.Bool("found", resolved != ""))
		m.currentF = resolved
	} else {
		slog.Warn("newTuiModel found no cached folders; defaulting directly to INBOX")
		m.currentF = targetFolder
	}
	m.loadCachedEmailsForFolder(m.currentF)
	return m
}

func (m *tuiModel) View() string {
	if m.done {
		return ""
	}
	if m.confirmSend {
		return renderConfirmSendDialog(m)
	}
	if m.showDiag {
		return renderDiag(m)
	}
	if m.showHelp && m.treeOpen {
		m.treeOpen = false
	}
	if m.showHelp && m.accountOverlayOpen {
		m.accountOverlayOpen = false
	}
	if m.showHelp {
		helpContent := renderHelp(m)
		if m.showError {
			helpContent += "\n" + renderErrorBanner(m)
		}
		result := renderGrayoutOverlay(m.width, m.height, helpContent)
		return result
	}

	if m.accountOverlayOpen {
		result := renderAccountOverlay(m)
		if m.showError {
			result += "\n" + renderErrorBanner(m)
		}
		return result
	}

	if m.treeOpen && len(m.entries) > 0 {
		result := renderFolderOverlay(m, renderFolderTree(m))
		if m.showError {
			result += "\n" + renderErrorBanner(m)
		}
		return result
	}
	if m.showLoad != "" {
		if m.showLoad == "Downloading..." {
			result := renderDownloadingDialog(m.width, m.height)
			if m.showError {
				result += "\n" + renderErrorBanner(m)
			}
			return result
		}
		if m.showLoad == "Unspamming..." {
			result := renderUnspammingDialog(m.width, m.height)
			if m.showError {
				result += "\n" + renderErrorBanner(m)
			}
			return result
		}
		ol := lipgloss.NewStyle().Width(m.width).Height(max(5, m.height/2)).
			Align(lipgloss.Center, lipgloss.Center).Bold(true).Render(m.showLoad)
		if m.showError {
			ol += "\n" + renderErrorBanner(m)
		}
		return ol
	}
	topBar := renderTopBar(m)
	if m.mode == ModeDetail {
		var selectedLine string
		width := m.width
		if m.menuOpen {
			width = m.width - 19
		}
		if m.eIdx >= 0 && m.eIdx < len(m.emails) {
			sjw := width - HeaderColTotal
			m.selectedRow = uicommon.RenderEmailRow(m.emails[m.eIdx], width, sjw, m.eIdx+1, m.theme.Theme(), true, m.isPending(m.emails[m.eIdx].ID))
			selectedLine = m.selectedRow.Render(m.theme.Theme())
		}
		vh := m.height - 3
		if m.showError {
			vh--
		}
		if m.attachmentDrawer {
			vh -= (len(m.attachments) + 3)
		}
		vh = max(5, vh)
		vw := max(20, width-2)
		if m.detailVpDirty || m.vp.Width != vw || m.vp.Height != vh {
			content := m.buildDetailContent(vw)
			m.vp.SetContent(content)
			m.vp.Width = vw
			m.vp.Height = vh
			m.detailVpDirty = false
		}
		viewportStr := m.vp.View()

		var bodyParts []string
		if selectedLine != "" {
			bodyParts = append(bodyParts, selectedLine)
		}
		bodyParts = append(bodyParts, viewportStr)
		if m.attachmentDrawer {
			bodyParts = append(bodyParts, renderAttachmentDrawer(m))
		}
		bodyStr := strings.Join(bodyParts, "\n")
		if m.menuOpen {
			bodyStr = lipgloss.JoinHorizontal(lipgloss.Top, renderMenu(m), bodyStr)
		}

		var parts []string
		if topBar != "" {
			parts = append(parts, topBar)
		}
		parts = append(parts, bodyStr)
		if m.showError {
			parts = append(parts, renderErrorBanner(m))
		}
		return strings.Join(parts, "\n")
	}

	var parts []string
	if topBar != "" {
		parts = append(parts, topBar)
	}
	var bodyStr string
	if m.splitPreview && len(m.emails) > 0 {
		bodyStr = renderSplitView(m)
	} else {
		bodyStr = renderIndex(m)
	}
	slog.Info("View renderIndex returned",
		slog.Int("width", m.width),
		slog.Int("height", m.height),
		slog.Int("len", len(bodyStr)),
		slog.String("content", bodyStr),
	)
	if m.menuOpen {
		bodyStr = lipgloss.JoinHorizontal(lipgloss.Top, renderMenu(m), bodyStr)
	}
	parts = append(parts, bodyStr)
	if m.showError {
		parts = append(parts, renderErrorBanner(m))
	}
	return strings.Join(parts, "\n")
}
