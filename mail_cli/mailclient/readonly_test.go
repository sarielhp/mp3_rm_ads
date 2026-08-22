package mailclient

import (
	"errors"
	"testing"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

func TestReadOnlyMailClient_PassThrough(t *testing.T) {
	cfg := &cfg_g.Config{DownloadDir: "/tmp/test", Username: "user@example.com"}
	mock := &MockMailClient{
		Cfg:            cfg,
		Labels:         []string{"INBOX", "Archive", "Spam"},
		LabelItems:     []cfg_acc.LabelItem{{Name: "INBOX", FullName: "INBOX"}},
		InboxFolderVal: "INBOX",
		BackendTypeVal: "jmap",
		DownloadedIDs: map[string][]string{
			"INBOX": {"msg1", "msg2"},
		},
	}

	ro := NewReadOnlyMailClient(mock, "test-acc")

	if err := ro.Validate(); err != nil {
		t.Fatalf("unexpected Validate error: %v", err)
	}

	if ro.InboxFolder() != "INBOX" {
		t.Errorf("expected INBOX, got %q", ro.InboxFolder())
	}

	if ro.BackendType() != "jmap" {
		t.Errorf("expected jmap, got %q", ro.BackendType())
	}

	if ro.Config() != cfg {
		t.Errorf("expected config pass-through")
	}

	labels, err := ro.GetMatchingLabels("IN")
	if err != nil || len(labels) != 1 || labels[0] != "INBOX" {
		t.Errorf("unexpected GetMatchingLabels result: %v, err: %v", labels, err)
	}

	items, err := ro.GetLabelItems()
	if err != nil || len(items) != 1 {
		t.Errorf("unexpected GetLabelItems result: %v, err: %v", items, err)
	}

	ids, err := ro.FetchAndDownloadEmails("INBOX", "inbox")
	if err != nil || len(ids) != 2 {
		t.Errorf("unexpected FetchAndDownloadEmails result: %v, err: %v", ids, err)
	}

	if err := ro.ListLabels(); err != nil {
		t.Errorf("unexpected ListLabels error: %v", err)
	}

	if err := ro.ExportRules(); err != nil {
		t.Errorf("unexpected ExportRules error: %v", err)
	}

	if err := ro.ListFilters(); err != nil {
		t.Errorf("unexpected ListFilters error: %v", err)
	}

	if ro.RewriteRuleLabelCasing("Test") != "" {
		t.Errorf("unexpected RewriteRuleLabelCasing output")
	}
}

func TestReadOnlyMailClient_NoOps(t *testing.T) {
	mock := &MockMailClient{}
	ro := NewReadOnlyMailClient(mock, "test-acc")

	// MoveEmail should be simulated no-op and not modify mock
	if err := ro.MoveEmail([]string{"msg1"}, "INBOX", "Archive"); err != nil {
		t.Errorf("expected nil error on simulated MoveEmail, got: %v", err)
	}
	if len(mock.MovedEmails) != 0 {
		t.Errorf("expected no delegate MoveEmail call, got %d calls", len(mock.MovedEmails))
	}

	// CopyEmail should be simulated no-op
	if err := ro.CopyEmail([]string{"msg1"}, "INBOX", "Archive"); err != nil {
		t.Errorf("expected nil error on simulated CopyEmail, got: %v", err)
	}
	if len(mock.CopiedEmails) != 0 {
		t.Errorf("expected no delegate CopyEmail call")
	}

	// MoveToInbox
	if err := ro.MoveToInbox([]string{"msg1"}, "Spam"); err != nil {
		t.Errorf("expected nil error on simulated MoveToInbox, got: %v", err)
	}

	// MarkAsRead
	if err := ro.MarkAsRead([]string{"msg1"}); err != nil {
		t.Errorf("expected nil error on simulated MarkAsRead, got: %v", err)
	}

	// ReportSpam
	if err := ro.ReportSpam([]string{"msg1"}, "INBOX"); err != nil {
		t.Errorf("expected nil error on simulated ReportSpam, got: %v", err)
	}

	// MoveAllSpam
	if err := ro.MoveAllSpam("Spam"); err != nil {
		t.Errorf("expected nil error on simulated MoveAllSpam, got: %v", err)
	}

	// LearnSpam
	if err := ro.LearnSpam(); err != nil {
		t.Errorf("expected nil error on simulated LearnSpam, got: %v", err)
	}

	// RenameLabel
	if err := ro.RenameLabel("old", "new"); err != nil {
		t.Errorf("expected nil error on simulated RenameLabel, got: %v", err)
	}

	// FixLabels
	if err := ro.FixLabels(); err != nil {
		t.Errorf("expected nil error on simulated FixLabels, got: %v", err)
	}

	// EnsureLabelExists
	if err := ro.EnsureLabelExists("newLabel"); err != nil {
		t.Errorf("expected nil error on simulated EnsureLabelExists, got: %v", err)
	}

	// CheckAndApplyRules
	ruleIds, err := ro.CheckAndApplyRules([]string{"msg1", "msg2"}, "INBOX", "inbox")
	if err != nil || len(ruleIds) != 2 {
		t.Errorf("unexpected CheckAndApplyRules result: %v, err: %v", ruleIds, err)
	}
}

func TestReadOnlyMailClient_BlockedMutations(t *testing.T) {
	mock := &MockMailClient{}
	ro := NewReadOnlyMailClient(mock, "test-acc")

	// DeleteAllSpam
	if err := ro.DeleteAllSpam(); !errors.Is(err, ErrReadOnlyOperationBlocked) {
		t.Errorf("expected ErrReadOnlyOperationBlocked on DeleteAllSpam, got: %v", err)
	}

	// DeleteLabel
	if err := ro.DeleteLabel("old"); !errors.Is(err, ErrReadOnlyOperationBlocked) {
		t.Errorf("expected ErrReadOnlyOperationBlocked on DeleteLabel, got: %v", err)
	}

	// UploadRawEmail
	if err := ro.UploadRawEmail([]byte("dummy"), "INBOX"); !errors.Is(err, ErrReadOnlyOperationBlocked) {
		t.Errorf("expected ErrReadOnlyOperationBlocked on UploadRawEmail, got: %v", err)
	}

	// SendEmail
	if err := ro.SendEmail([]byte("dummy")); !errors.Is(err, ErrReadOnlyOperationBlocked) {
		t.Errorf("expected ErrReadOnlyOperationBlocked on SendEmail, got: %v", err)
	}
}
