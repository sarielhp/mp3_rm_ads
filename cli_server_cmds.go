package main

import (
	"github.com/sarielhp/clihelp"
)

func buildServerCommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {
	return clihelp.Command{
		Name:        "server",
		Description: "Manage and interact with the Audiobookshelf server",
		Subcommands: buildServerSubcommands(opts, action, countVal, keepVal),
	}
}
