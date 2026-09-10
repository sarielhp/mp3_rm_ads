package cli

import (
	"abs/pkg/tui"
	"fmt"
	"os"
)

func Execute(args []string) int {
	action, cli, err := parseFlagsArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if action == "" {
		return 0
	}
	ensureConfigExists()
	config := loadConfig()

	if handled, hErr := handleParityCommands(action, config, cli); handled {
		if hErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", hErr)
			return 1
		}
		return 0
	}

	switch action {
	case "config":
		if err := handleMainConfig(&config, cli); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	case "tui":
		if err := tui.RunTUI(&config, cli.PodcastsDir); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			return 1
		}
	case "server", "sync":
		if err := handleServerCommand(config, cli); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	case "offload":
		handleRemoteCommand(config, cli)
	case "rm_ads":
		if err := handleMainProc(config, cli, action); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}
	return 0
}
