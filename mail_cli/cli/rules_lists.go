package cli

import (
	"fmt"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"

	"github.com/sarielhp/clihelp"
)

// WlistCmd returns the clihelp.Command for the whitelist command.
func WlistCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "whitelist",
		Aliases:     []string{"wlist"},
		Description: "Manage the personal sender whitelist to bypass spam checks.",
		UsageLine:   "mail_cli whitelist <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "add",
				Title:       "whitelist add <email...>",
				Description: "Add one or more email addresses to the whitelist.",
				UsageLine:   "mail_cli whitelist add <email...>",
				Parameters: []clihelp.Param{
					{Name: "<email...>", Description: "Email addresses to add to whitelist."},
				},
				Args: clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "whitelistadd", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli whitelist add friend@example.com"},
				},
			},
			{
				Name:        "del",
				Aliases:     []string{"delete"},
				Title:       "whitelist del <email...>",
				Description: "Remove one or more email addresses from the whitelist.",
				UsageLine:   "mail_cli whitelist del <email...>",
				Parameters: []clihelp.Param{
					{Name: "<email...>", Description: "Email addresses to remove from whitelist."},
				},
				Args: clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "whitelistdel", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli whitelist del friend@example.com"},
				},
			},
			{
				Name:        "list",
				Description: "List all whitelisted email addresses.",
				UsageLine:   "mail_cli whitelist list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "whitelistlist", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli whitelist list"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli whitelist list"},
			{Line: "mail_cli whitelist add friend@example.com"},
		},
	}
}

// BlistCmd returns the clihelp.Command for the blacklist command.
func BlistCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "blacklist",
		Aliases:     []string{"blist"},
		Description: "Manage the personal sender blacklist to instantly classify messages as spam.",
		UsageLine:   "mail_cli blacklist <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "add",
				Title:       "blacklist add <email...>",
				Description: "Add one or more email addresses to the blacklist.",
				UsageLine:   "mail_cli blacklist add <email...>",
				Parameters: []clihelp.Param{
					{Name: "<email...>", Description: "Email addresses to add to blacklist."},
				},
				Args: clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "blacklistadd", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli blacklist add spammer@example.com"},
				},
			},
			{
				Name:        "del",
				Aliases:     []string{"delete"},
				Title:       "blacklist del <email...>",
				Description: "Remove one or more email addresses from the blacklist.",
				UsageLine:   "mail_cli blacklist del <email...>",
				Parameters: []clihelp.Param{
					{Name: "<email...>", Description: "Email addresses to remove from blacklist."},
				},
				Args: clihelp.MinimumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "blacklistdel", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli blacklist del spammer@example.com"},
				},
			},
			{
				Name:        "list",
				Description: "List all blacklisted email addresses.",
				UsageLine:   "mail_cli blacklist list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return whitelistOrBlacklist(session, "blacklistlist", ctx.Args)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli blacklist list"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli blacklist list"},
			{Line: "mail_cli blacklist add spammer@example.com"},
		},
	}
}

// FilterCmd returns the clihelp.Command for the filter command.
func FilterCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "filter",
		Description: "Manage remote Gmail server-side filters and forwarding rules.",
		UsageLine:   "mail_cli filter <subcommand>",
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Description: "List all remote Gmail server-side filters for the selected account.",
				UsageLine:   "mail_cli filter list",
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
					return client.ListFilters()
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli filter list"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli filter list"},
		},
	}
}

func whitelistOrBlacklist(session *app.Session, listType string, args []string) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	field := "whitelist"
	if listType == "blacklistadd" || listType == "blacklistdel" || listType == "blacklistlist" {
		field = "blacklist"
	}

	switch listType {
	case "whitelistadd":
		for _, emailAddr := range args {
			AddToList(targetAcc, field, emailAddr)
		}
	case "blacklistadd":
		for _, emailAddr := range args {
			AddToList(targetAcc, field, emailAddr)
			ensureRuleForBlacklist(configPath, fc, targetAcc, emailAddr)
		}
	case "whitelistdel":
		for _, emailAddr := range args {
			RemoveFromList(targetAcc, field, emailAddr)
		}
	case "blacklistdel":
		for _, emailAddr := range args {
			RemoveFromList(targetAcc, field, emailAddr)
			removeRuleForBlacklist(configPath, fc, targetAcc, emailAddr)
		}
	case "whitelistlist", "blacklistlist":
		ListEntries(targetAcc, field)
		return nil
	}

	return cfg_g.SaveConfigFile(configPath, fc)
}

func ensureRuleForBlacklist(configPath string, fc *cfg_g.FileConfig, targetAcc *cfg_acc.AccountConfig, emailAddr string) {
	if targetAcc.SpamLearn == "" {
		fmt.Printf("%s Warning: no SpamLearn folder configured for account %s; skipping rule creation.\n", app.PrefixWarn, targetAcc.Name)
		return
	}
	for _, r := range targetAcc.Rules {
		if strings.EqualFold(r.Sender, emailAddr) {
			return
		}
	}
	targetAcc.Rules = append(targetAcc.Rules, cfg_acc.Rule{Sender: emailAddr, Label: targetAcc.SpamLearn})
	fmt.Printf("%s Created rule: %q -> %q for account %s.\n", app.PrefixSuccess, emailAddr, targetAcc.SpamLearn, targetAcc.Name)
}

func removeRuleForBlacklist(configPath string, fc *cfg_g.FileConfig, targetAcc *cfg_acc.AccountConfig, emailAddr string) {
	for i, r := range targetAcc.Rules {
		if strings.EqualFold(r.Sender, emailAddr) {
			targetAcc.Rules = append(targetAcc.Rules[:i], targetAcc.Rules[i+1:]...)
			fmt.Printf("%s Removed rule for %q on account %s.\n", app.PrefixSuccess, emailAddr, targetAcc.Name)
			return
		}
	}
}
