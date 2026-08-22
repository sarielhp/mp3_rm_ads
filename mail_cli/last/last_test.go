package last

import (
	"fmt"
	"os"
	"testing"
	"time"

	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestVirtualMailbox_SaveLoad(t *testing.T) {
	tempDir := t.TempDir()

	vm := &VirtualMailbox{
		Name:        "test-virtual",
		Description: "A test virtual mailbox",
		MessageIDs:  []string{"msg1", "msg2", "msg3"},
		FolderMap: map[string]string{
			"msg1": "INBOX",
			"msg2": "Sent",
			"msg3": "Archive",
		},
	}

	if err := Save(tempDir, vm); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := Load(tempDir, "test-virtual")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Name != vm.Name {
		t.Errorf("expected name %q, got %q", vm.Name, loaded.Name)
	}
	if len(loaded.MessageIDs) != 3 {
		t.Errorf("expected 3 message IDs, got %d", len(loaded.MessageIDs))
	}
	if loaded.FolderMap["msg2"] != "Sent" {
		t.Errorf("expected FolderMap[msg2] = 'Sent', got %q", loaded.FolderMap["msg2"])
	}

	folder := FindMessageFolder(tempDir, "msg3")
	if folder != "Archive" {
		t.Errorf("expected FindMessageFolder('msg3') = 'Archive', got %q", folder)
	}
}

func TestPerform_LastN(t *testing.T) {
	tempDir := t.TempDir()

	// Store dummy messages in msg cache
	now := time.Now()
	for i := 1; i <= 5; i++ {
		msgID := fmt.Sprintf("id%d", i)
		raw := []byte(fmt.Sprintf("From: sender%d@example.com\r\nSubject: Test Email %d\r\nDate: %s\r\n\r\nBody %d",
			i, i, now.Add(time.Duration(i)*time.Hour).Format(time.RFC1123Z), i))
		if err := msg.Store(tempDir, msgID, raw, now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("msg.Store failed: %v", err)
		}
	}

	mock := &mailclient.MockMailClient{
		LabelItems: []cfg_acc.LabelItem{
			{Name: "INBOX", FullName: "INBOX", MessagesTotal: 3},
			{Name: "Work", FullName: "Work", MessagesTotal: 1},
			{Name: "Archive", FullName: "Archive", MessagesTotal: 1},
		},
		AccountLatestRefs: []cfg_acc.MessageFolderRef{
			{MessageID: "id5", Folder: "INBOX"},
			{MessageID: "id4", Folder: "Archive"},
			{MessageID: "id3", Folder: "INBOX"},
			{MessageID: "id2", Folder: "Work"},
			{MessageID: "id1", Folder: "INBOX"},
		},
		DownloadedIDs: map[string][]string{
			"INBOX":   {"id1", "id3", "id5"},
			"Work":    {"id2"},
			"Archive": {"id4"},
		},
	}

	config := &cfg_g.Config{
		DownloadDir: tempDir,
		Accounts: []cfg_acc.AccountConfig{
			{
				Name: "default",
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Perform(mock, config, 3)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Perform() error = %v", err)
	}

	var buf [4096]byte
	nRead, _ := r.Read(buf[:])
	output := string(buf[:nRead])

	if nRead == 0 {
		t.Errorf("expected non-empty output from Perform()")
	}
	_ = output

	// Check virtual mailbox was created
	vm, err := Load(tempDir, "last")
	if err != nil {
		t.Fatalf("expected 'last' virtual mailbox to exist: %v", err)
	}
	if len(vm.MessageIDs) != 3 {
		t.Errorf("expected 3 messages in virtual mailbox, got %d", len(vm.MessageIDs))
	}
	// The 3 most recent should be id3, id4, id5
	expectedIDs := map[string]bool{"id3": true, "id4": true, "id5": true}
	for _, id := range vm.MessageIDs {
		if !expectedIDs[id] {
			t.Errorf("unexpected message ID %q in virtual mailbox", id)
		}
	}
}
