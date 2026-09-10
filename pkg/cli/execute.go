package cli

import (
	"fmt"
	"os"

	"abs/pkg/tui"
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

	if err := dispatch(action, &config, cli); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// dispatch routes a parsed command to its handler. Every top-level command has
// exactly one case here, in the order the commands are registered. There used
// to be a second table, handleParityCommands, consulted first and holding an
// unrelated four of them.
func dispatch(action string, config *Config, cli CLIOptions) error {
	switch action {
	case "config":
		return runConfigCommand(config, cli)
	case "info":
		if cli.InfoSubcmd == "status" || cli.InfoSubcmd == "check" {
			return runStatusCommand(config, cli)
		}
		return runInfoCommand(*config, cli)
	case "offload":
		handleRemoteCommand(*config, cli)
		return nil
	case "player":
		return runPlayerCommand(*config, cli)
	case "queue":
		return runQueueCommand(*config, cli)
	case "rm_ads":
		return runRmAdsCommand(*config, cli, action)
	case "server", "sync":
		return handleServerCommand(*config, cli)
	case "tui":
		return tui.RunTUI(config, cli.PodcastsDir)
	}
	return nil
}
