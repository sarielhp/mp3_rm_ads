package tui

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	mailLabel "mail_cli/label"
	"mail_cli/mailclient"
	"mail_cli/uicommon"
)

func createTestEmailDir(t *testing.T) string {
	return t.TempDir()
}

func TestBuildFolderTree(t *testing.T) {
	items := []mailLabel.LabelItem{
		{Name: "INBOX", FullName: "INBOX"},
		{Name: "Work", FullName: "Work"},
		{Name: "Work/Projects", FullName: "Work/Projects"},
		{Name: "Work/Projects/Beta", FullName: "Work/Projects/Beta"},
		{Name: "Personal", FullName: "Personal"},
	}
	tree := uicommon.BuildLabelTree(items)
	if len(tree) != 3 {
		t.Errorf("expected 3 root nodes, got %d", len(tree))
	}
}

func TestFlattenTree(t *testing.T) {
	projects := &uicommon.LabelTreeNode{Name: "Projects", FullName: "Work/Projects"}
	projects.Expanded = true
	beta := &uicommon.LabelTreeNode{Name: "Beta", FullName: "Work/Projects/Beta"}
	projects.Children = []*uicommon.LabelTreeNode{beta}

	work := &uicommon.LabelTreeNode{Name: "Work", FullName: "Work"}
	work.Children = []*uicommon.LabelTreeNode{projects}
	work.Expanded = true

	inbox := &uicommon.LabelTreeNode{Name: "INBOX", FullName: "INBOX"}
	nodes := []*uicommon.LabelTreeNode{inbox, work}
	entries := uicommon.FlattenTree(nodes)

	if len(entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(entries))
	}
}

func TestFlattenTreePrefixes(t *testing.T) {
	inbox := &uicommon.LabelTreeNode{Name: "INBOX", FullName: "INBOX"}

	projects := &uicommon.LabelTreeNode{Name: "Projects", FullName: "Work/Projects"}
	projects.Expanded = true
	beta := &uicommon.LabelTreeNode{Name: "Beta", FullName: "Work/Projects/Beta"}
	projects.Children = []*uicommon.LabelTreeNode{beta}

	work := &uicommon.LabelTreeNode{Name: "Work", FullName: "Work"}
	work.Children = []*uicommon.LabelTreeNode{projects}
	work.Expanded = true

	nodes := []*uicommon.LabelTreeNode{inbox, work}
	entries := uicommon.FlattenTree(nodes)

	if entries[0].Prefix != "" {
		t.Errorf("expected entries[0] prefix to be empty, got %q", entries[0].Prefix)
	}
	if entries[1].Prefix != "" {
		t.Errorf("expected entries[1] prefix to be empty, got %q", entries[1].Prefix)
	}
	if entries[2].Prefix != "└── " {
		t.Errorf("expected entries[2] prefix to be '└── ', got %q", entries[2].Prefix)
	}
	if entries[3].Prefix != "    └── " {
		t.Errorf("expected entries[3] prefix to be '    └── ', got %q", entries[3].Prefix)
	}
}

func TestFlattenTreeCollapsed(t *testing.T) {
	work := &uicommon.LabelTreeNode{Name: "Work", FullName: "Work"}
	work.Children = []*uicommon.LabelTreeNode{{Name: "Child", FullName: "Work/Child"}}
	work.Expanded = false

	nodes := []*uicommon.LabelTreeNode{work}
	entries := uicommon.FlattenTree(nodes)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry when collapsed, got %d", len(entries))
	}
}

func TestTreeEntryDepth(t *testing.T) {
	projects := &uicommon.LabelTreeNode{Name: "Projects", FullName: "Work/Projects"}
	projects.Expanded = true
	deep := &uicommon.LabelTreeNode{Name: "Deep", FullName: "Work/Projects/Deep"}
	projects.Children = []*uicommon.LabelTreeNode{deep}

	work := &uicommon.LabelTreeNode{Name: "Work", FullName: "Work"}
	work.Children = []*uicommon.LabelTreeNode{projects}
	work.Expanded = true

	entries := uicommon.FlattenTree([]*uicommon.LabelTreeNode{work})

	if len(entries) < 3 {
		t.Fatalf("not enough entries: %d", len(entries))
	}
	if entries[0].Depth != 0 {
		t.Errorf("expected depth 0, got %d", entries[0].Depth)
	}
	if entries[1].Depth != 1 {
		t.Errorf("expected depth 1, got %d", entries[1].Depth)
	}
	if entries[2].Depth != 2 {
		t.Errorf("expected depth 2, got %d", entries[2].Depth)
	}
}

