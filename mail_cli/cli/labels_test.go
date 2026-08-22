package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

func TestLabelsListFull(t *testing.T) {
	mockClient := &mailclient.MockMailClient{
		LabelItems: []cfg_acc.LabelItem{
			{Name: "Inbox", FullName: "Inbox", MessagesTotal: 5},
			{Name: "wuna", FullName: "sort/mailing_list/wuna", MessagesTotal: 2},
			{Name: "empty", FullName: "sort/empty", MessagesTotal: 0},
		},
		Cfg: &cfg_g.Config{},
	}

	session := &app.Session{
		Config: mockClient.Cfg,
		GetClient: func(cfg *cfg_g.Config) (mailclient.MailClient, error) {
			return mockClient, nil
		},
	}

	cliApp := InitCLI(session)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cliApp.Execute([]string{"labels", "list", "full"})

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error running labels list full: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Inbox") {
		t.Errorf("expected output to contain Inbox, got %q", output)
	}
	if !strings.Contains(output, "sort/mailing_list/wuna") {
		t.Errorf("expected output to contain sort/mailing_list/wuna, got %q", output)
	}
	if strings.Contains(output, "sort/empty") {
		t.Errorf("expected zero-count label sort/empty to be hidden when -a is not passed, got %q", output)
	}

	// Now test with -a (all labels)
	app.FlagLabelsListAll = false
	oldStdout = os.Stdout
	r2, w2, _ := os.Pipe()
	os.Stdout = w2

	err = cliApp.Execute([]string{"labels", "list", "full", "-a"})

	_ = w2.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error running labels list full -a: %v", err)
	}

	var buf2 bytes.Buffer
	_, _ = io.Copy(&buf2, r2)
	outputAll := buf2.String()

	if !strings.Contains(outputAll, "sort/empty") {
		t.Errorf("expected output to contain sort/empty when -a is passed, got %q", outputAll)
	}

	// Test labels list full ext
	app.FlagLabelsListAll = false
	mockClient.LabelItems = []cfg_acc.LabelItem{
		{Name: "Inbox", FullName: "Inbox", MessagesUnread: 2, MessagesTotal: 10},
		{Name: "wuna", FullName: "sort/mailing_list/wuna", MessagesUnread: 0, MessagesTotal: 5},
	}

	oldStdout = os.Stdout
	r3, w3, _ := os.Pipe()
	os.Stdout = w3

	err = cliApp.Execute([]string{"labels", "list", "full", "ext"})

	_ = w3.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error running labels list full ext: %v", err)
	}

	var buf3 bytes.Buffer
	_, _ = io.Copy(&buf3, r3)
	outputExt := buf3.String()

	if !strings.Contains(outputExt, "Inbox (2/10)") {
		t.Errorf("expected output to contain 'Inbox (2/10)', got %q", outputExt)
	}
	if !strings.Contains(outputExt, "sort/mailing_list/wuna (0/5)") {
		t.Errorf("expected output to contain 'sort/mailing_list/wuna (0/5)', got %q", outputExt)
	}

	// Test labels list full ext nozero (filters out zero-message folders)
	app.FlagLabelsListAll = false
	mockClient.LabelItems = []cfg_acc.LabelItem{
		{Name: "Inbox", FullName: "Inbox", MessagesUnread: 2, MessagesTotal: 10},
		{Name: "wuna", FullName: "sort/mailing_list/wuna", MessagesUnread: 0, MessagesTotal: 5},
		{Name: "empty", FullName: "sort/empty", MessagesUnread: 0, MessagesTotal: 0},
	}

	oldStdout = os.Stdout
	r4, w4, _ := os.Pipe()
	os.Stdout = w4

	err = cliApp.Execute([]string{"labels", "list", "full", "ext", "nozero"})

	_ = w4.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error running labels list full ext nozero: %v", err)
	}

	var buf4 bytes.Buffer
	_, _ = io.Copy(&buf4, r4)
	outputNoZero := buf4.String()

	if !strings.Contains(outputNoZero, "Inbox (2/10)") {
		t.Errorf("expected output to contain 'Inbox (2/10)', got %q", outputNoZero)
	}
	if !strings.Contains(outputNoZero, "sort/mailing_list/wuna (0/5)") {
		t.Errorf("expected output to contain 'sort/mailing_list/wuna (0/5)', got %q", outputNoZero)
	}
	if strings.Contains(outputNoZero, "sort/empty") {
		t.Errorf("expected zero-message folder 'sort/empty' to be omitted when 'nozero' is passed, got %q", outputNoZero)
	}
}
