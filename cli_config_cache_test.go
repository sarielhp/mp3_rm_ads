package main

import (
	"os"
	"testing"
)

func TestConfigCacheRequiresAnExplicitDestructiveAction(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	cases := []struct {
		args    []string
		wantCmd string
		wantErr bool
	}{
		{[]string{"abs", "config", "cache"}, "cache-show", false},
		{[]string{"abs", "config", "cache", "show"}, "cache-show", false},
		{[]string{"abs", "config", "cache", "clear"}, "cache-reset", false},
		{[]string{"abs", "config", "cache", "reset"}, "cache-reset", false},
		{[]string{"abs", "config", "cache", "bogus"}, "", true},
	}

	for _, tc := range cases {
		var action string
		var opts CLIOptions
		app := buildCLIApp(&action, &opts)
		err := app.Execute(tc.args[1:])

		if tc.wantErr {
			if err == nil {
				t.Errorf("%v: expected an error, got ConfigCmd=%q", tc.args, opts.ConfigCmd)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: unexpected error %v", tc.args, err)
			continue
		}
		if opts.ConfigCmd != tc.wantCmd {
			t.Errorf("%v: ConfigCmd = %q, want %q (a bare `config cache` must not wipe the cache)",
				tc.args, opts.ConfigCmd, tc.wantCmd)
		}
	}
}