func TestDecDecode(t *testing.T) {
	dec := new(mime.WordDecoder)
	encoded := "=?UTF-8?Q?Hello_World?="
	result := uicommon.DecDecode(dec, encoded)
	if result != "Hello World" {
		t.Errorf("DecDecode(%q) = %q, want %q", encoded, result, "Hello World")
	}
}

func TestTuiModelFoldersLoaded(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	folders := []mailLabel.LabelItem{
		{Name: "INBOX", FullName: "INBOX"},
		{Name: "Work", FullName: "Work"},
		{Name: "Work/Projects", FullName: "Work/Projects"},
	}

	res, _ := m.Update(foldersLoadedMsg{folders: folders})
	updatedModel := res.(*tuiModel)
	if len(updatedModel.folders) != 3 {
		t.Errorf("expected 3 folders, got %d", len(updatedModel.folders))
	}
	if len(updatedModel.tree) != 2 {
		t.Errorf("expected 2 root tree nodes, got %d", len(updatedModel.tree))
	}
	if len(updatedModel.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(updatedModel.entries))
	}
}

func TestFolderSearch(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	folders := []mailLabel.LabelItem{
		{Name: "INBOX", FullName: "INBOX"},
		{Name: "Work", FullName: "Work"},
		{Name: "Work/Projects", FullName: "Work/Projects"},
		{Name: "Personal", FullName: "Personal"},
	}

	res, _ := m.Update(foldersLoadedMsg{folders: folders})
	updatedModel := res.(*tuiModel)

	if len(updatedModel.entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(updatedModel.entries))
	}

	updatedModel.inFolderSearch = true
	updatedModel.folderSearch = "proj"
	updatedModel.buildEntries()

	foundProj := false
	for _, e := range updatedModel.entries {
		if strings.Contains(strings.ToLower(e.Node.FullName), "proj") {
			foundProj = true
		}
	}
	if !foundProj {
		t.Error("expected to find folder matching 'proj' in search results")
	}
}

func TestFolderSearchCursorJump(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	folders := []mailLabel.LabelItem{
		{Name: "Work", FullName: "Work"},
		{Name: "Work/Projects", FullName: "Work/Projects"},
		{Name: "Work/Projects/Sub", FullName: "Work/Projects/Sub"},
		{Name: "Other", FullName: "Other"},
	}

	res, _ := m.Update(foldersLoadedMsg{folders: folders})
	updatedModel := res.(*tuiModel)

	updatedModel.inFolderSearch = true
	updatedModel.folderSearch = "work"
	updatedModel.buildEntries()

	if updatedModel.cursor < 0 || updatedModel.cursor >= len(updatedModel.entries) {
		t.Fatalf("expected cursor to be valid, got %d (len: %d)", updatedModel.cursor, len(updatedModel.entries))
	}

	selectedNode := updatedModel.entries[updatedModel.cursor].Node
	if selectedNode.FullName != "Work/Projects/Sub" {
		t.Errorf("expected selected folder to be 'Work/Projects/Sub', got %q", selectedNode.FullName)
	}
}

func TestCountAllDescendants(t *testing.T) {
	grandchild := &uicommon.LabelTreeNode{Name: "Grandchild", FullName: "Work/Projects/Grandchild"}
	projects := &uicommon.LabelTreeNode{Name: "Projects", FullName: "Work/Projects", Children: []*uicommon.LabelTreeNode{grandchild}}
	work := &uicommon.LabelTreeNode{Name: "Work", FullName: "Work", Children: []*uicommon.LabelTreeNode{projects}}

	if count := uicommon.CountAllDescendants(work); count != 2 {
		t.Errorf("expected Work to have 2 descendants, got %d", count)
	}
	if count := uicommon.CountAllDescendants(projects); count != 1 {
		t.Errorf("expected Projects to have 1 descendant, got %d", count)
	}
	if count := uicommon.CountAllDescendants(grandchild); count != 0 {
		t.Errorf("expected Grandchild to have 0 descendants, got %d", count)
	}
}

