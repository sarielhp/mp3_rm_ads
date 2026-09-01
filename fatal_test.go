package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFatalError(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		fatalError("This is a %s error", "fatal")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalError")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("process ran with err %v, want exit status 1", err)
	}

	if !strings.Contains(string(out), "This is a fatal error") {
		t.Errorf("expected output to contain 'This is a fatal error', got %q", string(out))
	}
}
