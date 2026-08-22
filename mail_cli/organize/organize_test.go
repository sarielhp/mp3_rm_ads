package organize_test

import (
	"os"
	"path/filepath"
	"testing"

	"mail_cli/cfg_g"
	"mail_cli/mailclient"
	"mail_cli/organize"
)

func TestResolveArchiveTarget(t *testing.T) {
	tests := []struct {
		name           string
		labels         []string
		receivedFolder string
		want           string
		wantErr        bool
	}{
		{
			name:    "Archive exact match",
			labels:  []string{"Inbox", "Archive", "Spam"},
			want:    "Archive",
			wantErr: false,
		},
		{
			name:    "Archive case-insensitive exact match",
			labels:  []string{"Inbox", "archive", "Spam"},
			want:    "archive",
			wantErr: false,
		},
		{
			name:    "Archive suffix match",
			labels:  []string{"Inbox", "[Gmail]/Archive", "Spam"},
			want:    "[Gmail]/Archive",
			wantErr: false,
		},
		{
			name:           "Configured receivedFolder exact match",
			labels:         []string{"Inbox", "custom-received", "Spam"},
			receivedFolder: "custom-received",
			want:           "custom-received",
			wantErr:        false,
		},
		{
			name:           "Configured receivedFolder suffix match",
			labels:         []string{"Inbox", "Personal/custom-received", "Spam"},
			receivedFolder: "custom-received",
			want:           "Personal/custom-received",
			wantErr:        false,
		},
		{
			name:    "Default received exact match",
			labels:  []string{"Inbox", "Received", "Spam"},
			want:    "Received",
			wantErr: false,
		},
		{
			name:    "Default received case-insensitive match",
			labels:  []string{"Inbox", "received", "Spam"},
			want:    "received",
			wantErr: false,
		},
		{
			name:    "Default received suffix match",
			labels:  []string{"Inbox", "Mail/received", "Spam"},
			want:    "Mail/received",
			wantErr: false,
		},
		{
			name:    "Neither exists error",
			labels:  []string{"Inbox", "Spam", "Sent"},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mailclient.MockMailClient{
				Labels: tt.labels,
				Cfg: &cfg_g.Config{
					ReceivedFolder: tt.receivedFolder,
				},
			}
			got, err := organize.ResolveArchiveTarget(client)
			if (err != nil) != tt.wantErr {
				t.Fatalf("organize.ResolveArchiveTarget() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("organize.ResolveArchiveTarget() got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestArchiveAll(t *testing.T) {
	tempDir := t.TempDir()

	config := &cfg_g.Config{
		DownloadDir: tempDir,
		Verbose:     true,
	}

	client := &mailclient.MockMailClient{
		Labels: []string{"Inbox", "archive", "test/newsletter", "test/other", "spam"},
		Cfg:    config,
		DownloadedIDs: map[string][]string{
			"Inbox":           {"msg1", "msg2"},
			"test/newsletter": {"msg3"},
			"test/other":      {"msg4", "msg5"},
		},
	}

	err := organize.All(config, client, "inbox", "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.MovedEmails) != 1 {
		t.Fatalf("expected 1 move operation, got %d", len(client.MovedEmails))
	}
	move := client.MovedEmails[0]
	if move.SourceLabel != "Inbox" || move.DestLabel != "archive" {
		t.Errorf("unexpected move source/dest: got %s -> %s, want Inbox -> archive", move.SourceLabel, move.DestLabel)
	}
	if len(move.MessageIDs) != 2 || move.MessageIDs[0] != "msg1" || move.MessageIDs[1] != "msg2" {
		t.Errorf("unexpected message IDs moved: %v", move.MessageIDs)
	}

	client.MovedEmails = nil

	err = organize.All(config, client, "test", "archive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.MovedEmails) != 2 {
		t.Fatalf("expected 2 move operations, got %d", len(client.MovedEmails))
	}

	var foundNewsletter, foundOther bool
	for _, m := range client.MovedEmails {
		if m.SourceLabel == "test/newsletter" {
			foundNewsletter = true
			if len(m.MessageIDs) != 1 || m.MessageIDs[0] != "msg3" {
				t.Errorf("unexpected newsletter IDs: %v", m.MessageIDs)
			}
		} else if m.SourceLabel == "test/other" {
			foundOther = true
			if len(m.MessageIDs) != 2 || m.MessageIDs[0] != "msg4" || m.MessageIDs[1] != "msg5" {
				t.Errorf("unexpected other IDs: %v", m.MessageIDs)
			}
		} else {
			t.Errorf("unexpected source label: %s", m.SourceLabel)
		}
		if m.DestLabel != "archive" {
			t.Errorf("unexpected dest label: %s", m.DestLabel)
		}
	}

	if !foundNewsletter || !foundOther {
		t.Errorf("did not find expected newsletter and other moves: newsletter=%v, other=%v", foundNewsletter, foundOther)
	}
}

func TestResolveArchiveTargetGmailFallback(t *testing.T) {
	tempDir := t.TempDir()

	configContent := `{
		"accounts": [
			{
				"name": "gmail-test",
				"type": "gmail",
				"received_folder": "INBOX"
			}
		]
	}`
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write dummy config: %v", err)
	}

	config := &cfg_g.Config{
		ConfigDir:      tempDir,
		ReceivedFolder: "INBOX",
	}

	client := &mailclient.MockMailClient{
		Labels:         []string{"Inbox", "Spam", "Sent"},
		Cfg:            config,
		BackendTypeVal: "gmail",
	}

	got, err := organize.ResolveArchiveTarget(client)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "Archive" {
		t.Errorf("expected archive target 'Archive', got %q", got)
	}
}