func TestIndexModeScrolling(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.height = 10

	var emails []uicommon.FolderEmail
	for i := 0; i < 20; i++ {
		emails = append(emails, uicommon.FolderEmail{
			ID:            fmt.Sprintf("id-%d", i),
			Subject:       fmt.Sprintf("Subject %d", i),
			FromEmail:     "a@b.com",
			FormattedDate: " 6/20/26",
		})
	}
	m.emails = emails

	m.adjustScrollOffset()
	renderIndex(m)
	if m.indexStart != 0 {
		t.Errorf("expected indexStart to be 0 at start, got %d", m.indexStart)
	}

	m.eIdx = 8
	m.adjustScrollOffset()
	renderIndex(m)
	if m.indexStart != 1 {
		t.Errorf("expected indexStart to scroll to 1 when eIdx is 8, got %d", m.indexStart)
	}

	m.eIdx = 19
	m.adjustScrollOffset()
	renderIndex(m)
	if m.indexStart != 12 {
		t.Errorf("expected indexStart to be 12 at the end, got %d", m.indexStart)
	}
}

func TestLargeFolderIndexScrollingUp(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.height = 20
	m.width = 80

	var emails []uicommon.FolderEmail
	for i := 0; i < 100; i++ {
		emails = append(emails, uicommon.FolderEmail{
			ID:            fmt.Sprintf("id-%d", i),
			Subject:       fmt.Sprintf("Subject %d", i),
			FromEmail:     "a@b.com",
			FormattedDate: " 6/20/26",
		})
	}
	m.emails = emails

	// Scroll to the bottom
	m.eIdx = 99
	m.adjustScrollOffset()

	// Press Up arrow 50 times
	for i := 0; i < 50; i++ {
		msg := tea.KeyMsg{Type: tea.KeyUp}
		m.Update(msg)
	}

	// Get the rendered view
	viewStr := m.View()
	lines := strings.Split(viewStr, "\n")

	// Check line count: should not exceed m.height!
	if len(lines) > m.height {
		t.Errorf("View returned %d lines, expected at most %d", len(lines), m.height)
	}

	// Check if the top line is the topBar
	if len(lines) > 0 {
		topLine := lines[0]
		if !strings.Contains(topLine, "default") {
			t.Errorf("Top line does not contain topBar content, got: %q", topLine)
		}
	}
}

func TestDetailModeArrowNavigation(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeDetail

	var emails []uicommon.FolderEmail
	for i := 0; i < 3; i++ {
		emails = append(emails, uicommon.FolderEmail{
			ID:            fmt.Sprintf("id-%d", i),
			Subject:       fmt.Sprintf("Subject %d", i),
			FromEmail:     "a@b.com",
			FormattedDate: " 6/20/26",
		})
	}
	m.emails = emails
	m.eIdx = 1
	m.selectedID = emails[1].ID

	{
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		newM := res.(*tuiModel)
		if newM.eIdx != 0 {
			t.Errorf("expected eIdx to be 0 after KeyLeft, got %d", newM.eIdx)
		}
		if newM.selectedID != emails[0].ID {
			t.Errorf("expected selectedID to be %q after KeyLeft, got %q", emails[0].ID, newM.selectedID)
		}
		if newM.detail != "(Loading message...)" {
			t.Errorf("expected detail body to reset, got %q", newM.detail)
		}
	}

	{
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		newM := res.(*tuiModel)
		if newM.eIdx != 0 {
			t.Errorf("expected eIdx to remain 0 after KeyLeft at boundary, got %d", newM.eIdx)
		}
	}

	{
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		newM := res.(*tuiModel)
		if newM.eIdx != 1 {
			t.Errorf("expected eIdx to be 1 after KeyRight, got %d", newM.eIdx)
		}
		if newM.selectedID != emails[1].ID {
			t.Errorf("expected selectedID to be %q after KeyRight, got %q", emails[1].ID, newM.selectedID)
		}
	}

	{
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
		newM := res.(*tuiModel)
		if newM.eIdx != 2 {
			t.Errorf("expected eIdx to be 2, got %d", newM.eIdx)
		}

		res2, _ := newM.Update(tea.KeyMsg{Type: tea.KeyRight})
		newM2 := res2.(*tuiModel)
		if newM2.eIdx != 2 {
			t.Errorf("expected eIdx to remain 2, got %d", newM2.eIdx)
		}
	}
}

