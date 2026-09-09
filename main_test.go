package main

import (
	"testing"

	"github.com/sariel/abs/pkg/cli"
)

func TestMainExecuteHelp(t *testing.T) {
	code := cli.Execute([]string{"--help"})
	if code != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", code)
	}
}
