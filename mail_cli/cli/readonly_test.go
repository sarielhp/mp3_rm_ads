package cli

import (
	"strings"
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_g"
)

func TestReadOnlyFlag_Setting(t *testing.T) {
	app.FlagReadOnly = false

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunScan: func(cfg *cfg_g.Config, labelPrefix, moveSpam, moveInbox string) (int, error) {
			return 0, nil
		},
	}

	cliApp := InitCLI(session)
	_ = cliApp.Execute([]string{"--read-only", "scan", "inbox"})

	if !session.Config.ReadOnly {
		t.Errorf("expected session.Config.ReadOnly to be true with --read-only flag")
	}

	// Reset and test --dry-run
	app.FlagReadOnly = false
	session.Config.ReadOnly = false
	_ = cliApp.Execute([]string{"--dry-run", "scan", "inbox"})

	if !session.Config.ReadOnly {
		t.Errorf("expected session.Config.ReadOnly to be true with --dry-run flag")
	}

	// Reset and test -ro
	app.FlagReadOnly = false
	session.Config.ReadOnly = false
	_ = cliApp.Execute([]string{"--ro", "scan", "inbox"})

	if !session.Config.ReadOnly {
		t.Errorf("expected session.Config.ReadOnly to be true with --ro flag")
	}
}

func TestReadOnly_SpamDelBlocked(t *testing.T) {
	session := &app.Session{
		Config: &cfg_g.Config{
			ReadOnly: true,
		},
	}

	cliApp := InitCLI(session)
	err := cliApp.Execute([]string{"spam", "del"})

	if err == nil {
		t.Fatal("expected error for spam del in read-only mode, got nil")
	}

	if !strings.Contains(err.Error(), "Cannot permanently delete messages while in read-only mode") {
		t.Errorf("unexpected error message: %v", err)
	}
}
