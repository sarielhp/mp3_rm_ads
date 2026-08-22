package jmap

import (
	"testing"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestNewJMAPClient(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:       "test",
		Type:       "jmap",
		Username:   "test@example.com",
		Password:   "token",
		SessionURL: "https://jmap.example.com",
	}
	config := &cfg_g.Config{}

	client := NewJMAPClient(acc, config)

	if client == nil {
		t.Fatal("NewJMAPClient returned nil")
	}

	// Verify it implements MailClient interface
	var _ mailclient.MailClient = client
}

func TestJMAPClient_Validate(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:           "test",
		Type:           "jmap",
		Username:       "test@example.com",
		Password:       "token",
		SessionURL:     "https://jmap.example.com",
		SpamFolder:     "Spam",
		ReceivedFolder: "Archive",
	}
	config := &cfg_g.Config{}

	client := NewJMAPClient(acc, config)

	// Validate should succeed with valid config (without actually connecting)
	// The validation happens in init() which is called by Validate()
	// For testing, we just verify the config is valid
	err := client.Validate()
	if err != nil {
		// If it's a network error, that's expected - the config is valid
		if err.Error() != "JMAP authentication failed for test@example.com: Get \"https://jmap.example.com\": dial tcp: lookup jmap.example.com: no such host" {
			t.Errorf("Validate() returned unexpected error: %v", err)
		}
	}
}

func TestJMAPClient_InboxFolder(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:       "test",
		Type:       "jmap",
		Username:   "test@example.com",
		Password:   "token",
		SessionURL: "https://jmap.example.com",
	}
	config := &cfg_g.Config{}

	client := NewJMAPClient(acc, config)

	inbox := client.InboxFolder()
	if inbox != "Inbox" {
		t.Errorf("InboxFolder() = %q, want %q", inbox, "Inbox")
	}
}

func TestJMAPClient_BackendType(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:       "test",
		Type:       "jmap",
		Username:   "test@example.com",
		Password:   "token",
		SessionURL: "https://jmap.example.com",
	}
	config := &cfg_g.Config{}

	client := NewJMAPClient(acc, config)

	backendType := client.BackendType()
	if backendType != "jmap" {
		t.Errorf("BackendType() = %q, want %q", backendType, "jmap")
	}
}

func TestJMAPClient_Config(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:       "test",
		Type:       "jmap",
		Username:   "test@example.com",
		Password:   "token",
		SessionURL: "https://jmap.example.com",
	}
	config := &cfg_g.Config{
		SelectedAccount: "test",
	}

	client := NewJMAPClient(acc, config)

	cfg := client.Config()
	if cfg == nil {
		t.Fatal("Config() returned nil")
	}
	if cfg.SelectedAccount != "test" {
		t.Errorf("Config().SelectedAccount = %q, want %q", cfg.SelectedAccount, "test")
	}
}
