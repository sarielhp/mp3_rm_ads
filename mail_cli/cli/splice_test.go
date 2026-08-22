package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestSpliceDestPath(t *testing.T) {
	tests := []struct {
		source           string
		folderFlag       string
		folderSuffixFlag string
		year             int
		month            int
		want             string
	}{
		{
			source: "research/cfps",
			year:   2026, month: 6,
			want: "keep/2026/06/cfps",
		},
		{
			source: "research/cfps", folderFlag: "research",
			year: 2026, month: 6,
			want: "keep/2026/06/research",
		},
		{
			source: "research/cfps", folderSuffixFlag: "research",
			year: 2026, month: 6,
			want: "keep/2026/06/research-2026-06",
		},
		{
			source: "research/cfps", folderFlag: "archive",
			year: 2026, month: 6,
			want: "keep/2026/06/archive",
		},
		{
			source: "inbox",
			year:   2026, month: 6,
			want: "keep/2026/06/inbox",
		},
		{
			source: "a/b/c",
			year:   2026, month: 6,
			want: "keep/2026/06/c",
		},
		{
			source: "a/b/c", folderSuffixFlag: "foo",
			year: 2026, month: 6,
			want: "keep/2026/06/foo-2026-06",
		},
		{
			source: "inbox",
			year:   2024, month: 1,
			want: "keep/2024/01/inbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			monthPadded := strconv.Itoa(tt.month)
			if tt.month < 10 {
				monthPadded = "0" + monthPadded
			}
			folder := ""
			if tt.folderSuffixFlag != "" {
				folder = tt.folderSuffixFlag + "-" + strconv.Itoa(tt.year) + "-" + monthPadded
			} else if tt.folderFlag != "" {
				folder = tt.folderFlag
			} else {
				folder = filepath.Base(tt.source)
			}
			dest := filepath.Join("keep", strconv.Itoa(tt.year),
				monthPadded, folder)

			if dest != tt.want {
				t.Errorf("want %q, got %q", tt.want, dest)
			}
		})
	}
}

func TestSpliceValidation(t *testing.T) {
	tests := []struct {
		name    string
		folder  string
		numMsg  int
		wantErr bool
		errSub  string
	}{
		{
			name:   "valid default",
			folder: "test", numMsg: 10,
			wantErr: false,
		},
		{
			name:   "n = 0 → error",
			folder: "test", numMsg: 0,
			wantErr: true, errSub: "-n must be at least 1",
		},
		{
			name:   "folder = keep → error",
			folder: "keep", numMsg: 10,
			wantErr: true, errSub: "must not be \"keep\"",
		},
		{
			name:   "Keep (case-insensitive) → error",
			folder: "Keep", numMsg: 10,
			wantErr: true, errSub: "must not be \"keep\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpliceArgs(tt.folder, tt.numMsg, false)
			hasErr := err != nil
			if hasErr != tt.wantErr {
				t.Errorf("wantErr=%v got=%v, err=%v",
					tt.wantErr, hasErr, err)
			}
			if hasErr && tt.errSub != "" {
				if !strings.Contains(err.Error(), tt.errSub) {
					t.Errorf("error %q should contain %q",
						err.Error(), tt.errSub)
				}
			}
		})
	}
}

func init() {
	if runtime.GOOS != "linux" {
		os.Exit(0)
	}
}

