package main

import (
	"testing"
)

func TestCLIScanAndNewCommands(t *testing.T) {
	var action string
	var opts CLIOptions

	app := buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"scan"}); err != nil {
		t.Fatalf("scan execution error: %v", err)
	}
	if action != "server" || opts.ServerSubcmd != "scan" {
		t.Errorf("scan command failed: action=%s, subcmd=%s", action, opts.ServerSubcmd)
	}
	if opts.EpisodesOnly {
		t.Errorf("scan without flags should not be EpisodesOnly")
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"scan", "-k", "5"}); err != nil {
		t.Fatalf("scan -k 5 error: %v", err)
	}
	if !opts.CountGiven || opts.Count != 5 {
		t.Errorf("scan -k 5 override failed: CountGiven=%v, Count=%d", opts.CountGiven, opts.Count)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"new"}); err != nil {
		t.Fatalf("new execution error: %v", err)
	}
	if action != "server" || opts.ServerSubcmd != "scan" || !opts.EpisodesOnly {
		t.Errorf("new command failed: action=%s, subcmd=%s, EpisodesOnly=%v", action, opts.ServerSubcmd, opts.EpisodesOnly)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"new", "-k", "2"}); err != nil {
		t.Fatalf("new -k 2 error: %v", err)
	}
	if !opts.CountGiven || opts.Count != 2 || !opts.EpisodesOnly {
		t.Errorf("new -k 2 override failed: CountGiven=%v, Count=%d, EpisodesOnly=%v", opts.CountGiven, opts.Count, opts.EpisodesOnly)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"server", "new", "-p", "Tech"}); err != nil {
		t.Fatalf("server new error: %v", err)
	}
	if action != "server" || opts.ServerSubcmd != "scan" || !opts.EpisodesOnly || opts.Podcast != "Tech" {
		t.Errorf("server new failed: action=%s, subcmd=%s, podcast=%s", action, opts.ServerSubcmd, opts.Podcast)
	}
}
