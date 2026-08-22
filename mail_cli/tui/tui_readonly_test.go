package tui

import (
	"strings"
	"testing"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
	"mail_cli/uicommon"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI_ReadOnly_TopBarAndAccountOverlay(t *testing.T) {
	cfg := &cfg_g.Config{
		DownloadDir:     t.TempDir(),
		ReadOnly:        true,
		SelectedAccount: "Personal",
		Accounts: []cfg_acc.AccountConfig{
			{Name: "Personal", DisplayName: "Personal", Type: "gmail", ReadOnly: true},
			{Name: "Work", DisplayName: "Work", Type: "jmap", ReadOnly: false},
		},
	}
	mockClient := &mailclient.MockMailClient{
		Cfg: cfg,
		LabelItems: []cfg_acc.LabelItem{
			{Name: "INBOX", FullName: "INBOX"},
		},
	}

	m := NewTuiModel(mockClient, "INBOX", nil)
	m.width = 80
	m.height = 24

	topBar := renderTopBar(m)
	if !strings.Contains(topBar, "[READ-ONLY]") {
		t.Errorf("expected top bar to contain [READ-ONLY], got: %q", topBar)
	}

	overlay := renderAccountOverlay(m)
	if !strings.Contains(overlay, "[RO]") {
		t.Errorf("expected account overlay to contain [RO], got: %q", overlay)
	}
	if strings.Contains(overlay, "● Active") {
		t.Errorf("expected account overlay not to contain '● Active', got: %q", overlay)
	}
	if !strings.Contains(overlay, "●=active") {
		t.Errorf("expected account overlay to contain '●=active', got: %q", overlay)
	}
}

func TestTUI_ReadOnly_ActionKeysIntercepted(t *testing.T) {
	cfg := &cfg_g.Config{
		DownloadDir: t.TempDir(),
		ReadOnly:    true,
		Accounts: []cfg_acc.AccountConfig{
			{Name: "Personal", Type: "gmail", ReadOnly: true},
		},
	}
	mockClient := &mailclient.MockMailClient{
		Cfg: cfg,
	}

	m := NewTuiModel(mockClient, "INBOX", nil)
	m.emails = []uicommon.FolderEmail{
		{ID: "msg1", Subject: "Test Email", FromEmail: "sender@example.com"},
	}
	m.eIdx = 0
	m.selectedID = "msg1"

	// Archive key 'E'
	m2, _ := m.archiveCurrentEmail()
	mMod := m2.(*tuiModel)
	if !mMod.showError || mMod.err == nil || !strings.Contains(mMod.err.Error(), "Read-Only Mode: Action simulated") {
		t.Errorf("expected read-only error banner on archive, got showError=%v err=%v", mMod.showError, mMod.err)
	}

	// Spam key 's'
	m2, _ = m.spamCurrentEmail()
	mMod = m2.(*tuiModel)
	if !mMod.showError || mMod.err == nil || !strings.Contains(mMod.err.Error(), "Read-Only Mode: Action simulated") {
		t.Errorf("expected read-only error banner on spam, got showError=%v err=%v", mMod.showError, mMod.err)
	}

	// Delete key 'd'
	m2, _ = m.deleteCurrentEmail()
	mMod = m2.(*tuiModel)
	if !mMod.showError || mMod.err == nil || !strings.Contains(mMod.err.Error(), "Read-Only Mode: Action simulated") {
		t.Errorf("expected read-only error banner on delete, got showError=%v err=%v", mMod.showError, mMod.err)
	}

	// Unspam key 'U'
	m2, _ = m.unspamCurrentEmail()
	mMod = m2.(*tuiModel)
	if !mMod.showError || mMod.err == nil || !strings.Contains(mMod.err.Error(), "Read-Only Mode: Action simulated") {
		t.Errorf("expected read-only error banner on unspam, got showError=%v err=%v", mMod.showError, mMod.err)
	}
}

func TestTUI_ReadOnly_SendBlocked(t *testing.T) {
	cfg := &cfg_g.Config{
		DownloadDir: t.TempDir(),
		ReadOnly:    true,
	}
	mockClient := &mailclient.MockMailClient{Cfg: cfg}

	m := NewTuiModel(mockClient, "INBOX", nil)
	m.confirmSend = true
	m.confirmSendBytes = []byte("From: user@example.com\r\nTo: dest@example.com\r\n\r\nHello")

	m2, _ := m.kConfirmSend(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	mMod := m2.(*tuiModel)

	if !mMod.showError || mMod.err == nil || !strings.Contains(mMod.err.Error(), "Sending is disabled in Read-Only mode") {
		t.Errorf("expected send disabled error, got: %v", mMod.err)
	}
}