func TestSplicePatternMatching(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "download")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(downloadDir, 0755)

	// Create fake emails under download/test-account
	accountDir := filepath.Join(downloadDir, "test-account")
	_ = os.MkdirAll(accountDir, 0755)

	// email 1: matches pattern
	err := msg.Store(accountDir, "msg1", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: bob@example.com\nSubject: Urgent Invoice\n\nbody1"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg1: %v", err)
	}
	// email 2: does not match
	err = msg.Store(accountDir, "msg2", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: alice@example.com\nSubject: Meeting Update\n\nbody2"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg2: %v", err)
	}

	mockClient := &mailclient.MockMailClient{
		Labels: []string{"Inbox"},
		DownloadedIDs: map[string][]string{
			"Inbox": {"msg1", "msg2"},
		},
		Cfg: &cfg_g.Config{
			DownloadDir:     accountDir,
			ConfigDir:       configDir,
			SelectedAccount: "test-account",
		},
	}

	session := &app.Session{
		Config: mockClient.Cfg,
		GetClient: func(cfg *cfg_g.Config) (mailclient.MailClient, error) {
			return mockClient, nil
		},
	}

	cliApp := InitCLI(session)

	// Run dry-run with pattern "*invoice*"
	err = cliApp.Execute([]string{"splice", "-p", "*invoice*", "-n", "10", "Inbox"})
	if err != nil {
		t.Fatalf("unexpected error running splice: %v", err)
	}

	// Run with actually move=true and -f custom to see which email gets moved without suffix
	mockClient.MovedEmails = nil
	err = cliApp.Execute([]string{"splice", "-p", "invoice", "--move", "-f", "wuna", "-n", "10", "Inbox"})
	if err != nil {
		t.Fatalf("unexpected error running splice move -f: %v", err)
	}

	if len(mockClient.MovedEmails) != 1 {
		t.Errorf("expected exactly 1 email to be moved, got %d", len(mockClient.MovedEmails))
	} else {
		record := mockClient.MovedEmails[0]
		if record.DestLabel != "keep/2026/01/wuna" {
			t.Errorf("expected 'keep/2026/01/wuna', got %q", record.DestLabel)
		}
	}

	// Run with actually move=true and -F custom to see which email gets moved with suffix
	mockClient.MovedEmails = nil
	err = cliApp.Execute([]string{"splice", "-p", "invoice", "--move", "-F", "wuna", "-n", "10", "Inbox"})
	if err != nil {
		t.Fatalf("unexpected error running splice move -F: %v", err)
	}

	if len(mockClient.MovedEmails) != 1 {
		t.Errorf("expected exactly 1 email to be moved, got %d", len(mockClient.MovedEmails))
	} else {
		record := mockClient.MovedEmails[0]
		if record.DestLabel != "keep/2026/01/wuna-2026-01" {
			t.Errorf("expected 'keep/2026/01/wuna-2026-01', got %q", record.DestLabel)
		}
	}
}

func TestSpliceAllowKeep(t *testing.T) {
	// Verify that with allow=true, ValidateSpliceArgs doesn't return an error for keep/ folders
	err := ValidateSpliceArgs("keep/archive", 10, true)
	if err != nil {
		t.Errorf("expected no error for keep/ folders when allow=true, got %v", err)
	}

	err = ValidateSpliceArgs("keep", 10, true)
	if err != nil {
		t.Errorf("expected no error for 'keep' folder when allow=true, got %v", err)
	}
}

