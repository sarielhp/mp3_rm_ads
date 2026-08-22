package cli

import (
	"fmt"
	"strconv"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// LastCmd returns the clihelp.Command for the last command.
func LastCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "last",
		Description: "Check all folders in the account, collect the last N emails received, save them to a virtual mailbox, and print each email with its folder name.",
		UsageLine:   "mail_cli last <N>",
		Parameters: []clihelp.Param{
			{Name: "<N>", Description: "Number of most recent emails to retrieve and display."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli last 10"},
			{Line: "mail_cli last 25"},
			{Line: "mail_cli -A work-jmap last 20"},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			n, err := strconv.Atoi(args[0])
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid number of emails %q (must be a positive integer)", args[0])
			}
			if session.RunLast == nil {
				return ErrConfigNotConfigured("last")
			}
			return session.RunLast(session.Config, n)
		},
	}
}
