package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

func TestCmdExecutionStartedFlag(t *testing.T) {
	// Reset the flag
	app.CmdExecutionStarted = false

	session := &app.Session{
		Config: &cfg_g.Config{},
		PreCheck: func(cfg *cfg_g.Config) error {
			return errors.New("pre-check failed")
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"scan", "inbox"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !app.CmdExecutionStarted {
		t.Error("expected app.CmdExecutionStarted to be true after executing command")
	}

	// Reset again and test validation failure
	app.CmdExecutionStarted = false
	err = cliApp.Execute([]string{"scan", "inbox", "extra_arg"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	if app.CmdExecutionStarted {
		t.Error("expected app.CmdExecutionStarted to be false when command validation fails")
	}
}

func TestScanInboxExplicitFlag(t *testing.T) {
	app.FlagExplicitScanInbox = false

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunScan: func(cfg *cfg_g.Config, labelPrefix, moveSpam, moveInbox string) (int, error) {
			return 0, nil
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"scan", "inbox"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !app.FlagExplicitScanInbox {
		t.Error("expected app.FlagExplicitScanInbox to be true when explicitly running scan inbox")
	}

	// Reset and run implicit/default scan (no arguments)
	app.FlagExplicitScanInbox = false
	err = cliApp.Execute([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.FlagExplicitScanInbox {
		t.Error("expected app.FlagExplicitScanInbox to be false when running default scan implicitly")
	}
}

func TestUnspamFolderCmd(t *testing.T) {
	unspamFolderCalled := false
	var targetFolder string

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunUnspamFolder: func(cfg *cfg_g.Config, folder string) error {
			unspamFolderCalled = true
			targetFolder = folder
			return nil
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"unspam", "folder", "Spam"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !unspamFolderCalled {
		t.Error("expected session.RunUnspamFolder to be called")
	}
	if targetFolder != "Spam" {
		t.Errorf("expected targetFolder to be Spam, got %s", targetFolder)
	}
}

func TestLearningResetCmd(t *testing.T) {
	learningResetCalled := false

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunLearningReset: func(cfg *cfg_g.Config) error {
			learningResetCalled = true
			return nil
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"learning", "reset"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !learningResetCalled {
		t.Error("expected session.RunLearningReset to be called")
	}
}

func TestScanPatternFlag(t *testing.T) {
	app.FlagScanPattern = ""

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunScan: func(cfg *cfg_g.Config, labelPrefix, moveSpam, moveInbox string) (int, error) {
			return 0, nil
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"scan", "received", "-p", "Urgent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.FlagScanPattern != "Urgent" {
		t.Errorf("expected app.FlagScanPattern to be Urgent, got %q", app.FlagScanPattern)
	}

	// Test implicit scan pattern flag
	app.FlagScanPattern = ""
	cliApp = InitCLI(session)
	err = cliApp.Execute([]string{"-p", "Urgent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.FlagScanPattern != "Urgent" {
		t.Errorf("expected app.FlagScanPattern to be Urgent, got %q", app.FlagScanPattern)
	}
}

func TestTuiPerfectSpamMatch(t *testing.T) {
	tmpDir := t.TempDir()
	config := &cfg_g.Config{
		ConfigDir:   tmpDir,
		DownloadDir: tmpDir,
		Accounts: []cfg_acc.AccountConfig{
			{Name: "test-account", Type: "gmail", ReceivedFolder: "INBOX", SpamFolder: "Spam"},
		},
		SelectedAccount: "test-account",
	}

	configJSON := `{
		"accounts": [
			{"name": "test-account", "type": "gmail", "received_folder": "INBOX", "spam_folder": "Spam"}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configJSON), 0600)
	if err != nil {
		t.Fatalf("failed to write config.json: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "test-account")
	err = os.MkdirAll(cacheDir, 0700)
	if err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	cs := &cache.DiskCacheStore{DownloadDir: cacheDir}
	// Setup folders, one of which is a perfect case-insensitive match for "spam"
	items := []cfg_acc.LabelItem{
		{Name: "Spam", FullName: "Spam"},
		{Name: "MySpam", FullName: "MySpam"},
		{Name: "ArchiveSpam", FullName: "ArchiveSpam"},
	}
	err = cs.SaveLabelItems(items)
	if err != nil {
		t.Fatalf("failed to save label items: %v", err)
	}

	var initTuiCalledWith string
	session := &app.Session{
		Config: config,
		InitTUI: func(cfg *cfg_g.Config, labelPrefix string) error {
			initTuiCalledWith = labelPrefix
			return nil
		},
	}

	cliApp := InitCLI(session)
	err = cliApp.Execute([]string{"tui", "spam"})
	if err != nil {
		t.Fatalf("unexpected error executing tui command: %v", err)
	}

	if initTuiCalledWith != "Spam" {
		t.Errorf("expected InitTUI to be called with Spam, got %q", initTuiCalledWith)
	}
}
