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

func newTestMultiAccountModel(accounts []cfg_acc.AccountConfig, selected string) *tuiModel {
	cfg := &cfg_g.Config{
		Accounts:        accounts,
		SelectedAccount: selected,
	}
	m := NewTuiModel(&mailclient.MockMailClient{Cfg: cfg}, "", nil)
	m.cfg = cfg
	m.width = 80
	m.height = 24
	m.theme = uicommon.NewThemeManager()
	return m
}

func TestAccountSwitch_GetAccountsAndCurrentIndex(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "personal", DisplayName: "Personal Gmail", Type: "gmail", Username: "user1@gmail.com"},
		{Name: "work", DisplayName: "Work Fastmail", Type: "jmap", Username: "user2@fastmail.com"},
	}
	m := newTestMultiAccountModel(accs, "work")

	got := m.getAccounts()
	if len(got) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(got))
	}

	idx := m.currentAccountIndex()
	if idx != 1 {
		t.Fatalf("expected current account index 1 for 'work', got %d", idx)
	}

	m.Config().SelectedAccount = "personal"
	idx = m.currentAccountIndex()
	if idx != 0 {
		t.Fatalf("expected current account index 0 for 'personal', got %d", idx)
	}
}

func TestAccountSwitch_OpenCloseOverlay(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "acc1", DisplayName: "Account 1", Type: "gmail"},
		{Name: "acc2", DisplayName: "Account 2", Type: "jmap"},
	}
	m := newTestMultiAccountModel(accs, "acc1")

	// Pressing 'a' in index mode should open account overlay
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen to be true after pressing 'a'")
	}
	if m.accountCursor != 0 {
		t.Errorf("expected accountCursor 0, got %d", m.accountCursor)
	}

	// Pressing 'Esc' should close account overlay
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	if m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen to be false after pressing Esc")
	}

	// Pressing 'A' should open account overlay
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if !m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen to be true after pressing 'A'")
	}

	// Pressing 'q' should close overlay
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen to be false after pressing 'q'")
	}
}

func TestAccountSwitch_OverlayNavigationAndSelection(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "acc1", DisplayName: "Account 1", Type: "gmail"},
		{Name: "acc2", DisplayName: "Account 2", Type: "jmap"},
		{Name: "acc3", DisplayName: "Account 3", Type: "outlook"},
	}
	m := newTestMultiAccountModel(accs, "acc1")
	m.openAccountOverlay()

	// Navigate down with 'j'
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.accountCursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", m.accountCursor)
	}

	// Navigate down with 'down' arrow
	m.key(tea.KeyMsg{Type: tea.KeyDown})
	if m.accountCursor != 2 {
		t.Errorf("expected cursor 2 after Down arrow, got %d", m.accountCursor)
	}

	// Wrap around down
	m.key(tea.KeyMsg{Type: tea.KeyDown})
	if m.accountCursor != 0 {
		t.Errorf("expected cursor 0 after wrap-around down, got %d", m.accountCursor)
	}

	// Navigate up with 'k'
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.accountCursor != 2 {
		t.Errorf("expected cursor 2 after 'k' wrap-around up, got %d", m.accountCursor)
	}

	// Select with Enter
	m.key(tea.KeyMsg{Type: tea.KeyEnter})
	if m.accountOverlayOpen {
		t.Error("expected account overlay to close after selection")
	}
	if m.Config().SelectedAccount != "acc3" {
		t.Errorf("expected SelectedAccount 'acc3', got %q", m.Config().SelectedAccount)
	}
}