func TestLoadCachedEmailsForFolder(t *testing.T) {
	tempDir := createTestEmailDir(t)

	today := time.Now()
	yy, mm, dd := today.Date()

	emlContent := "From: test@example.com\nSubject: Test Subject\nDate: Mon, 22 Jun 2026 12:00:00 +0000\n\nBody text"
	if err := msg.Store(tempDir, "msg123", []byte(emlContent), time.Date(yy, mm, dd, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	if err := label.ReplaceAll(tempDir, "Archive", []string{"msg123"}); err != nil {
		t.Fatal(err)
	}

	cfg := &cfg_g.Config{
		DownloadDir: tempDir,
	}
	cl := &mailclient.MockMailClient{Cfg: cfg}
	m := NewTuiModel(cl, "", nil)

	m.loadCachedEmailsForFolder("Archive")

	if len(m.emails) != 1 {
		t.Fatalf("expected 1 cached email, got %d", len(m.emails))
	}
	if m.emails[0].ID != "msg123" {
		t.Errorf("expected email ID to be 'msg123', got %q", m.emails[0].ID)
	}
	if m.emails[0].Subject != "Test Subject" {
		t.Errorf("expected subject to be 'Test Subject', got %q", m.emails[0].Subject)
	}
}

func TestDefaultFolderResolution(t *testing.T) {
	cfg1 := &cfg_g.Config{
		ReceivedFolder: "received",
	}
	mc1 := &mailclient.MockMailClient{
		Cfg: cfg1,
	}
	m1 := NewTuiModel(mc1, "", nil)
	if m1.currentF != "received" {
		t.Errorf("expected targetFolder to be %q, got %q", "received", m1.currentF)
	}

	cfg2 := &cfg_g.Config{
		ReceivedFolder: "Inbox",
	}
	mc2 := &mailclient.MockMailClient{
		Cfg: cfg2,
	}
	m2 := NewTuiModel(mc2, "", nil)
	if m2.currentF != "Inbox" {
		t.Errorf("expected targetFolder to be %q, got %q", "Inbox", m2.currentF)
	}

	folders := []mailLabel.LabelItem{
		{Name: "Inbox", FullName: "Inbox"},
		{Name: "Work", FullName: "Work"},
	}
	m2.currentF = ""
	res, _ := m2.Update(foldersLoadedMsg{folders: folders})
	updatedModel := res.(*tuiModel)
	if updatedModel.currentF != "Inbox" {
		t.Errorf("expected foldersLoadedMsg fallback to target %q, got %q", "Inbox", updatedModel.currentF)
	}
}

func TestNewNavigationKeys(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.height = 10

	var emails []uicommon.FolderEmail
	for i := 0; i < 20; i++ {
		emails = append(emails, uicommon.FolderEmail{
			ID:        fmt.Sprintf("id-%d", i),
			Subject:   fmt.Sprintf("Subject %d", i),
			FromEmail: "a@b.com",
		})
	}
	m.emails = emails

	{
		m.eIdx = 5
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("home")})
		newM := res.(*tuiModel)
		if newM.eIdx != 0 {
			t.Errorf("expected eIdx to be 0 after 'home', got %d", newM.eIdx)
		}
		if newM.selectedID != emails[0].ID {
			t.Errorf("expected selectedID to be %q after 'home', got %q", emails[0].ID, newM.selectedID)
		}

		res2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("end")})
		newM2 := res2.(*tuiModel)
		if newM2.eIdx != 19 {
			t.Errorf("expected eIdx to be 19 after 'end', got %d", newM2.eIdx)
		}
		if newM2.selectedID != emails[19].ID {
			t.Errorf("expected selectedID to be %q after 'end', got %q", emails[19].ID, newM2.selectedID)
		}
	}

	{
		m.eIdx = 15
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pageup")})
		newM := res.(*tuiModel)
		if newM.eIdx != 7 {
			t.Errorf("expected eIdx to be 7 after 'pageup', got %d", newM.eIdx)
		}

		res2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgdown")})
		newM2 := res2.(*tuiModel)
		if newM2.eIdx != 15 {
			t.Errorf("expected eIdx to be 15 after 'pgdown', got %d", newM2.eIdx)
		}
	}
}

func TestEscapeToQuit(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex

	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	newM := res.(*tuiModel)
	if !newM.done {
		t.Error("expected done to be true after esc in index view")
	}
	if cmd == nil {
		t.Error("expected a tea.Quit command after esc in index view")
	}
}

