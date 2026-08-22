package cli

import (
	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// CalendarCmd returns the clihelp.Command for the calendar command.
func CalendarCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "calendar",
		Description: "Manage calendar events extracted from email attachments.",
		UsageLine:   "mail_cli calendar <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "add",
				Title:       "calendar add [label_prefix] <message_id>",
				Description: "Add a calendar event from an .ics attachment in a specific email.",
				UsageLine:   "mail_cli calendar add [label_prefix] <message_id>",
				Parameters: []clihelp.Param{
					{Name: "[label_prefix]", Description: "Label prefix to locate the message (defaults to inbox)."},
					{Name: "<message_id>", Description: "Message ID or prefix containing the .ics attachment."},
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli calendar add 1234abcd"},
					{Line: "mail_cli calendar add Receipts 5678efgh"},
				},
				Args: clihelp.RangeArgs(1, 2),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					if session.GetClient == nil {
						return ErrClientNotConfigured()
					}
					client, err := session.GetClient(session.Config)
					if err != nil {
						return err
					}
					labelPrefix := client.InboxFolder()
					msgID := args[0]
					if len(args) > 1 {
						labelPrefix = args[0]
						msgID = args[1]
					}
					if session.CalendarAdd == nil {
						return ErrConfigNotConfigured("calendar add")
					}
					return session.CalendarAdd(session.Config, client, labelPrefix, msgID)
				},
			},
			{
				Name:        "week",
				Title:       "calendar week",
				Description: "Show all calendar events in the upcoming week.",
				UsageLine:   "mail_cli calendar week",
				Examples: []clihelp.Example{
					{Line: "mail_cli calendar week"},
				},
				Args: clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.CalendarWeek == nil {
						return ErrConfigNotConfigured("calendar week")
					}
					return session.CalendarWeek(session.Config)
				},
			},
			{
				Name:        "add-all",
				Aliases:     []string{"addall"},
				Title:       "calendar add-all",
				Description: "Scan inbox for .ics attachments and add them to calendar.",
				UsageLine:   "mail_cli calendar add-all",
				Examples: []clihelp.Example{
					{Line: "mail_cli calendar add-all"},
				},
				Args: clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.CalAddAll == nil {
						return ErrConfigNotConfigured("caladd")
					}
					if session.GetClient == nil {
						return ErrClientNotConfigured()
					}
					client, err := session.GetClient(session.Config)
					if err != nil {
						return err
					}
					return session.CalAddAll(session.Config, client)
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli calendar week"},
			{Line: "mail_cli calendar add 1234abcd"},
			{Line: "mail_cli calendar add-all"},
		},
	}
}

// CalAddCmd returns the clihelp.Command for the caladd shortcut command.
func CalAddCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "caladd",
		Aliases:     []string{"add-all"},
		Description: "Scan the inbox for messages containing .ics attachments, and add them to the calendar if they are not already present.",
		UsageLine:   "mail_cli caladd",
		Examples: []clihelp.Example{
			{Line: "mail_cli caladd"},
		},
		Args: clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			if session.CalAddAll == nil {
				return ErrConfigNotConfigured("caladd")
			}
			if session.GetClient == nil {
				return ErrClientNotConfigured()
			}
			client, err := session.GetClient(session.Config)
			if err != nil {
				return err
			}
			return session.CalAddAll(session.Config, client)
		},
	}
}
