package cli

import (
	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// ArcCmd returns the clihelp.Command for the archive command.
func ArcCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "archive",
		Aliases:     []string{"arc"},
		Description: "Move message(s) by ID from their current folder to the Archive or Received folder. Or archive all messages in Inbox (default) or the specified label (by prefix).",
		UsageLine:   "mail_cli archive <all [label] | message-id...>",
		Parameters: []clihelp.Param{
			{Name: "all [label]", Description: "Archive all emails in the Inbox or specified label prefix."},
			{Name: "<message-id...>", Description: "One or more message IDs to archive (short 8-char or full)."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli archive abc123de"},
			{Line: "mail_cli archive all"},
			{Line: "mail_cli archive all receipts"},
		},
		Args: clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			defaultClient, err := GetValidatedClient(session)
			if err != nil {
				return err
			}

			if args[0] == "all" {
				if len(args) > 2 {
					return ErrInvalidArgs("arc all", "arc all [label]")
				}
				client := defaultClient
				sourcePrefix := client.InboxFolder()
				if len(args) > 1 {
					c, sPrefix, err := session.ResolveClientAndLabel(args[1])
					if err != nil {
						return err
					}
					client = c
					sourcePrefix = sPrefix
				}
				if err := client.Validate(); err != nil {
					return err
				}
				targetFolder, err := session.ResolveArchiveTarget(client)
				if err != nil {
					return err
				}
				if session.ArchiveAll == nil {
					return ErrConfigNotConfigured("archive all")
				}
				return session.ArchiveAll(session.Config, client, sourcePrefix, targetFolder)
			}

			if err := defaultClient.Validate(); err != nil {
				return err
			}
			targetFolder, err := session.ResolveArchiveTarget(defaultClient)
			if err != nil {
				return err
			}

			for _, arg := range args {
				if session.ArchiveByID == nil {
					return ErrConfigNotConfigured("archive by ID")
				}
				if err := session.ArchiveByID(session.Config, defaultClient, targetFolder, arg); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
