package cli

import (
	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// ConfigCmd returns the clihelp.Command for the config command.
func ConfigCmd(session *app.Session) clihelp.Command {
	showRun := func(ctx *clihelp.Context) error {
		if session.ConfigShow == nil {
			return ErrConfigNotConfigured("config show")
		}
		return session.ConfigShow(session.Config)
	}

	return clihelp.Command{
		Name:        "config",
		Description: "Show or manage configuration options.",
		UsageLine:   "mail_cli config [subcommand]",
		Run:         showRun,
		Subcommands: []clihelp.Command{
			{
				Name:        "show",
				Title:       "config show",
				Description: "Display the active configuration values.",
				UsageLine:   "mail_cli config show",
				Args:        clihelp.NoArgs,
				Run:         showRun,
				Examples: []clihelp.Example{
					{Line: "mail_cli config show"},
					{Line: "mail_cli config"},
				},
			},
			{
				Name:        "set",
				Title:       "config set <key> <value>",
				Description: "Set a configuration parameter to a new value (supports account-specific overrides).",
				UsageLine:   "mail_cli config set <key> <value>",
				Parameters: []clihelp.Param{
					{Name: "<key>", Description: "The configuration parameter name."},
					{Name: "<value>", Description: "The value to set for the parameter."},
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli config set spam_folder Junk"},
					{Line: "mail_cli config set limit 50"},
				},
				Args: clihelp.RangeArgs(2, 3),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					key := args[0]
					value := args[1]
					accountSpecific := len(args) > 2
					if session.ConfigSet == nil {
						return ErrConfigNotConfigured("config set")
					}
					return session.ConfigSet(session.Config, key, value, accountSpecific)
				},
			},
			{
				Name:        "reset",
				Title:       "config reset <key>",
				Description: "Reset a configuration parameter to its default value.",
				UsageLine:   "mail_cli config reset <key>",
				Parameters: []clihelp.Param{
					{Name: "<key>", Description: "The configuration parameter name to reset."},
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli config reset score_threshold"},
				},
				Args: clihelp.ExactArgs(1),
				Run: func(ctx *clihelp.Context) error {
					key := ctx.Args[0]
					if session.ConfigReset == nil {
						return ErrConfigNotConfigured("config reset")
					}
					return session.ConfigReset(session.Config, key)
				},
			},
			{
				Name:        "validate",
				Title:       "config validate",
				Description: "Validate configurations, account parameters, DNS reachability, and Bogofilter service.",
				UsageLine:   "mail_cli config validate",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					if session.ConfigValidate == nil {
						return ErrConfigNotConfigured("config validate")
					}
					return session.ConfigValidate(session.Config)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli config validate"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli config show"},
			{Line: "mail_cli config set spam_folder Junk"},
			{Line: "mail_cli config reset score_threshold"},
			{Line: "mail_cli config validate"},
		},
	}
}
