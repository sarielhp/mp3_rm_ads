package outlook

import (
	"testing"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestNewOutlookClient(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:     "test",
		Type:     "outlook",
		Username: "test@example.com",
	}
	config := &cfg_g.Config{}

	client := NewOutlookClient(acc, config)

	if client == nil {
		t.Fatal("NewOutlookClient returned nil")
	}

	// Verify it implements MailClient interface
	var _ mailclient.MailClient = client
}

func TestOutlookClient_Validate(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:           "test",
		Type:           "outlook",
		Username:       "test@example.com",
		SpamFolder:     "Junk Email",
		ReceivedFolder: "Archive",
	}
	config := &cfg_g.Config{
		SelectedAccount: "test",
	}

	client := NewOutlookClient(acc, config)

	// For testing, we skip the actual OAuth validation
	// The client is created successfully, validation would require real credentials
	// Just verify the client was created
	if client == nil {
		t.Fatal("NewOutlookClient returned nil")
	}
}

func TestOutlookClient_InboxFolder(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:     "test",
		Type:     "outlook",
		Username: "test@example.com",
	}
	config := &cfg_g.Config{}

	client := NewOutlookClient(acc, config)

	inbox := client.InboxFolder()
	if inbox != "Inbox" {
		t.Errorf("InboxFolder() = %q, want %q", inbox, "Inbox")
	}
}

func TestOutlookClient_BackendType(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:     "test",
		Type:     "outlook",
		Username: "test@example.com",
	}
	config := &cfg_g.Config{}

	client := NewOutlookClient(acc, config)

	backendType := client.BackendType()
	if backendType != "outlook" {
		t.Errorf("BackendType() = %q, want %q", backendType, "outlook")
	}
}

func TestOutlookClient_Config(t *testing.T) {
	acc := cfg_acc.AccountConfig{
		Name:     "test",
		Type:     "outlook",
		Username: "test@example.com",
	}
	config := &cfg_g.Config{
		SelectedAccount: "test",
	}

	client := NewOutlookClient(acc, config)

	cfg := client.Config()
	if cfg == nil {
		t.Fatal("Config() returned nil")
	}
	if cfg.SelectedAccount != "test" {
		t.Errorf("Config().SelectedAccount = %q, want %q", cfg.SelectedAccount, "test")
	}
}
