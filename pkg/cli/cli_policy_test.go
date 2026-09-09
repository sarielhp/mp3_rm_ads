package cli

import (
	"testing"
)

func TestCLIScanAndNewCommands(t *testing.T) {
	var action string
	var opts CLIOptions

	app := buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"sync"}); err != nil {
		t.Fatalf("sync execution error: %v", err)
	}
	if action != "sync" {
		t.Errorf("sync command failed: action=%s", action)
	}
	if opts.EpisodesOnly {
		t.Errorf("sync without flags should not be EpisodesOnly")
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"sync", "-k", "5"}); err != nil {
		t.Fatalf("sync -k 5 error: %v", err)
	}
	if !opts.CountGiven || opts.Count != 5 {
		t.Errorf("sync -k 5 override failed: CountGiven=%v, Count=%d", opts.CountGiven, opts.Count)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"sync", "--episodes-only"}); err != nil {
		t.Fatalf("sync --episodes-only execution error: %v", err)
	}
	if action != "sync" || !opts.EpisodesOnly {
		t.Errorf("sync --episodes-only failed: action=%s, EpisodesOnly=%v", action, opts.EpisodesOnly)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"sync", "--episodes-only", "-k", "2"}); err != nil {
		t.Fatalf("sync -k 2 error: %v", err)
	}
	if !opts.CountGiven || opts.Count != 2 || !opts.EpisodesOnly {
		t.Errorf("sync -k 2 override failed: CountGiven=%v, Count=%d, EpisodesOnly=%v", opts.CountGiven, opts.Count, opts.EpisodesOnly)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"sync", "--episodes-only", "-p", "Tech"}); err != nil {
		t.Fatalf("sync error: %v", err)
	}
	if action != "sync" || !opts.EpisodesOnly || opts.Podcast != "Tech" {
		t.Errorf("sync failed: action=%s, podcast=%s", action, opts.Podcast)
	}
}
