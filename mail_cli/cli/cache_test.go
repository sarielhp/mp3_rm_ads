package cli

import (
	"os"
	"path/filepath"
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestCachePruneWithFolder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mail_cli_cache_prune_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "inbox")
	_ = os.MkdirAll(subDir, 0700)

	testFile := filepath.Join(subDir, "msg1.eml")
	_ = os.WriteFile(testFile, []byte("email"), 0600)

	session := &app.Session{
		Config: &cfg_g.Config{
			DownloadDir:     tempDir,
			SelectedAccount: "inbox",
			Accounts: []cfg_acc.AccountConfig{
				{Name: "inbox", Type: "gmail"},
			},
		},
	}
	cliApp := InitCLI(session)

	if err := cliApp.Execute([]string{"cache", "prune", "0"}); err != nil {
		t.Fatalf("failed to execute cache prune command: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted, but it still exists", testFile)
	}
}

func TestCacheReset(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mail_cli_cache_reset_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	accDir := filepath.Join(tempDir, "test_account")
	otherDir := filepath.Join(tempDir, "other_account")
	_ = os.MkdirAll(accDir, 0700)
	_ = os.MkdirAll(otherDir, 0700)

	accFile := filepath.Join(accDir, "msg.eml")
	otherFile := filepath.Join(otherDir, "msg.eml")
	_ = os.WriteFile(accFile, []byte("email"), 0600)
	_ = os.WriteFile(otherFile, []byte("email"), 0600)

	session := &app.Session{
		Config: &cfg_g.Config{
			DownloadDir:     tempDir,
			SelectedAccount: "test_account",
			Accounts: []cfg_acc.AccountConfig{
				{Name: "test_account", Type: "gmail"},
				{Name: "other_account", Type: "gmail"},
			},
		},
		GetClient: func(config *cfg_g.Config) (mailclient.MailClient, error) {
			return &mailclient.MockMailClient{
				Cfg: config,
			}, nil
		},
	}
	cliApp := InitCLI(session)

	if err := cliApp.Execute([]string{"cache", "reset"}); err != nil {
		t.Fatalf("failed to execute cache reset: %v", err)
	}

	if _, err := os.Stat(accFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s in target account cache to be deleted, but it still exists", accFile)
	}

	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Errorf("expected file %s in other account cache to still exist, but it was deleted", otherFile)
	}

	if _, err := os.Stat(accDir); os.IsNotExist(err) {
		t.Errorf("expected per-account cache directory %s to be recreated, but it does not exist", accDir)
	}
}
