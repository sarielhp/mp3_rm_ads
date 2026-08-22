package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mail_cli/app"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestAddDomainRuleToConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	downloadDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		t.Fatalf("failed to create download dir: %v", err)
	}

	// Set up initial config
	fc := &cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{
				Name:           "default",
				Type:           "gmail",
				Username:       "user@example.com",
				Password:       "pass",
				SpamFolder:     "Spam",
				ReceivedFolder: "INBOX",
				SpamLearn:      "Spam",
				Rules:          []cfg_acc.Rule{},
			},
		},
	}
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Store a test email in cache and index it in INBOX
	msgID := "testmsg123"
	emailRaw := []byte("From: \"Alice Smith\" <alice@acme.org>\r\nSubject: Test Email\r\n\r\nHello World")
	if err := msg.Store(downloadDir, msgID, emailRaw, time.Now()); err != nil {
		t.Fatalf("failed to store cached email: %v", err)
	}
	if err := label.Add(downloadDir, "INBOX", msgID); err != nil {
		t.Fatalf("failed to index cached email: %v", err)
	}

	config := &cfg_g.Config{
		ConfigDir:       tempDir,
		DownloadDir:     downloadDir,
		SelectedAccount: "default",
	}

	session := &app.Session{
		Config: config,
		GetClient: func(c *cfg_g.Config) (mailclient.MailClient, error) {
			return &mockMailClient{}, nil
		},
	}

	// Run addDomainRuleToConfig with target label "Newsletters"
	if err := addDomainRuleToConfig(session, msgID, "Newsletters"); err != nil {
		t.Fatalf("addDomainRuleToConfig failed: %v", err)
	}

	// Read updated config.json and verify rule was added for @acme.org
	updatedFC, err := cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to read updated config: %v", err)
	}

	accs := *updatedFC.Accounts
	if len(accs[0].Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(accs[0].Rules))
	}

	rule := accs[0].Rules[0]
	if rule.Sender != "@acme.org" {
		t.Errorf("expected rule sender '@acme.org', got %q", rule.Sender)
	}
	if rule.Label != "Newsletters" {
		t.Errorf("expected rule label 'Newsletters', got %q", rule.Label)
	}

	// Verify MatchRules matches emails from @acme.org
	matchedRule := cfg_acc.MatchRules(accs[0].Rules, "bob@acme.org", "Any Subject")
	if matchedRule == nil {
		t.Errorf("MatchRules failed to match bob@acme.org against rule @acme.org")
	}
}