func TestSpliceAccountIndexed(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "download")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(downloadDir, 0755)

	acc1Dir := filepath.Join(downloadDir, "acc1")
	acc2Dir := filepath.Join(downloadDir, "acc2")
	_ = os.MkdirAll(acc1Dir, 0755)
	_ = os.MkdirAll(acc2Dir, 0755)

	err := msg.Store(acc1Dir, "msg101", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: test@example.com\nSubject: Test Email\n\nbody"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg101: %v", err)
	}

	mockClient1 := &mailclient.MockMailClient{
		Labels: []string{"sort/mailing_list/wuna"},
		DownloadedIDs: map[string][]string{
			"sort/mailing_list/wuna": {"msg101"},
		},
		Cfg: &cfg_g.Config{
			DownloadDir: acc1Dir,
			ConfigDir:   configDir,
		},
	}
	mockClient2 := &mailclient.MockMailClient{
		Labels: []string{"wuna"},
		Cfg: &cfg_g.Config{
			DownloadDir: acc2Dir,
			ConfigDir:   configDir,
		},
	}

	session := &app.Session{
		Config: &cfg_g.Config{
			DownloadDir: downloadDir,
			ConfigDir:   configDir,
			Accounts: []cfg_acc.AccountConfig{
				{Name: "acc1", Type: "gmail"},
				{Name: "acc2", Type: "jmap"},
			},
		},
		GetClient: func(cfg *cfg_g.Config) (mailclient.MailClient, error) {
			if strings.EqualFold(cfg.SelectedAccount, "acc2") {
				return mockClient2, nil
			}
			return mockClient1, nil
		},
	}

	cliApp := InitCLI(session)

	// "acc1:sort/mailing_list/wuna" — splice from account "acc1" using account name
	err = cliApp.Execute([]string{"splice", "acc1:sort/mailing_list/wuna", "-F", "acc2:wuna", "--move"})
	if err != nil {
		t.Fatalf("unexpected error running splice acc1:folder -F acc2:folder: %v", err)
	}

	if len(mockClient2.UploadedEmails) != 1 {
		t.Errorf("expected 1 email uploaded to account 2, got %d", len(mockClient2.UploadedEmails))
	} else {
		up := mockClient2.UploadedEmails[0]
		if up.TargetLabel != "keep/2026/01/wuna-2026-01" {
			t.Errorf("expected target label keep/2026/01/wuna-2026-01, got %q", up.TargetLabel)
		}
	}

	// Test splicing with short suffix "acc1:wuna" matching sort/mailing_list/wuna
	mockClient2.UploadedEmails = nil
	mockClient2.DownloadedIDs = nil
	err = cliApp.Execute([]string{"splice", "acc1:wuna", "-F", "acc2:wuna", "--move"})
	if err != nil {
		t.Fatalf("unexpected error running splice acc1:wuna -F acc2:wuna: %v", err)
	}

	if len(mockClient2.UploadedEmails) != 1 {
		t.Errorf("expected 1 email uploaded to account 2 when using '1:wuna', got %d", len(mockClient2.UploadedEmails))
	}

	// Test splicing with --move when message already exists in destination (should not re-upload)
	mockClient2.UploadedEmails = nil
	if mockClient2.DownloadedIDs == nil {
		mockClient2.DownloadedIDs = make(map[string][]string)
	}
	mockClient2.DownloadedIDs["keep/2026/01/wuna-2026-01"] = []string{"msg101"}

	err = cliApp.Execute([]string{"splice", "acc1:wuna", "-F", "acc2:wuna", "--move"})
	if err != nil {
		t.Fatalf("unexpected error running splice acc1:wuna -F acc2:wuna --move on existing msg: %v", err)
	}
	if len(mockClient2.UploadedEmails) != 0 {
		t.Errorf("expected 0 emails uploaded when already in destination, got %d", len(mockClient2.UploadedEmails))
	}

	// Test splicing with -Y option (drops month directory path, e.g. keep/2026/received-2026-01)
	mockClient2.UploadedEmails = nil
	mockClient2.DownloadedIDs = nil
	err = cliApp.Execute([]string{"splice", "acc1:wuna", "-Y", "acc2:received", "--move"})
	if err != nil {
		t.Fatalf("unexpected error running splice acc1:wuna -Y acc2:received: %v", err)
	}
	if len(mockClient2.UploadedEmails) != 1 {
		t.Fatalf("expected 1 email uploaded to account 2 when using '-Y 2:received', got %d", len(mockClient2.UploadedEmails))
	}
	if mockClient2.UploadedEmails[0].TargetLabel != "keep/2026/received-2026-01" {
		t.Errorf("expected destination folder 'keep/2026/received-2026-01', got %q", mockClient2.UploadedEmails[0].TargetLabel)
	}
}

func TestSpliceCopyOption(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "download")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(downloadDir, 0755)

	err := msg.Store(downloadDir, "msg201", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: test@example.com\nSubject: Copy Test\n\nbody"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg201: %v", err)
	}

	mockClient := &mailclient.MockMailClient{
		Labels: []string{"Inbox"},
		DownloadedIDs: map[string][]string{
			"Inbox": {"msg201"},
		},
		Cfg: &cfg_g.Config{
			DownloadDir: downloadDir,
			ConfigDir:   configDir,
		},
	}

	session := &app.Session{
		Config: mockClient.Cfg,
		GetClient: func(cfg *cfg_g.Config) (mailclient.MailClient, error) {
			return mockClient, nil
		},
	}

	cliApp := InitCLI(session)

	// Test error when passing both --move and --copy
	err = cliApp.Execute([]string{"splice", "Inbox", "--move", "--copy"})
	if err == nil {
		t.Fatalf("expected error when passing both --move and --copy")
	}

	// Test copy execution
	err = cliApp.Execute([]string{"splice", "Inbox", "-f", "wuna", "--copy"})
	if err != nil {
		t.Fatalf("unexpected error running splice --copy: %v", err)
	}

	if len(mockClient.CopiedEmails) != 1 {
		t.Fatalf("expected 1 copied email record, got %d", len(mockClient.CopiedEmails))
	}
	if mockClient.CopiedEmails[0].DestLabel != "keep/2026/01/wuna" {
		t.Errorf("expected dest label keep/2026/01/wuna, got %q", mockClient.CopiedEmails[0].DestLabel)
	}

	// Test skipping copy if message already exists in destination
	mockClient.DownloadedIDs["keep/2026/01/wuna"] = []string{"msg201"}
	mockClient.CopiedEmails = nil

	err = cliApp.Execute([]string{"splice", "Inbox", "-f", "wuna", "--copy"})
	if err != nil {
		t.Fatalf("unexpected error running splice --copy on existing message: %v", err)
	}
	if len(mockClient.CopiedEmails) != 0 {
		t.Errorf("expected 0 copied email records when message already exists, got %d", len(mockClient.CopiedEmails))
	}
}
