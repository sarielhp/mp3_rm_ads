package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestPerformPatternMatching(t *testing.T) {
	tmpDir := t.TempDir()
	downloadDir := filepath.Join(tmpDir, "download")
	_ = os.MkdirAll(downloadDir, 0755)

	// Create fake emails under download/test-account
	accountDir := filepath.Join(downloadDir, "test-account")
	_ = os.MkdirAll(accountDir, 0755)

	// Store fake emails using msg.Store so the compact binary is updated
	err := msg.Store(accountDir, "msg1", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: bob@example.com\nSubject: Urgent Invoice\n\nbody1"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg1: %v", err)
	}
	err = msg.Store(accountDir, "msg2", []byte("Date: Mon, 2 Jan 2026 15:04:05 -0700\nFrom: alice@example.com\nSubject: Meeting Update\n\nbody2"), time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("failed to store msg2: %v", err)
	}

	// Set classification so they are treated as cached (skips calling bogofilter executable)
	err = msg.SetClassification(accountDir, "msg1", false, false, false, 0.0)
	if err != nil {
		t.Fatalf("failed to set classification for msg1: %v", err)
	}
	err = msg.SetClassification(accountDir, "msg2", false, false, false, 0.0)
	if err != nil {
		t.Fatalf("failed to set classification for msg2: %v", err)
	}

	mockClient := &mailclient.MockMailClient{
		Labels: []string{"received"},
		DownloadedIDs: map[string][]string{
			"received": {"msg1", "msg2"},
		},
		Cfg: &cfg_g.Config{
			DownloadDir:     accountDir,
			ConfigDir:       tmpDir,
			SelectedAccount: "test-account",
		},
	}

	// 1. With pattern "Invoice"
	app.FlagScanPattern = "Invoice"
	_, err = Perform(mockClient, mockClient.Cfg, "received", "", "")
	if err != nil {
		t.Fatalf("Perform failed: %v", err)
	}

	// 2. With pattern "Nonexistent" -> should not find any messages
	app.FlagScanPattern = "Nonexistent"
	_, err = Perform(mockClient, mockClient.Cfg, "received", "", "")
	if err != nil {
		t.Fatalf("Perform failed: %v", err)
	}

	// Reset global flag
	app.FlagScanPattern = ""
}
