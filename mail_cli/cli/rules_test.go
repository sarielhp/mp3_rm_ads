package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

// mockMailClient implementing MailClient just for GetMatchingLabels
type mockMailClient struct {
	mailclient.MailClient
	matchingLabels []string
	cfg            *cfg_g.Config
}

func (m *mockMailClient) GetMatchingLabels(prefix string) ([]string, error) {
	if prefix == "" {
		return m.matchingLabels, nil
	}
	var res []string
	for _, l := range m.matchingLabels {
		if strings.HasPrefix(l, prefix) {
			res = append(res, l)
		}
	}
	return res, nil
}

func (m *mockMailClient) Config() *cfg_g.Config {
	return m.cfg
}

func TestRulesAccountLevelAndResolution(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Set up initial config
	fc := &cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{
				Name:           "default",
				Type:           "gmail",
				Username:       "test@gmail.com",
				Password:       "pass",
				IMAPHost:       "imap.gmail.com",
				SpamFolder:     "Spam",
				ReceivedFolder: "INBOX",
				SpamLearn:      "Spam",
				Rules:          []cfg_acc.Rule{},
			},
		},
	}
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("failed to marshal setup config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write setup config: %v", err)
	}

	config := &cfg_g.Config{
		ConfigDir:       tempDir,
		SelectedAccount: "default",
	}

	mockClient := &mockMailClient{
		matchingLabels: []string{"INBOX", "Work/Projects/Antigravity", "Spam"},
	}

	session := &app.Session{
		Config: config,
		GetClient: func(c *cfg_g.Config) (mailclient.MailClient, error) {
			return mockClient, nil
		},
	}

	// 1. Add rule with unresolved label suffix, check that it resolves and prints
	// Redirect stdout to inspect printing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = addRuleToConfig(session, "spammy@sender.com", "Antigravity", "")
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("addRuleToConfig failed: %v", err)
	}

	outBytes, _ := io.ReadAll(r)
	outStr := string(outBytes)

	// Verify resolution output
	// ResolveLabel prints "[*] Using label: Work/Projects/Antigravity"
	if !strings.Contains(outStr, "Using label: Work/Projects/Antigravity") && !strings.Contains(outStr, "Work/Projects/Antigravity") {
		t.Errorf("stdout output did not show using/adding Work/Projects/Antigravity: %s", outStr)
	}

	// Load config and check that the rule is under default account, not global
	loadedFc, err := cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loadedFc.Rules != nil && len(*loadedFc.Rules) > 0 {
		t.Errorf("expected global rules to be nil, got: %v", loadedFc.Rules)
	}

	accs := *loadedFc.Accounts
	if len(accs) == 0 {
		t.Fatal("no accounts in saved config")
	}
	rules := accs[0].Rules
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in account rules, got %d", len(rules))
	}
	if rules[0].Sender != "spammy@sender.com" {
		t.Errorf("expected sender 'spammy@sender.com', got '%s'", rules[0].Sender)
	}
	if rules[0].Label != "Work/Projects/Antigravity" {
		t.Errorf("expected resolved label 'Work/Projects/Antigravity', got '%s'", rules[0].Label)
	}

	// 2. Test listRules
	r, w, _ = os.Pipe()
	os.Stdout = w
	err = listRules(session)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("listRules failed: %v", err)
	}
	outBytes, _ = io.ReadAll(r)
	outStr = string(outBytes)
	if !strings.Contains(outStr, "spammy@sender.com -> Work/Projects/Antigravity") && !strings.Contains(outStr, "spammy@sender.com") {
		t.Errorf("listRules output did not contain rule. Output: %s", outStr)
	}

	// 2b. Test updating an existing rule (overwriting / rewriting)
	// First manually set Exported = true on the loaded config rule
	loadedFc, err = cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	(*loadedFc.Accounts)[0].Rules[0].Exported = true
	data, _ = json.Marshal(loadedFc)
	_ = os.WriteFile(configPath, data, 0600)

	// Now add rule again but with a different label (e.g. INBOX)
	err = addRuleToConfig(session, "spammy@sender.com", "INBOX", "")
	if err != nil {
		t.Fatalf("updating rule failed: %v", err)
	}

	// Load and verify that the label is updated and Exported is set back to false
	loadedFc, err = cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config after update: %v", err)
	}
	updatedRules := (*loadedFc.Accounts)[0].Rules
	if len(updatedRules) != 1 {
		t.Fatalf("expected exactly 1 rule, got %d", len(updatedRules))
	}
	if updatedRules[0].Label != "INBOX" {
		t.Errorf("expected updated label to be 'INBOX', got '%s'", updatedRules[0].Label)
	}
	if updatedRules[0].Exported {
		t.Error("expected Exported to be reset to false after label update")
	}

	// 2c. Test deduplication when listing rules
	// Manually inject duplicate rules into config.json
	loadedFc, err = cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	(*loadedFc.Accounts)[0].Rules = []cfg_acc.Rule{
		{Sender: "duplicate@sender.com", Label: "Label1"},
		{Sender: "other@sender.com", Label: "Label2"},
		{Sender: "duplicate@sender.com", Label: "Label3"}, // duplicate, keeping this one (last)
	}
	data, _ = json.Marshal(loadedFc)
	_ = os.WriteFile(configPath, data, 0600)

	// List rules to trigger deduplication
	r, w, _ = os.Pipe()
	os.Stdout = w
	err = listRules(session)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("listRules with duplicates failed: %v", err)
	}
	outBytes, _ = io.ReadAll(r)
	outStr = string(outBytes)

	// Verify deduplication message was printed
	if !strings.Contains(outStr, "Found duplicate rule for 'duplicate@sender.com'") {
		t.Errorf("expected deduplication warning, got: %s", outStr)
	}

	// Load and verify that only the last rule was kept and duplicates deleted
	loadedFc, err = cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config after deduplication: %v", err)
	}
	cleanedRules := (*loadedFc.Accounts)[0].Rules
	if len(cleanedRules) != 2 {
		t.Fatalf("expected 2 rules after deduplication, got %d", len(cleanedRules))
	}
	if cleanedRules[0].Sender != "other@sender.com" || cleanedRules[1].Sender != "duplicate@sender.com" {
		t.Errorf("unexpected rules: %+v", cleanedRules)
	}
	if cleanedRules[1].Label != "Label3" {
		t.Errorf("expected duplicate rule kept to have label 'Label3', got '%s'", cleanedRules[1].Label)
	}

	// 3. Delete rules
	err = deleteRuleFromConfig(session, "duplicate@sender.com", strings.NewReader(""))
	if err != nil {
		t.Fatalf("delete duplicate rule failed: %v", err)
	}
	err = deleteRuleFromConfig(session, "other@sender.com", strings.NewReader(""))
	if err != nil {
		t.Fatalf("delete other rule failed: %v", err)
	}

	loadedFc, err = cfg_g.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if len((*loadedFc.Accounts)[0].Rules) != 0 {
		t.Errorf("expected 0 rules after deletion, got %d", len((*loadedFc.Accounts)[0].Rules))
	}
}

