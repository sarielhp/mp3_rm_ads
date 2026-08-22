package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"

	"github.com/sarielhp/clihelp"
)

// RuleCmd returns the clihelp.Command for the rule command.
func RuleCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "rule",
		Description: "Manage custom routing rules and auto-labeling filters.",
		UsageLine:   "mail_cli rule <subcommand> [args...]",
		Options: []clihelp.Option{
			clihelp.String(&app.FlagRuleExportFile, "--export <file>", "", "Export all existing rules to a file"),
			clihelp.String(&app.FlagRuleImportFile, "-import <file>", "", "Import rules from a file"),
		},
		Subcommands: []clihelp.Command{
			{
				Name:        "add",
				Title:       "rule add <email> <lbl>",
				Description: "Add a server filter rule to auto-label emails from a sender.",
				UsageLine:   "mail_cli rule add <email> <lbl>",
				Parameters: []clihelp.Param{
					{Name: "<email>", Description: "The sender email address to match."},
					{Name: "<lbl>", Description: "The target label/folder hierarchy."},
				},
				Args: clihelp.ExactArgs(2),
				Run: func(ctx *clihelp.Context) error {
					return addRuleToConfig(session, ctx.Args[0], ctx.Args[1], "")
				},
				Examples: []clihelp.Example{
					{Line: `mail_cli rule add notifications@github.com "Dev/GitHub"`},
				},
			},
			{
				Name:        "add-by-title",
				Aliases:     []string{"add_by_title"},
				Title:       "rule add_by_title <title> <lbl>",
				Description: "Add a server filter rule to auto-label emails with matching subject prefix.",
				UsageLine:   "mail_cli rule add_by_title <title> <lbl>",
				Parameters: []clihelp.Param{
					{Name: "<title>", Description: "Subject prefix/substring to match."},
					{Name: "<lbl>", Description: "The target label/folder hierarchy."},
				},
				Args: clihelp.ExactArgs(2),
				Run: func(ctx *clihelp.Context) error {
					return addRuleToConfig(session, "", ctx.Args[1], ctx.Args[0])
				},
				Examples: []clihelp.Example{
					{Line: `mail_cli rule add_by_title "[JIRA]" "Work/Jira"`},
				},
			},
			ruleAddDomainCmd(session),
			{
				Name:        "del",
				Aliases:     []string{"delete", "rm", "remove"},
				Title:       "rule del <email|title>",
				Description: "Remove an auto-labeling rule by sender email address or subject title.",
				UsageLine:   "mail_cli rule del <email|title>",
				Parameters: []clihelp.Param{
					{Name: "<email|title>", Description: "The sender email or subject title to remove rule for."},
				},
				Args: clihelp.ExactArgs(1),
				Run: func(ctx *clihelp.Context) error {
					return deleteRuleFromConfig(session, ctx.Args[0], os.Stdin)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli rule del notifications@github.com"},
				},
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Title:       "rule list",
				Description: "List custom routing rules with their target labels and export status.",
				UsageLine:   "mail_cli rule list [flags]",
				Options: []clihelp.Option{
					clihelp.Bool(&app.FlagRuleListAll, "-a, --all", false, "List all custom routing rules, including those already exported"),
				},
				Args: clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return listRules(session)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli rule list"},
					{Line: "mail_cli rule list --all"},
				},
			},
			{
				Name:        "delete-all",
				Aliases:     []string{"del-all", "del_all", "delete_all"},
				Title:       "rule delete-all",
				Description: "Delete all routing rules for the selected account.",
				UsageLine:   "mail_cli rule delete-all",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return deleteAllRules(session)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli rule delete-all"},
				},
			},
			{
				Name:        "update",
				Title:       "rule update",
				Description: "Ensure all blacklisted senders have a corresponding local rule.",
				UsageLine:   "mail_cli rule update",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					return syncBlacklistToRules(session)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli rule update"},
				},
			},
			{
				Name:        "export",
				Title:       "rule export",
				Description: "Export local rules to mail server filters (e.g. Gmail filters or Sieve script).",
				UsageLine:   "mail_cli rule export [force] [flags]",
				Parameters: []clihelp.Param{
					{Name: "[force]", Description: "Optional force flag to overwrite existing remote filters."},
				},
				Options: []clihelp.Option{
					clihelp.String(&app.FlagSieveExport, "--sieve <file>", "", "Export rules as a Sieve script to file path"),
					clihelp.Bool(&app.FlagForceRuleExport, "-f, --force", false, "Force overwrite conflicting remote filters"),
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					if app.FlagForceRuleExport || (len(args) > 0 && strings.EqualFold(args[0], "force")) {
						session.Config.ForceLearn = true
					}
					if app.FlagSieveExport != "" {
						if session.ExportSieve == nil {
							return fmt.Errorf("sieve export not configured")
						}
						return session.ExportSieve(session.Config, app.FlagSieveExport)
					}
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
					return client.ExportRules()
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli rule export"},
					{Line: "mail_cli rule export --sieve ~/rules.sieve"},
				},
			},
		},
		Run: func(ctx *clihelp.Context) error {
			if app.FlagRuleExportFile != "" {
				if session.ExportRules == nil {
					return fmt.Errorf("export rules not configured")
				}
				return session.ExportRules(session.Config, app.FlagRuleExportFile)
			}
			if app.FlagRuleImportFile != "" {
				if session.ImportRules == nil {
					return fmt.Errorf("import rules not configured")
				}
				return session.ImportRules(session.Config, app.FlagRuleImportFile)
			}
			ctx.App.RenderCommand(clihelp.Options{Writer: ctx.Stdout}, "rule")
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli rule list"},
			{Line: `mail_cli rule add notifications@github.com "Dev/GitHub"`},
			{Line: `mail_cli rule add_by_title "[ALERT]" "Alerts"`},
			{Line: "mail_cli rule export"},
		},
	}
}

