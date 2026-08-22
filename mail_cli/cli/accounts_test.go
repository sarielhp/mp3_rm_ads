package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

func TestAccountRename(t *testing.T) {
	// Create a temp directory for config and download dirs
	tempDir, err := os.MkdirTemp("", "mail_cli_accounts_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, "config")
	downloadDir := filepath.Join(tempDir, "download")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		t.Fatalf("failed to create download dir: %v", err)
	}

	// Create a mock config.json
	mockConfig := cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{Name: "acc1", Type: "gmail"},
			{Name: "acc2", Type: "jmap"},
		},
	}
	configBytes, err := json.Marshal(mockConfig)
	if err != nil {
		t.Fatalf("failed to marshal mock config: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	err = os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		t.Fatalf("failed to write mock config file: %v", err)
	}

	// Create fake cache directory and token file for acc1
	acc1Cache := filepath.Join(downloadDir, "acc1")
	err = os.MkdirAll(acc1Cache, 0755)
	if err != nil {
		t.Fatalf("failed to create fake cache dir: %v", err)
	}
	fakeEmailFile := filepath.Join(acc1Cache, "test_msg")
	err = os.WriteFile(fakeEmailFile, []byte("fake"), 0644)
	if err != nil {
		t.Fatalf("failed to write fake cache file: %v", err)
	}

	tokensDir := filepath.Join(configDir, "tokens")
	err = os.MkdirAll(tokensDir, 0755)
	if err != nil {
		t.Fatalf("failed to create tokens dir: %v", err)
	}
	acc1Token := filepath.Join(tokensDir, "token_acc1.json")
	err = os.WriteFile(acc1Token, []byte("fake token"), 0644)
	if err != nil {
		t.Fatalf("failed to write fake token file: %v", err)
	}

	session := &app.Session{
		Config: &cfg_g.Config{
			ConfigDir:       configDir,
			DownloadDir:     downloadDir,
			SelectedAccount: "acc1",
		},
	}

	cliApp := InitCLI(session)

	// 1. Rename nonexistent account (should error)
	err = cliApp.Execute([]string{"account", "rename", "nonexistent", "acc3"})
	if err == nil {
		t.Error("expected error renaming nonexistent account, got nil")
	}

	// 2. Rename to existing account name (should error - collision)
	err = cliApp.Execute([]string{"account", "rename", "acc1", "acc2"})
	if err == nil {
		t.Error("expected error renaming to existing account name, got nil")
	}

	// 3. Rename to same name but different case (should succeed)
	err = cliApp.Execute([]string{"account", "rename", "acc1", "ACC1"})
	if err != nil {
		t.Errorf("unexpected error renaming to same name with case change: %v", err)
	}

	// 4. Normal rename (should succeed and update DisplayName)
	err = cliApp.Execute([]string{"account", "rename", "ACC1", "acc3"})
	if err != nil {
		t.Errorf("unexpected error renaming account: %v", err)
	}

	// Verify changes in config file
	savedConfig, err := cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(*savedConfig.Accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(*savedConfig.Accounts))
	}
	foundDisplay := false
	for _, acc := range *savedConfig.Accounts {
		if acc.DisplayName == "acc3" {
			foundDisplay = true
			if acc.Name != "acc1" {
				t.Errorf("expected internal name to remain 'acc1', got %q", acc.Name)
			}
		}
	}
	if !foundDisplay {
		t.Error("display name 'acc3' not found in saved config")
	}

	// Verify cache directory and token file were NOT moved or deleted (structures stay unchanged)
	if _, err := os.Stat(filepath.Join(downloadDir, "acc1")); err != nil {
		t.Error("expected original cache directory 'acc1' to still exist")
	}
	if _, err := os.Stat(filepath.Join(downloadDir, "acc3")); !os.IsNotExist(err) {
		t.Error("expected cache directory 'acc3' to not be created")
	}

	if _, err := os.Stat(filepath.Join(tokensDir, "token_acc1.json")); err != nil {
		t.Error("expected original token file 'token_acc1.json' to still exist")
	}
	if _, err := os.Stat(filepath.Join(tokensDir, "token_acc3.json")); !os.IsNotExist(err) {
		t.Error("expected token file 'token_acc3.json' to not be created")
	}
}

func TestAccountListNumbers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mail_cli_list_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, "config")
	downloadDir := filepath.Join(tempDir, "download")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(downloadDir, 0755)

	mockConfig := cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{Name: "acc1", Type: "gmail"},
			{Name: "acc2", Type: "jmap"},
		},
	}
	configBytes, err := json.Marshal(mockConfig)
	if err != nil {
		t.Fatalf("failed to marshal mock config: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		t.Fatalf("failed to write mock config file: %v", err)
	}

	session := &app.Session{
		Config: &cfg_g.Config{
			ConfigDir:       configDir,
			DownloadDir:     downloadDir,
			SelectedAccount: "acc1",
		},
	}

	cliApp := InitCLI(session)

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err = cliApp.Execute([]string{"account", "list"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("account list failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "1. acc1 (gmail, type: regular) *") {
		t.Errorf("expected output to contain '1. acc1 (gmail, type: regular) *', got:\n%s", output)
	}
	if !strings.Contains(output, "2. acc2 (jmap, type: regular)") {
		t.Errorf("expected output to contain '2. acc2 (jmap, type: regular)', got:\n%s", output)
	}
}