func TestDeleteRuleByIndex(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	fc := &cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{
				Name: "default",
				Type: "gmail",
				Rules: []cfg_acc.Rule{
					{Sender: "rule1@sender.com", Label: "Label1"},
					{Sender: "rule2@sender.com", Label: "Label2", Exported: true},
					{Sender: "rule3@sender.com", Label: "Label3"},
				},
			},
		},
	}
	data, _ := json.Marshal(fc)
	_ = os.WriteFile(configPath, data, 0600)

	config := &cfg_g.Config{
		ConfigDir:       tempDir,
		SelectedAccount: "default",
	}
	session := &app.Session{
		Config: config,
	}

	// Test cancellation (inputting "n")
	err := deleteRuleFromConfig(session, "3", strings.NewReader("n\n"))
	if err != nil {
		t.Fatalf("unexpected error on cancel: %v", err)
	}

	// Verify not deleted
	loadedFc, _ := cfg_g.LoadConfigFile(configPath)
	if len((*loadedFc.Accounts)[0].Rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len((*loadedFc.Accounts)[0].Rules))
	}

	// Test confirm deletion of Rule 3 (which is index 2)
	err = deleteRuleFromConfig(session, "3", strings.NewReader("y\n"))
	if err != nil {
		t.Fatalf("delete rule by index failed: %v", err)
	}

	// Verify deleted
	loadedFc, _ = cfg_g.LoadConfigFile(configPath)
	rules := (*loadedFc.Accounts)[0].Rules
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Sender != "rule1@sender.com" || rules[1].Sender != "rule2@sender.com" {
		t.Errorf("unexpected remaining rules: %+v", rules)
	}
}

func TestDeleteInternalRuleByEmail(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	fc := &cfg_g.FileConfig{
		Accounts: &[]cfg_acc.AccountConfig{
			{
				Name: "default",
				Type: "gmail",
				Rules: []cfg_acc.Rule{
					{Sender: "foo@bar.com", Label: "Label1", Internal: true},
					{Sender: "John Doe <trigger@example.com>", Label: "Label2", Internal: true},
				},
			},
		},
	}
	data, _ := json.Marshal(fc)
	_ = os.WriteFile(configPath, data, 0600)

	config := &cfg_g.Config{
		ConfigDir:       tempDir,
		SelectedAccount: "default",
	}
	session := &app.Session{
		Config: config,
	}

	// Delete internal rule by email address trigger
	err := deleteRuleFromConfig(session, "trigger@example.com", strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error deleting rule by email: %v", err)
	}

	loadedFc, _ := cfg_g.LoadConfigFile(configPath)
	rules := (*loadedFc.Accounts)[0].Rules
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule remaining, got %d", len(rules))
	}
	if rules[0].Sender != "foo@bar.com" {
		t.Errorf("expected remaining rule for foo@bar.com, got %s", rules[0].Sender)
	}
}
