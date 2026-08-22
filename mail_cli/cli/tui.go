package cli

import (
	"fmt"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// TuiCmd returns the clihelp.Command for the tui command.
func TuiCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "tui",
		Description: "Open the interactive terminal email browser. With an optional label_prefix argument, open the TUI with the matching label as the initial folder. The prefix is matched case-insensitively as a substring against the full label path. If exactly one label matches, the TUI opens on that label. If multiple match, all matching labels are printed and the program exits.",
		UsageLine:   "mail_cli tui [label_prefix]",
		Parameters: []clihelp.Param{
			{Name: "[label_prefix]", Description: "Optional folder/label prefix to open directly into."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli tui"},
			{Line: "mail_cli tui receipts"},
		},
		Args: clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			if session.InitTUI == nil {
				return fmt.Errorf("tui not configured")
			}

			labelPrefix := ""
			if len(args) > 0 && args[0] != "" {
				matches, err := SearchLabels(session.Config, []string{args[0]})
				if err != nil {
					fmt.Printf("%s Labels cache not available; opening TUI without folder filter\n", app.PrefixInfo)
				} else if len(matches) == 0 {
					return fmt.Errorf("no folder matches %q", args[0])
				} else {
					resolved := ResolveUniqueMatch(args[0], matches)
					if resolved != "" {
						labelPrefix = resolved
					} else {
						fmt.Printf("%d folders match %q:\n", len(matches), args[0])
						for _, m := range matches {
							fmt.Println(m)
						}
						return nil
					}
				}
			}

			return session.InitTUI(session.Config, labelPrefix)
		},
	}
}