func TestEmailThreadingAndCollapsing(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Thread Root", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	e1 := uicommon.FolderEmail{ID: "e1", Subject: "Re: Thread Root", MessageID: "<e1>", InReplyTo: "<e0>", FromRaw: "Bob <b@b.com>", EmailDate: baseTime.Add(time.Minute)}
	e2 := uicommon.FolderEmail{ID: "e2", Subject: "Re: Thread Root", MessageID: "<e2>", InReplyTo: "<e1>", FromRaw: "Charlie <c@b.com>", EmailDate: baseTime.Add(2 * time.Minute)}
	e3 := uicommon.FolderEmail{ID: "e3", Subject: "Unrelated Subject", MessageID: "<e3>", FromRaw: "Dave <d@b.com>", EmailDate: baseTime.Add(3 * time.Minute)}

	m.rawEmails = []uicommon.FolderEmail{e0, e1, e2, e3}
	m.rebuildVisibleEmails()

	if len(m.emails) != 2 {
		t.Fatalf("expected 2 visible emails initially (collapsed by default), got %d", len(m.emails))
	}
	if m.emails[0].ID != "e0" || m.emails[1].ID != "e3" {
		t.Errorf("expected visible emails to be e0 and e3, got %s and %s", m.emails[0].ID, m.emails[1].ID)
	}
	if !m.emails[0].ThreadCollapsed {
		t.Error("expected e0 to be collapsed by default")
	}

	m.expandedThreads["<e0>"] = true
	m.expandedThreads["<e1>"] = true
	m.rebuildVisibleEmails()

	if len(m.emails) != 4 {
		t.Fatalf("expected 4 visible emails after expanding e0, got %d", len(m.emails))
	}

	if m.emails[0].ThreadDepth != 0 || !m.emails[0].ThreadHasReplies || m.emails[0].ThreadRepliesCount != 2 {
		t.Errorf("e0 thread properties unexpected: depth=%d, hasReplies=%v, count=%d",
			m.emails[0].ThreadDepth, m.emails[0].ThreadHasReplies, m.emails[0].ThreadRepliesCount)
	}
	if m.emails[1].ThreadDepth != 1 || !m.emails[1].ThreadHasReplies || m.emails[1].ThreadRepliesCount != 1 {
		t.Errorf("e1 thread properties unexpected: depth=%d, hasReplies=%v, count=%d",
			m.emails[1].ThreadDepth, m.emails[1].ThreadHasReplies, m.emails[1].ThreadRepliesCount)
	}
	if m.emails[2].ThreadDepth != 2 || m.emails[2].ThreadHasReplies {
		t.Errorf("e2 thread properties unexpected: depth=%d, hasReplies=%v",
			m.emails[2].ThreadDepth, m.emails[2].ThreadHasReplies)
	}

	if !strings.Contains(m.emails[0].ThreadSenderSummary, "Alice") || !strings.Contains(m.emails[0].ThreadSenderSummary, "Bob") {
		t.Errorf("expected sender summary to contain Alice and Bob, got %q", m.emails[0].ThreadSenderSummary)
	}

	m.expandedThreads["<e0>"] = false
	m.rebuildVisibleEmails()

	if len(m.emails) != 2 {
		t.Fatalf("expected 2 visible emails after collapsing e0, got %d", len(m.emails))
	}

	m.eIdx = 0
	m.selectedID = "e0"
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = res.(*tuiModel)
	if !m.splitPreview {
		t.Error("expected Space key to toggle split preview")
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = res.(*tuiModel)
	if m.splitPreview {
		t.Error("expected second Space key to toggle split preview off")
	}

	m.eIdx = 0
	m.selectedID = "e0"
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(*tuiModel)
	if !m.expandedThreads["<e0>"] {
		t.Error("expected Enter key on collapsed line to expand it")
	}
	if len(m.emails) != 4 {
		t.Errorf("expected 4 visible emails after Enter expand, got %d", len(m.emails))
	}
	if m.mode != ModeIndex {
		t.Errorf("expected mode to remain ModeIndex on expand Enter, got %d", m.mode)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(*tuiModel)
	if m.mode != ModeDetail {
		t.Errorf("expected mode to become ModeDetail on second Enter, got %d", m.mode)
	}
}

func TestNewTuiImprovements(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Thread Root", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	e1 := uicommon.FolderEmail{ID: "e1", Subject: "Re: Thread Root", MessageID: "<e1>", InReplyTo: "<e0>", FromRaw: "Bob <b@b.com>", EmailDate: baseTime.Add(time.Minute)}
	e2 := uicommon.FolderEmail{ID: "e2", Subject: "Re: Thread Root", MessageID: "<e2>", InReplyTo: "<e1>", FromRaw: "Charlie <c@b.com>", EmailDate: baseTime.Add(2 * time.Minute)}
	e3 := uicommon.FolderEmail{ID: "e3", Subject: "Unrelated", MessageID: "<e3>", FromRaw: "Dave <d@b.com>", EmailDate: baseTime.Add(3 * time.Minute)}

	m.rawEmails = []uicommon.FolderEmail{e0, e1, e2, e3}
	m.rebuildVisibleEmails()

	if len(m.emails) != 2 {
		t.Fatalf("expected 2 visible emails, got %d", len(m.emails))
	}
	if m.emails[0].ThreadIndex != 1 || m.emails[1].ThreadIndex != 4 {
		t.Errorf("expected stable indices 1 and 4, got %d and %d", m.emails[0].ThreadIndex, m.emails[1].ThreadIndex)
	}

	m.expandRecursively("<e0>", "e0")
	if !m.expandedThreads["<e0>"] || !m.expandedThreads["<e1>"] {
		t.Error("expected recursive expansion to expand both e0 and e1")
	}

	m.rebuildVisibleEmails()
	if len(m.emails) != 4 {
		t.Fatalf("expected all 4 emails visible after recursive expand, got %d", len(m.emails))
	}

	if m.emails[1].ThreadPrefix != "└── " {
		t.Errorf("expected e1 ThreadPrefix to be '└── ', got %q", m.emails[1].ThreadPrefix)
	}
	if m.emails[2].ThreadPrefix != "  └── " {
		t.Errorf("expected e2 ThreadPrefix to be '  └── ', got %q", m.emails[2].ThreadPrefix)
	}

	m.treeOpen = true
	m.height = 10
	m.entries = []uicommon.TreeEntry{
		{Node: &uicommon.LabelTreeNode{FullName: "1"}},
		{Node: &uicommon.LabelTreeNode{FullName: "2"}},
		{Node: &uicommon.LabelTreeNode{FullName: "3"}},
		{Node: &uicommon.LabelTreeNode{FullName: "4"}},
		{Node: &uicommon.LabelTreeNode{FullName: "5"}},
		{Node: &uicommon.LabelTreeNode{FullName: "6"}},
		{Node: &uicommon.LabelTreeNode{FullName: "7"}},
		{Node: &uicommon.LabelTreeNode{FullName: "8"}},
		{Node: &uicommon.LabelTreeNode{FullName: "9"}},
		{Node: &uicommon.LabelTreeNode{FullName: "10"}},
	}
	m.cursor = 0

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgdown")})
	m = res.(*tuiModel)
	if m.cursor != 8 {
		t.Errorf("expected folder cursor to be 8 after pagedown, got %d", m.cursor)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pageup")})
	m = res.(*tuiModel)
	if m.cursor != 0 {
		t.Errorf("expected folder cursor to be 0 after pageup, got %d", m.cursor)
	}

	m.treeOpen = false
	m.showGlobalTree = false
	m.currentF = "INBOX"

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	m = res.(*tuiModel)
	if !m.treeOpen || !m.showGlobalTree {
		t.Errorf("expected treeOpen=true, showGlobalTree=true after F key, got treeOpen=%v, showGlobalTree=%v", m.treeOpen, m.showGlobalTree)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = res.(*tuiModel)
	if m.treeOpen {
		t.Error("expected treeOpen=false after toggling with f")
	}
}

func TestSpamAndArchiveImmediateRemoval(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Email 1", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	e1 := uicommon.FolderEmail{ID: "e1", Subject: "Email 2", MessageID: "<e1>", FromRaw: "Bob <b@b.com>", EmailDate: baseTime.Add(time.Minute)}
	m.rawEmails = []uicommon.FolderEmail{e0, e1}
	m.rebuildVisibleEmails()

	if len(m.emails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(m.emails))
	}

	m.eIdx = 0
	m.selectedID = "e0"

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = res.(*tuiModel)

	if len(m.emails) != 1 {
		t.Errorf("expected 1 email remaining in memory after pressing 's', got %d", len(m.emails))
	}
	if m.emails[0].ID != "e1" {
		t.Errorf("expected email 'e1' to remain, got '%s'", m.emails[0].ID)
	}
	if m.selectedID != "e1" {
		t.Errorf("expected selection to move to 'e1', got '%s'", m.selectedID)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = res.(*tuiModel)

	if len(m.emails) != 0 {
		t.Errorf("expected 0 emails remaining in memory after pressing 'E', got %d", len(m.emails))
	}
	if m.selectedID != "" {
		t.Errorf("expected selectedID to be empty, got '%s'", m.selectedID)
	}
}

func TestUnspamImmediateRemoval(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex
	m.currentF = "Spam"

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Spam Email 1", MessageID: "<e0>", FromRaw: "Spammer <s@b.com>", EmailDate: baseTime}
	e1 := uicommon.FolderEmail{ID: "e1", Subject: "Spam Email 2", MessageID: "<e1>", FromRaw: "Spammer2 <s2@b.com>", EmailDate: baseTime.Add(time.Minute)}
	m.rawEmails = []uicommon.FolderEmail{e0, e1}
	m.rebuildVisibleEmails()

	if len(m.emails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(m.emails))
	}

	m.eIdx = 0
	m.selectedID = "e0"

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = res.(*tuiModel)

	if len(m.emails) != 1 {
		t.Errorf("expected 1 email remaining in memory after pressing 'U', got %d", len(m.emails))
	}
	if m.emails[0].ID != "e1" {
		t.Errorf("expected email 'e1' to remain, got '%s'", m.emails[0].ID)
	}
	if m.selectedID != "e1" {
		t.Errorf("expected selection to move to 'e1', got '%s'", m.selectedID)
	}

	if m.pendingEmails["e0"] != PendingUnspam {
		t.Errorf("expected 'e0' to be marked as PendingUnspam, got %v", m.pendingEmails["e0"])
	}
}

func TestHelpToggle(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	if m.showHelp {
		t.Error("expected showHelp=false by default")
	}

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = res.(*tuiModel)
	if !m.showHelp {
		t.Error("expected showHelp=true after pressing '?'")
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = res.(*tuiModel)
	if m.showHelp {
		t.Error("expected showHelp=false after pressing '?' again")
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = res.(*tuiModel)
	if !m.showHelp {
		t.Error("expected showHelp=true after pressing 'h'")
	}
}

func TestUnspamInInboxRetained(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex
	m.currentF = "INBOX"

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Email 1", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	m.rawEmails = []uicommon.FolderEmail{e0}
	m.rebuildVisibleEmails()

	if len(m.emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(m.emails))
	}

	m.eIdx = 0
	m.selectedID = "e0"

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = res.(*tuiModel)

	if len(m.emails) != 1 {
		t.Errorf("expected 1 email remaining in memory when unspamming inside INBOX, got %d", len(m.emails))
	}
	if m.emails[0].ID != "e0" {
		t.Errorf("expected email 'e0' to remain in index, got '%s'", m.emails[0].ID)
	}
	if m.pendingEmails["e0"] != PendingUnspam {
		t.Errorf("expected 'e0' to be marked as PendingUnspam, got %v", m.pendingEmails["e0"])
	}
}

func TestMenuNavigation(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex
	m.currentF = "INBOX"

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Email 1", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	m.rawEmails = []uicommon.FolderEmail{e0}
	m.rebuildVisibleEmails()

	if m.menuOpen {
		t.Error("expected menuOpen=false by default")
	}

	// 1. Press 'm' to open the menu
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = res.(*tuiModel)
	if !m.menuOpen {
		t.Error("expected menuOpen=true after pressing 'm'")
	}
	if m.menuCursor != 0 {
		t.Errorf("expected menuCursor=0 initially, got %d", m.menuCursor)
	}

	// 2. Press 'j' to move cursor down to "Exit"
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = res.(*tuiModel)
	if m.menuCursor != 1 {
		t.Errorf("expected menuCursor=1 after pressing 'j', got %d", m.menuCursor)
	}

	// 3. Press 'k' to move cursor back up to "Move to Spam"
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = res.(*tuiModel)
	if m.menuCursor != 0 {
		t.Errorf("expected menuCursor=0 after pressing 'k', got %d", m.menuCursor)
	}

	// 4. Press 'Enter' on "Move to Spam"
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(*tuiModel)
	if m.menuOpen {
		t.Error("expected menuOpen=false after selecting an action")
	}
	if len(m.emails) != 0 {
		t.Errorf("expected email to be removed from index, got %d remaining", len(m.emails))
	}
	if m.pendingEmails["e0"] != PendingSpam {
		t.Errorf("expected 'e0' to be marked as PendingSpam, got %v", m.pendingEmails["e0"])
	}
}

func TestSingleEmailBackFromViewer(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.mode = ModeIndex
	m.height = 20
	m.width = 80

	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Single Email", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	m.rawEmails = []uicommon.FolderEmail{e0}
	m.rebuildVisibleEmails()

	if len(m.emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(m.emails))
	}

	// View the email (press Enter)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(*tuiModel)
	if m.mode != ModeDetail {
		t.Errorf("expected ModeDetail, got %d", m.mode)
	}

	// Go back to the index (press q)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = res.(*tuiModel)
	if m.mode != ModeIndex {
		t.Errorf("expected ModeIndex, got %d", m.mode)
	}

	// Render view and check if the single email is listed
	viewStr := m.View()
	if !strings.Contains(viewStr, "Single Email") {
		t.Errorf("expected email subject 'Single Email' to be rendered, but view output was:\n%s", viewStr)
	}
}

func TestSingleEmailBgClassifyWithMockClient(t *testing.T) {
	tempDir := t.TempDir()

	// Create cache dir structure
	if err := os.MkdirAll(filepath.Join(tempDir, "indexes"), 0700); err != nil {
		t.Fatal(err)
	}

	// Write mock email into the msg cache
	baseTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	e0 := uicommon.FolderEmail{ID: "e0", Subject: "Single Email", MessageID: "<e0>", FromRaw: "Alice <a@b.com>", EmailDate: baseTime}
	emailContent := []byte("Subject: Single Email\nMessage-ID: <e0>\nFrom: Alice <a@b.com>\nDate: Sat, 20 Jun 2026 12:00:00 +0000\n\nBody")
	if err := msg.Store(tempDir, "e0", emailContent, baseTime); err != nil {
		t.Fatal(err)
	}

	// Write the index file for INBOX
	if err := label.ReplaceAll(tempDir, "INBOX", []string{"e0"}); err != nil {
		t.Fatal(err)
	}

	cfg := &cfg_g.Config{
		DownloadDir: tempDir,
	}
	cl := &mailclient.MockMailClient{Cfg: cfg}
	m := NewTuiModel(cl, "INBOX", nil)
	m.height = 20
	m.width = 80

	// Initially loaded from cache
	if len(m.emails) != 1 {
		t.Fatalf("expected 1 email loaded from cache, got %d", len(m.emails))
	}

	// Simulate emailsLoadedMsg
	res, _ := m.Update(emailsLoadedMsg{emails: []uicommon.FolderEmail{e0}})
	m = res.(*tuiModel)
	if len(m.emails) != 1 {
		t.Fatalf("expected 1 email after emailsLoadedMsg, got %d", len(m.emails))
	}

	// View the email (press Enter)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(*tuiModel)
	if m.mode != ModeDetail {
		t.Fatalf("expected ModeDetail, got %d", m.mode)
	}

	// While in Detail mode, receive classifyDoneMsg
	res, _ = m.Update(classifyDoneMsg{})
	m = res.(*tuiModel)

	// Go back to the index (press tab)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(*tuiModel)
	if m.mode != ModeIndex {
		t.Fatalf("expected ModeIndex, got %d", m.mode)
	}

	// Render view and check if the single email is listed
	viewStr := m.View()
	if !strings.Contains(viewStr, "Single Email") {
		t.Errorf("expected email subject 'Single Email' to be rendered, but view output was:\n%s", viewStr)
	}
}

func TestHelpGrayoutOverlay(t *testing.T) {
	m := NewTuiModel(nil, "", nil)
	m.width = 80
	m.height = 40

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = res.(*tuiModel)
	if !m.showHelp {
		t.Fatal("expected showHelp=true after pressing '?'")
	}

	viewStr := m.View()

	viewLines := strings.Split(viewStr, "\n")

	if !strings.Contains(viewStr, "Move down") {
		t.Error("expected help content 'Move down' in view output")
	}
	if !strings.Contains(viewStr, "KEYBINDINGS") {
		t.Error("expected 'KEYBINDINGS' header in view output")
	}

	if len(viewLines) != m.height {
		t.Errorf("expected output to be exactly %d lines (terminal height), got %d", m.height, len(viewLines))
	}

	firstContentIdx := -1
	for i, line := range viewLines {
		if strings.Contains(line, "KEYBINDINGS") {
			firstContentIdx = i
			break
		}
	}
	if firstContentIdx > 0 {
		t.Logf("grayout top padding: %d lines", firstContentIdx)
	} else {
		t.Error("expected grayout padding above help dialog")
	}

	lastContentIdx := -1
	for i := len(viewLines) - 1; i >= 0; i-- {
		if strings.Contains(viewLines[i], "dismiss") {
			lastContentIdx = i
			break
		}
	}
	bottomPad := len(viewLines) - lastContentIdx - 1
	if bottomPad > 0 {
		t.Logf("grayout bottom padding: %d lines", bottomPad)
	} else {
		t.Error("expected grayout padding below help dialog")
	}

	grayoutAnsi := "48;2;26;26;26"
	if !strings.Contains(viewStr, grayoutAnsi) {
		t.Error("expected grayout background color (48;2;26;26;26) in help overlay")
	}

	if !strings.Contains(viewStr, "┌") {
		t.Error("expected dialog top border in help overlay")
	}
	if !strings.Contains(viewStr, "└") {
		t.Error("expected dialog bottom border in help overlay")
	}
	if !strings.Contains(viewStr, "│") {
		t.Error("expected dialog vertical borders in help overlay")
	}
}
