package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mail_cli/cache"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		subject string
		want    bool
	}{
		{"*invoice*", "Invoice #1234", true},
		{"*invoice*", "Billing department", false},
		{"urgent?", "urgent!", true},
		{"urgent?", "urgent12", false},
		{"*urgent*", "RE: urgent meeting request", true},
		{"exactly-this", "Exactly-this", true},
		{"exactly-this", "not exactly-this", false},
		{"*/*", "Subject with / slash", true},
	}

	for _, tt := range tests {
		got, err := matchPattern(tt.pattern, tt.subject)
		if err != nil {
			t.Errorf("matchPattern(%q, %q) returned unexpected error: %v", tt.pattern, tt.subject, err)
			continue
		}
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.subject, got, tt.want)
		}
	}
}

func TestResolveUniqueLabel(t *testing.T) {
	mockClient := &mailclient.MockMailClient{
		Labels: []string{
			"INBOX",
			"Work",
			"Work/ProjectA",
			"Work/ProjectB",
			"Personal/Finance",
			"Archive",
		},
		Cfg: &cfg_g.Config{
			ReceivedFolder: "INBOX",
		},
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"Exact match", "Work", "Work", false},
		{"Case-insensitive exact match", "work", "Work", false},
		{"Ambiguous prefix match", "Work/Project", "", true},
		{"Unique suffix match", "Finance", "Personal/Finance", false},
		{"Unique prefix match", "Personal", "Personal/Finance", false},
		{"Inbox alias resolution", "inbox", "INBOX", false},
		{"Not found", "Nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveUniqueLabel(mockClient, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveUniqueLabel() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("resolveUniqueLabel() got = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestResolveUniqueLabelError(t *testing.T) {
	mockErrClient := &mailclient.MockMailClient{
		Err: errors.New("API error"),
	}
	_, err := resolveUniqueLabel(mockErrClient, "inbox")
	if err == nil {
		t.Error("expected error from client failure, got nil")
	}
}

func TestSearchLabels(t *testing.T) {
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
	items := []cfg_acc.LabelItem{
		{Name: "INBOX", FullName: "INBOX"},
		{Name: "Work/ProjectA", FullName: "Work/ProjectA"},
		{Name: "Work/ProjectB", FullName: "Work/ProjectB"},
		{Name: "Personal/Finance", FullName: "Personal/Finance"},
	}
	err = cs.SaveLabelItems(items)
	if err != nil {
		t.Fatalf("failed to save label items: %v", err)
	}

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{"Single match", []string{"Finance"}, []string{"Personal/Finance"}},
		{"Conjunction match", []string{"Work", "ProjectA"}, []string{"Work/ProjectA"}},
		{"Multiple matches", []string{"Project"}, []string{"Work/ProjectA", "Work/ProjectB"}},
		{"No match", []string{"Work", "Finance"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SearchLabels(config, tt.patterns)
			if err != nil {
				t.Fatalf("SearchLabels() returned unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SearchLabels() returned %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("SearchLabels() got[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}
