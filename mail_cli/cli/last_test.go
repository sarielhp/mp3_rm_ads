package cli

import (
	"testing"

	"mail_cli/app"
	"mail_cli/cfg_g"
)

func TestLastCmd_Validation(t *testing.T) {
	called := false
	var calledN int

	session := &app.Session{
		Config: &cfg_g.Config{},
		RunLast: func(cfg *cfg_g.Config, n int) error {
			called = true
			calledN = n
			return nil
		},
	}

	cliApp := InitCLI(session)

	// Test valid integer
	err := cliApp.Execute([]string{"last", "15"})
	if err != nil {
		t.Fatalf("expected no error for 'last 15', got %v", err)
	}
	if !called || calledN != 15 {
		t.Errorf("expected RunLast to be called with 15, called=%v, calledN=%d", called, calledN)
	}

	// Test invalid non-integer argument
	err = cliApp.Execute([]string{"last", "abc"})
	if err == nil {
		t.Errorf("expected error for 'last abc', got nil")
	}

	// Test invalid 0 or negative argument
	err = cliApp.Execute([]string{"last", "0"})
	if err == nil {
		t.Errorf("expected error for 'last 0', got nil")
	}
}
