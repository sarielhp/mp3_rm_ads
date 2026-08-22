package cli

import (
	"fmt"
	"strings"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
	"github.com/spf13/pflag"
)

// ScanCmd returns the clihelp.Command for the scan command.
func ScanCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "scan",
		Description: "Scan all folders starting with the given label prefix (case-insensitive) for spam.",
		UsageLine:   "mail_cli scan <lbl_prefix> [flags]",
		Parameters: []clihelp.Param{
			{Name: "<lbl_prefix>", Description: "The prefix of the label/folder to scan (e.g. 'inbox' or 'receipts')."},
		},
		Options: []clihelp.Option{
			{
				Flags:       "-m, --move [From]",
				Description: "Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message.",
				Binder: func(fs *pflag.FlagSet) error {
					fs.StringVarP(&app.FlagMoveSpamStr, "move", "m", "", "Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message.")
					if f := fs.Lookup("move"); f != nil {
						f.NoOptDefVal = "true"
					}
					return nil
				},
			},
			clihelp.String(&app.FlagMoveInboxStr, "--inbox-move <From>", "", "Move identified emails from a specific From address back to the Inbox folder."),
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli scan inbox"},
			{Line: "mail_cli scan inbox -m"},
			{Line: "mail_cli scan receipts -m spammer@example.com"},
		},
		Args: clihelp.MaximumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			if len(args) == 0 {
				ctx.App.RenderCommand(clihelp.Options{Writer: ctx.Stdout}, "scan")
				return nil
			}
			if session.PreCheck != nil {
				if err := session.PreCheck(session.Config); err != nil {
					return err
				}
			}
			labelPrefix := args[0]
			if strings.EqualFold(labelPrefix, "inbox") {
				app.FlagExplicitScanInbox = true
			}
			if session.RunScan == nil {
				return ErrConfigNotConfigured("scan")
			}
			moved, err := session.RunScan(session.Config, labelPrefix, app.FlagMoveSpamStr, app.FlagMoveInboxStr)
			if err != nil {
				return err
			}
			if moved > 0 {
				fmt.Printf("\nTotal emails moved during scan: %s\n", app.ColorBoldYellow.Sprint(moved))
			}
			return nil
		},
	}
}

// ShowCmd returns the clihelp.Command for the show command.
func ShowCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "show",
		Description: "Show the contents of emails in folders matching a label prefix, or show a specific email's details and body without running spam checks.",
		UsageLine:   "mail_cli show <lbl_prefix> [message-id] [flags]",
		Parameters: []clihelp.Param{
			{Name: "<lbl_prefix>", Description: "Prefix of folder/label to inspect."},
			{Name: "[message-id]", Description: "Optional message ID or short prefix to show a single message."},
		},
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagShowWeb, "-w, --web", false, "Open the HTML body of the email in your configured browser"),
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli show receipts"},
			{Line: "mail_cli show receipts 1234abcd"},
			{Line: "mail_cli show receipts 1234abcd --web"},
		},
		Args: clihelp.RangeArgs(1, 2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			labelPrefix := args[0]
			targetMsgID := ""
			if len(args) > 1 {
				targetMsgID = args[1]
			}
			if session.RunShow == nil {
				return ErrConfigNotConfigured("show")
			}
			return session.RunShow(session.Config, labelPrefix, targetMsgID)
		},
	}
}

// TestCmd returns the clihelp.Command for the test command.
func TestCmd(session *app.Session) clihelp.Command {
	runTest := func(ctx *clihelp.Context) error {
		if session.RunTests == nil {
			return ErrConfigNotConfigured("testing")
		}
		return session.RunTests(session.Config)
	}

	return clihelp.Command{
		Name:        "test",
		Description: "Run system and integration self-tests to verify API credentials and mail flow.",
		UsageLine:   "mail_cli test run",
		Run:         runTest,
		Subcommands: []clihelp.Command{
			{
				Name:        "run",
				Description: "Execute connection and integration tests.",
				UsageLine:   "mail_cli test run",
				Args:        clihelp.NoArgs,
				Run:         runTest,
				Examples: []clihelp.Example{
					{Line: "mail_cli test run"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli test run"},
		},
	}
}

// UnspamCmd returns the clihelp.Command for the unspam command.
func UnspamCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "unspam",
		Description: "Mark a message as not being spam: train bogofilter as ham and move it from Spam back to Inbox on the server.",
		UsageLine:   "mail_cli unspam <message_id...>\n  mail_cli unspam folder <folder_name>",
		Parameters: []clihelp.Param{
			{Name: "<message_id...>", Description: "One or more message IDs to unspam (short 8-char or full)."},
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "folder",
				Title:       "unspam folder",
				Description: "Mark all messages in the specified folder as ham and move them back to Inbox.",
				UsageLine:   "mail_cli unspam folder <folder_name>",
				Parameters: []clihelp.Param{
					{Name: "<folder_name>", Description: "Folder name containing emails to unspam."},
				},
				Args: clihelp.ExactArgs(1),
				Run: func(ctx *clihelp.Context) error {
					if session.RunUnspamFolder == nil {
						return ErrConfigNotConfigured("unspam folder")
					}
					return session.RunUnspamFolder(session.Config, ctx.Args[0])
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli unspam folder Spam"},
				},
			},
		},
		Args: clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			if len(args) == 1 {
				if session.GetClient == nil {
					return ErrClientNotConfigured()
				}
				client, err := session.GetClient(session.Config)
				if err != nil {
					return err
				}
				if err := client.Validate(); err != nil {
					return err
				}
				matches, err := client.GetMatchingLabels(args[0])
				if err == nil && len(matches) > 0 {
					for _, m := range matches {
						if strings.EqualFold(m, args[0]) {
							if session.RunUnspamFolder == nil {
								return ErrConfigNotConfigured("unspam folder")
							}
							return session.RunUnspamFolder(session.Config, m)
						}
					}
				}
			}

			if session.RunUnspam == nil {
				return ErrConfigNotConfigured("unspam")
			}
			for _, arg := range args {
				if err := session.RunUnspam(session.Config, arg); err != nil {
					return err
				}
			}
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli unspam abc123de"},
			{Line: "mail_cli unspam folder Spam"},
		},
	}
}
