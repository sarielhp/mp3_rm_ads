package main

import (
	"os"
	"testing"
)

func TestParseFlagsDownloadAll(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tests := []struct {
		args        []string
		expectedCmd string
		downloadAll bool
	}{
		{
			args:        []string{"abs", "sync", "download", "--all"},
			expectedCmd: "sync",
			downloadAll: true,
		},
		{
			args:        []string{"abs", "sync", "scan", "--all"},
			expectedCmd: "sync",
			downloadAll: true,
		},
		{
			args:        []string{"abs", "sync", "new", "--all"},
			expectedCmd: "sync",
			downloadAll: true,
		},
	}

	for _, tt := range tests {
		os.Args = tt.args
		action, opts := parseFlags()
		if action != tt.expectedCmd {
			t.Errorf("for args %v: expected action %s, got %s", tt.args, tt.expectedCmd, action)
		}
		if opts.DownloadAll != tt.downloadAll {
			t.Errorf("for args %v: expected DownloadAll=%v, got %v", tt.args, tt.downloadAll, opts.DownloadAll)
		}
	}
}