func addRuleToConfig(session *app.Session, emailAddr, label, title string) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	resolvedLabel := label
	if session.GetClient != nil {
		if client, err := session.GetClient(session.Config); err == nil {
			if rl, err := mailclient.ResolveLabel(client, label); err == nil {
				resolvedLabel = rl
			}
		}
	}

	if emailAddr != "" {
		for i, rule := range targetAcc.Rules {
			if strings.EqualFold(rule.Sender, emailAddr) {
				oldLabel := rule.Label
				targetAcc.Rules[i].Label = resolvedLabel
				targetAcc.Rules[i].Exported = false
				if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
					return err
				}
				fmt.Printf("%s Updated existing rule for %s: %s -> %s (Account: %s)\n", app.PrefixSuccess, emailAddr, oldLabel, resolvedLabel, targetAcc.Name)
				return nil
			}
		}
		targetAcc.Rules = append(targetAcc.Rules, cfg_acc.Rule{
			Sender: emailAddr,
			Label:  resolvedLabel,
		})
		if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
			return err
		}
		fmt.Printf("%s Added rule for %s -> %s (Account: %s)\n", app.PrefixSuccess, emailAddr, resolvedLabel, targetAcc.Name)
		return nil
	}

	for i, rule := range targetAcc.Rules {
		if strings.EqualFold(rule.Subject, title) {
			oldLabel := rule.Label
			targetAcc.Rules[i].Label = resolvedLabel
			targetAcc.Rules[i].Exported = false
			if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
				return err
			}
			fmt.Printf("%s Updated existing rule for title %s: %s -> %s (Account: %s)\n", app.PrefixSuccess, title, oldLabel, resolvedLabel, targetAcc.Name)
			return nil
		}
	}
	targetAcc.Rules = append(targetAcc.Rules, cfg_acc.Rule{
		Subject: title,
		Label:   resolvedLabel,
	})
	if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
		return err
	}
	fmt.Printf("%s Added rule for title %s -> %s (Account: %s)\n", app.PrefixSuccess, title, resolvedLabel, targetAcc.Name)
	return nil
}

