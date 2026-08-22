package cli

import (
	"fmt"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// SpamCmd returns the clihelp.Command for the spam command.
func SpamCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "spam",
		Description: "Manage Spam folder, train filters, and unsubscribe from political mail.",
		UsageLine:   "mail_cli spam <subcommand>\nmail_cli spam <message_id...>",
		Parameters: []clihelp.Param{
			{Name: "<message_id...>", Description: "One or more message IDs to mark as spam."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli spam del"},
			{Line: "mail_cli spam bye"},
			{Line: "mail_cli spam learn"},
			{Line: "mail_cli spam pol audit"},
			{Line: "mail_cli spam pol unsub"},
			{Line: "mail_cli spam abc123de"},
		},
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			if len(args) > 0 {
				if session.MarkSpam == nil {
					return fmt.Errorf("mark spam not configured")
				}
				for _, id := range args {
					if err := session.MarkSpam(session.Config, id); err != nil {
						return err
					}
				}
				return nil
			}
			ctx.App.RenderCommand(clihelp.Options{Writer: ctx.Stdout}, "spam")
			return nil
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "del",
				Aliases:     []string{"delete"},
				Title:       "spam del",
				Description: "Permanently purge all emails in the Spam folder.",
				UsageLine:   "mail_cli spam del",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.Config.ReadOnly {
						return fmt.Errorf("Error: Cannot permanently delete messages while in read-only mode.")
					}
					client, err := GetValidatedClient(session)
					if err != nil {
						return err
					}
					return client.DeleteAllSpam()
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli spam del"},
				},
			},
			{
				Name:        "bye",
				Title:       "spam bye",
				Description: "Execute a complete sweep: unsubscribe from political lists, train spam filters on all messages in Spam, and purge the Spam folder.",
				UsageLine:   "mail_cli spam bye",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.GetClient == nil {
						return fmt.Errorf("client not configured")
					}
					client, err := session.GetClient(session.Config)
					if err != nil {
						return err
					}
					if err := client.Validate(); err != nil {
						return err
					}
					destLabel := session.Config.SpamLearn
					if destLabel == "" {
						destLabel = "LearnSpam"
					}
					if session.Config.ReadOnly {
						fmt.Printf("[DRY-RUN] Would mark spam and move to %s\n", destLabel)
						return nil
					}
					if err := client.ShowPoliticalSpam(true); err != nil {
						return err
					}
					if err := client.LearnSpam(); err != nil {
						return err
					}
					return client.MoveAllSpam(destLabel)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli spam bye"},
				},
			},
			{
				Name:        "learn",
				Title:       "spam learn",
				Description: "Train Bogofilter on all messages currently in the Spam folder, then move them to the LearnSpam folder.",
				UsageLine:   "mail_cli spam learn [dest_label]",
				Parameters: []clihelp.Param{
					{Name: "[dest_label]", Description: "Optional destination label (defaults to LearnSpam or configured SpamLearn folder)."},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					if session.GetClient == nil {
						return fmt.Errorf("client not configured")
					}
					client, err := session.GetClient(session.Config)
					if err != nil {
						return err
					}
					if err := client.Validate(); err != nil {
						return err
					}
					destLabel := session.Config.SpamLearn
					if destLabel == "" {
						destLabel = "LearnSpam"
					}
					return client.MoveAllSpam(destLabel)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli spam learn"},
				},
			},
			{
				Name:        "pol",
				Title:       "spam pol",
				Description: "Manage political spam processing and unsubscribing.",
				UsageLine:   "mail_cli spam pol <subcommand>",
				Subcommands: []clihelp.Command{
					{
						Name:        "audit",
						Description: "Scan Spam folder for political fundraising emails and print scoring details.",
						UsageLine:   "mail_cli spam pol audit",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							if session.GetClient == nil {
								return fmt.Errorf("client not configured")
							}
							client, err := session.GetClient(session.Config)
							if err != nil {
								return err
							}
							if err := client.Validate(); err != nil {
								return err
							}
							return client.ShowPoliticalSpam(false)
						},
						Examples: []clihelp.Example{
							{Line: "mail_cli spam pol audit"},
						},
					},
					{
						Name:        "unsub",
						Description: "Scan the Spam folder for political messages and execute unsubscription opt-outs.",
						UsageLine:   "mail_cli spam pol unsub",
						Args:        clihelp.NoArgs,
						Run: func(ctx *clihelp.Context) error {
							if session.GetClient == nil {
								return fmt.Errorf("client not configured")
							}
							client, err := session.GetClient(session.Config)
							if err != nil {
								return err
							}
							if err := client.Validate(); err != nil {
								return err
							}
							destLabel := session.Config.SpamLearn
							if destLabel == "" {
								destLabel = "LearnSpam"
							}
							return client.MoveAllSpam(destLabel)
						},
						Examples: []clihelp.Example{
							{Line: "mail_cli spam pol unsub"},
						},
					},
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli spam pol audit"},
					{Line: "mail_cli spam pol unsub"},
				},
			},
		},
	}
}

// LearnHamCmd returns the clihelp.Command for the learn-ham command.
func LearnHamCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "learn-ham",
		Aliases:     []string{"learn_ham"},
		Description: "Train Bogofilter on ham (non-spam) emails in a folder. The folder must be an exact match and cannot have subfolders.",
		UsageLine:   "mail_cli learn-ham <label> [flags]",
		Parameters: []clihelp.Param{
			{Name: "<label>", Description: "The exact folder/label name to learn as ham."},
		},
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagForceLearnHam, "--force", false, "Bypass trained message database and re-train all emails"),
			{Flags: "--batch", Description: "Process messages in larger batches for speed."},
			{Flags: "--rescan", Description: "Rescan all messages in folder regardless of cache status."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli learn-ham INBOX"},
			{Line: "mail_cli learn-ham INBOX --force"},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			if session.LearnHam == nil {
				return fmt.Errorf("learn ham not configured")
			}
			return session.LearnHam(session.Config, ctx.Args[0], app.FlagForceLearnHam)
		},
	}
}

// LearningCmd returns the clihelp.Command for the learning command.
func LearningCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "learning",
		Description: "Manage local spam learning and training.",
		UsageLine:   "mail_cli learning <subcommand>",
		Examples: []clihelp.Example{
			{Line: "mail_cli learning reset"},
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "reset",
				Title:       "learning reset",
				Description: "Reset all local spam classifications and scores in the cache for the current account.",
				UsageLine:   "mail_cli learning reset",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.RunLearningReset == nil {
						return fmt.Errorf("learning reset not configured")
					}
					return session.RunLearningReset(session.Config)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli learning reset"},
				},
			},
		},
	}
}