func TestAccountSwitch_DirectNumberKeys(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "acc1", DisplayName: "Account 1", Type: "gmail"},
		{Name: "acc2", DisplayName: "Account 2", Type: "jmap"},
	}
	m := newTestMultiAccountModel(accs, "acc1")

	// Pressing '2' in index mode should directly switch to account 2
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if m.Config().SelectedAccount != "acc2" {
		t.Errorf("expected SelectedAccount 'acc2' after pressing '2', got %q", m.Config().SelectedAccount)
	}

	// Pressing '1' in index mode should directly switch back to account 1
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.Config().SelectedAccount != "acc1" {
		t.Errorf("expected SelectedAccount 'acc1' after pressing '1', got %q", m.Config().SelectedAccount)
	}
}

func TestAccountSwitch_CyclePrevNext(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "acc1", DisplayName: "Account 1", Type: "gmail"},
		{Name: "acc2", DisplayName: "Account 2", Type: "jmap"},
		{Name: "acc3", DisplayName: "Account 3", Type: "outlook"},
	}
	m := newTestMultiAccountModel(accs, "acc1")

	// Next account with ']'
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.Config().SelectedAccount != "acc2" {
		t.Errorf("expected SelectedAccount 'acc2' after ']', got %q", m.Config().SelectedAccount)
	}

	// Next account with ']' again
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.Config().SelectedAccount != "acc3" {
		t.Errorf("expected SelectedAccount 'acc3' after second ']', got %q", m.Config().SelectedAccount)
	}

	// Prev account with '['
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if m.Config().SelectedAccount != "acc2" {
		t.Errorf("expected SelectedAccount 'acc2' after '[', got %q", m.Config().SelectedAccount)
	}
}

func TestAccountSwitch_MenuOption(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "acc1", DisplayName: "Account 1", Type: "gmail"},
		{Name: "acc2", DisplayName: "Account 2", Type: "jmap"},
	}
	m := newTestMultiAccountModel(accs, "acc1")

	// Open menu with 'm'
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !m.menuOpen {
		t.Fatal("expected menuOpen to be true")
	}

	// Move cursor down to "Switch Account"
	m.key(tea.KeyMsg{Type: tea.KeyDown})
	if m.menuItems[m.menuCursor] != "Switch Account" {
		t.Fatalf("expected 'Switch Account' at cursor 1, got %q", m.menuItems[m.menuCursor])
	}

	// Press Enter to open account overlay
	m.key(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen to be true after selecting 'Switch Account' from menu")
	}
	if m.menuOpen {
		t.Error("expected menuOpen to be false after selection")
	}
}

func TestAccountSwitch_RenderOverlay(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "personal", DisplayName: "Personal", Type: "gmail", Username: "me@gmail.com"},
		{Name: "work", DisplayName: "Work", Type: "jmap", Username: "me@work.com"},
	}
	m := newTestMultiAccountModel(accs, "personal")
	m.accountOverlayOpen = true

	view := m.View()
	if !strings.Contains(view, "SWITCH ACCOUNT") {
		t.Error("expected view to contain 'SWITCH ACCOUNT'")
	}
	if !strings.Contains(view, "Personal") {
		t.Error("expected view to contain 'Personal'")
	}
	if !strings.Contains(view, "Work") {
		t.Error("expected view to contain 'Work'")
	}
	if strings.Contains(view, "● Active") {
		t.Error("expected view not to contain '● Active'")
	}
	if !strings.Contains(view, "●=active") {
		t.Error("expected view to contain '●=active'")
	}
	if !strings.Contains(view, "[1]") || !strings.Contains(view, "●") {
		t.Error("expected view to contain '[1]' and '●' for active account")
	}
}

func TestAccountSwitch_F3KeyOpensOverlay(t *testing.T) {
	accs := []cfg_acc.AccountConfig{
		{Name: "personal", DisplayName: "Personal", Type: "gmail"},
		{Name: "work", DisplayName: "Work", Type: "jmap"},
	}
	m := newTestMultiAccountModel(accs, "personal")
	m.accountOverlayOpen = false

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	m = res.(*tuiModel)
	if !m.accountOverlayOpen {
		t.Error("expected accountOverlayOpen=true after F3 key")
	}
}