func deleteRuleFromConfig(session *app.Session, target string, stdin io.Reader) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	var targetRuleIndex = -1
	var targetRule cfg_acc.Rule
	isIndexMatch := false

	// First, try matching as a 1-based index (rule number)
	if num, err := strconv.Atoi(target); err == nil && num > 0 {
		idx := num - 1
		if idx < len(targetAcc.Rules) {
			targetRuleIndex = idx
			targetRule = targetAcc.Rules[idx]
			isIndexMatch = true
		}
	}

	// If not matched as a valid index, search by sender or subject
	if targetRuleIndex == -1 {
		targetClean := strings.TrimSpace(target)
		targetAddr := strings.ToLower(email.ParseEmailAddress(targetClean))

		for i, r := range targetAcc.Rules {
			senderClean := strings.TrimSpace(r.Sender)
			senderAddr := strings.ToLower(email.ParseEmailAddress(senderClean))

			if strings.EqualFold(senderClean, targetClean) ||
				strings.EqualFold(r.Subject, targetClean) ||
				(targetAddr != "" && (strings.EqualFold(senderAddr, targetAddr) || strings.EqualFold(senderClean, targetAddr))) {
				targetRuleIndex = i
				targetRule = r
				break
			}
		}
	}

	if targetRuleIndex == -1 {
		fmt.Printf("%s Rule %s not found.\n", app.PrefixWarn, target)
		return nil
	}

	// Show rule details
	ident := targetRule.Sender
	if targetRule.Subject != "" {
		ident = "[Subject: " + targetRule.Subject + "]"
	}
	status := "pending"
	if targetRule.Exported {
		status = "exported"
	}
	if targetRule.Internal {
		status = "internal"
	}

	if isIndexMatch {
		fmt.Printf("Rule %d: %s -> %s [%s]\n", targetRuleIndex+1, ident, targetRule.Label, status)

		// Ask for confirmation Y/N from the user
		fmt.Print("Delete this rule? (y/N): ")
		reader := bufio.NewReader(stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete this rule
	targetAcc.Rules = append(targetAcc.Rules[:targetRuleIndex], targetAcc.Rules[targetRuleIndex+1:]...)
	fmt.Printf("%s Deleted rule for %s on account %s.\n", app.PrefixSuccess, ident, targetAcc.Name)
	return cfg_g.SaveConfigFile(configPath, fc)
}

func listRules(session *app.Session) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	if len(targetAcc.Rules) > 0 {
		seen := make(map[string]int)
		for i, r := range targetAcc.Rules {
			key := ""
			if r.Sender != "" {
				key = "sender:" + strings.ToLower(r.Sender)
			} else if r.Subject != "" {
				key = "subject:" + strings.ToLower(r.Subject)
			}
			if key != "" {
				seen[key] = i
			}
		}

		var keptRules []cfg_acc.Rule
		changed := false
		for i, r := range targetAcc.Rules {
			key := ""
			if r.Sender != "" {
				key = "sender:" + strings.ToLower(r.Sender)
			} else if r.Subject != "" {
				key = "subject:" + strings.ToLower(r.Subject)
			}

			if key == "" {
				keptRules = append(keptRules, r)
				continue
			}

			lastIndex := seen[key]
			if i == lastIndex {
				keptRules = append(keptRules, r)
			} else {
				ident := r.Sender
				if r.Subject != "" {
					ident = "[Subject: " + r.Subject + "]"
				}
				fmt.Printf("%s Found duplicate rule for '%s' pointing to %q. Deleting duplicate (keeping last rule pointing to %q).\n",
					app.PrefixInfo, ident, r.Label, targetAcc.Rules[lastIndex].Label)
				changed = true
			}
		}

		if changed {
			targetAcc.Rules = keptRules
			if saveErr := cfg_g.SaveConfigFile(configPath, fc); saveErr != nil {
				return fmt.Errorf("failed to save config during rule deduplication: %w", saveErr)
			}
		}
	}

	if len(targetAcc.Rules) == 0 {
		fmt.Printf("No rules found for account %s.\n", targetAcc.Name)
		return nil
	}

	fmt.Printf("Rules for account %s:\n", targetAcc.Name)
	for i, r := range targetAcc.Rules {
		if !app.FlagRuleListAll && r.Exported {
			continue
		}
		status := "pending"
		if r.Exported {
			status = "exported"
		}
		if r.Internal {
			status = "internal"
		}
		ident := r.Sender
		if r.Subject != "" {
			ident = "[Subject: " + r.Subject + "]"
		}
		fmt.Printf("  %d. %s -> %s [%s]\n", i+1, ident, r.Label, status)
	}
	return nil
}

func deleteAllRules(session *app.Session) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	targetAcc.Rules = nil
	fmt.Printf("%s Deleted all rules for account %s.\n", app.PrefixSuccess, targetAcc.Name)
	return cfg_g.SaveConfigFile(configPath, fc)
}

func syncBlacklistToRules(session *app.Session) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	if len(targetAcc.Blacklist) == 0 {
		fmt.Printf("No blacklisted senders for account %s.\n", targetAcc.Name)
		return nil
	}

	if targetAcc.SpamLearn == "" {
		return fmt.Errorf("no SpamLearn folder configured for account %s", targetAcc.Name)
	}

	added := 0
	for _, emailAddr := range targetAcc.Blacklist {
		found := false
		for _, r := range targetAcc.Rules {
			if strings.EqualFold(r.Sender, emailAddr) {
				found = true
				break
			}
		}
		if !found {
			targetAcc.Rules = append(targetAcc.Rules, cfg_acc.Rule{Sender: emailAddr, Label: targetAcc.SpamLearn})
			fmt.Printf("  + %s -> %s\n", emailAddr, targetAcc.SpamLearn)
			added++
		}
	}

	if added > 0 {
		fmt.Printf("%s Added %d rule(s) for blacklisted sender(s) on account %s.\n", app.PrefixSuccess, added, targetAcc.Name)
	} else {
		fmt.Printf("All blacklisted senders already have rules for account %s.\n", targetAcc.Name)
	}
	return cfg_g.SaveConfigFile(configPath, fc)
}
