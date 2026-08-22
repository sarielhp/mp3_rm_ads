package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"

	"github.com/sarielhp/clihelp"
)

// AccountsCmd returns the clihelp.Command for the account command.
func AccountsCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "account",
		Aliases:     []string{"accounts"},
		Description: "Manage and list configured mail accounts.",
		UsageLine:   "mail_cli account <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "list",
				Description: "List all configured accounts and their properties.",
				UsageLine:   "mail_cli account list",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					if len(*fc.Accounts) == 0 {
						fmt.Println("No accounts configured.")
						return nil
					}
					fmt.Printf("Configured accounts (%d):\n", len(*fc.Accounts))
					for i, acc := range *fc.Accounts {
						selected := ""
						if acc.Name == session.Config.SelectedAccount {
							selected = " *"
						}
						accType := acc.AccountType
						if accType == "" {
							accType = "regular"
						}
						displayName := acc.GetDisplayName()
						if displayName != acc.Name {
							fmt.Printf("%d. %s [%s] (%s, type: %s)%s\n", i+1, displayName, acc.Name, acc.Type, accType, selected)
						} else {
							fmt.Printf("%d. %s (%s, type: %s)%s\n", i+1, acc.Name, acc.Type, accType, selected)
						}
					}
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account list"},
				},
			},
			accountNewCmd(session),
			{
				Name:        "associate",
				Aliases:     []string{"assoc"},
				Title:       "account associate <name> <program>",
				Description: "Associate a program/symlink name with an account for automatic account selection.",
				UsageLine:   "mail_cli account associate <account_name> <program_name>",
				Parameters: []clihelp.Param{
					{Name: "<account_name>", Description: "The existing account name to associate with."},
					{Name: "<program_name>", Description: "The command name that will trigger this account (e.g. \"work_mail\")."},
				},
				Args: clihelp.ExactArgs(2),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					accountName := args[0]
					progName := args[1]

					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					for i := range *fc.Accounts {
						if strings.EqualFold((*fc.Accounts)[i].Name, accountName) {
							acc := &(*fc.Accounts)[i]
							if acc.Aliases == nil {
								acc.Aliases = []string{}
							}
							acc.Aliases = append(acc.Aliases, progName)
							if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
								return err
							}
							fmt.Printf("%s Successfully associated program name %q with account %q.\n", app.PrefixSuccess, progName, accountName)
							return nil
						}
					}
					return ErrAccountNotFound(accountName)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account associate work-account work_mail"},
				},
			},
			{
				Name:        "test",
				Title:       "account test [account_name]",
				Description: "Test validation and server connection for an account.",
				UsageLine:   "mail_cli account test [account_name]",
				Parameters: []clihelp.Param{
					{Name: "[account_name]", Description: "Account name to test (defaults to currently selected account)."},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
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
					fmt.Printf("%s Access successfully verified for account %q!\n", app.PrefixSuccess, session.Config.SelectedAccount)
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account test"},
					{Line: "mail_cli account test work-fastmail"},
				},
			},
			{
				Name:        "calendar",
				Aliases:     []string{"cal"},
				Title:       "account calendar [account_name]",
				Description: "Designate or show the calendar manager account.",
				UsageLine:   "mail_cli account calendar [account_name]",
				Parameters: []clihelp.Param{
					{Name: "[account_name]", Description: "Account name to configure calendar for."},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					if len(args) == 0 {
						calName := ""
						for _, acc := range *fc.Accounts {
							if acc.CalendarManager {
								calName = acc.Name
								break
							}
						}
						if calName == "" {
							fmt.Println("No calendar manager account designated.")
							return nil
						}
						fmt.Printf("Calendar manager account: %s\n", calName)
						return nil
					}

					accountName := args[0]
					for i := range *fc.Accounts {
						if strings.EqualFold((*fc.Accounts)[i].Name, accountName) {
							(*fc.Accounts)[i].CalendarManager = true
							if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
								return err
							}
							fmt.Printf("%s Successfully designated account %q as calendar manager.\n", app.PrefixSuccess, accountName)
							return nil
						}
					}
					return ErrAccountNotFound(accountName)
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account calendar"},
					{Line: "mail_cli account calendar personal-gmail"},
				},
			},
			{
				Name:        "login",
				Title:       "account login [account_name]",
				Description: "Perform interactive OAuth login for a Gmail or Outlook account.",
				UsageLine:   "mail_cli account login [account_name]",
				Parameters: []clihelp.Param{
					{Name: "[account_name]", Description: "Account name to re-authenticate (defaults to active account)."},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					name := ""
					if len(args) > 0 {
						name = args[0]
					} else {
						name = session.Config.SelectedAccount
					}
					var targetAcc *cfg_acc.AccountConfig
					for i := range *fc.Accounts {
						if strings.EqualFold((*fc.Accounts)[i].Name, name) {
							targetAcc = &(*fc.Accounts)[i]
							break
						}
					}
					if targetAcc == nil {
						return ErrAccountNotFoundAlt(name)
					}

					// Set the selected account so GetClient uses the right account
					session.Config.SelectedAccount = targetAcc.Name

					tokenDir := filepath.Dir(configPath)
					var tokenPath string
					if strings.EqualFold(targetAcc.Type, "gmail") {
						tokenPath = filepath.Join(tokenDir, fmt.Sprintf("token_%s.json", cfg_g.SanitizeLabelForCache(targetAcc.Name)))
					} else if strings.EqualFold(targetAcc.Type, "outlook") {
						tokenPath = filepath.Join(tokenDir, fmt.Sprintf("outlook_token_%s.json", cfg_g.SanitizeLabelForCache(targetAcc.Name)))
					}

					if tokenPath != "" {
						if _, err := os.Stat(tokenPath); err == nil {
							os.Remove(tokenPath)
							fmt.Printf("%s Deleted old token file: %s\n", app.PrefixInfo, tokenPath)
						}
					}

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
					fmt.Printf("%s Successfully authenticated and logged in account %q!\n", app.PrefixSuccess, targetAcc.Name)
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account login"},
					{Line: "mail_cli account login personal-gmail"},
				},
			},
			{
				Name:        "rename",
				Title:       "account rename <old_name> <new_name>",
				Description: "Rename an existing account's display name.",
				UsageLine:   "mail_cli account rename <old_name> <new_name>",
				Parameters: []clihelp.Param{
					{Name: "<old_name>", Description: "The current name of the account to rename."},
					{Name: "<new_name>", Description: "The new name for the account."},
				},
				Args: clihelp.ExactArgs(2),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					oldName := args[0]
					newName := args[1]

					if strings.TrimSpace(newName) == "" {
						return fmt.Errorf("new account name cannot be empty")
					}

					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					// Find old account by Name or DisplayName
					var oldAccIdx = -1
					for i, acc := range *fc.Accounts {
						if strings.EqualFold(acc.Name, oldName) || (acc.DisplayName != "" && strings.EqualFold(acc.DisplayName, oldName)) {
							oldAccIdx = i
							break
						}
					}

					if oldAccIdx == -1 {
						return ErrAccountNotFound(oldName)
					}

					// Check for new name collision against other accounts
					for i, acc := range *fc.Accounts {
						if i == oldAccIdx {
							continue
						}
						if strings.EqualFold(acc.Name, newName) || (acc.DisplayName != "" && strings.EqualFold(acc.DisplayName, newName)) {
							return ErrAccountExists(newName)
						}
					}

					// Rename only the DisplayName (directory structure and internal name stay unchanged!)
					(*fc.Accounts)[oldAccIdx].DisplayName = newName

					if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
						return err
					}

					fmt.Printf("%s Successfully renamed account %q to %q (display name updated).\n", app.PrefixSuccess, oldName, newName)
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account rename old-gmail new-gmail"},
				},
			},
			{
				Name:        "delete",
				Aliases:     []string{"del"},
				Title:       "account delete <account_name>",
				Description: "Delete an existing account and its credentials.",
				UsageLine:   "mail_cli account delete <account_name>",
				Parameters: []clihelp.Param{
					{Name: "<account_name>", Description: "The name of the account to delete."},
				},
				Args: clihelp.ExactArgs(1),
				Run: func(ctx *clihelp.Context) error {
					name := ctx.Args[0]
					_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
					if err != nil {
						return err
					}
					fc, err := cfg_g.LoadConfigFile(configPath)
					if err != nil {
						return err
					}

					foundIdx := -1
					for i, acc := range *fc.Accounts {
						if strings.EqualFold(acc.Name, name) || (acc.DisplayName != "" && strings.EqualFold(acc.DisplayName, name)) {
							foundIdx = i
							break
						}
					}

					if foundIdx == -1 {
						return ErrAccountNotFound(name)
					}

					targetAcc := (*fc.Accounts)[foundIdx]

					// Remove from slice
					*fc.Accounts = append((*fc.Accounts)[:foundIdx], (*fc.Accounts)[foundIdx+1:]...)

					if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
						return err
					}

					// Clean up token files if they exist
					tokenDir := filepath.Dir(configPath)
					if strings.EqualFold(targetAcc.Type, "gmail") {
						tokenPath := filepath.Join(tokenDir, "tokens", targetAcc.Name+".json")
						_ = os.Remove(tokenPath)
					} else if strings.EqualFold(targetAcc.Type, "outlook") {
						tokenPath := filepath.Join(tokenDir, fmt.Sprintf("outlook_token_%s.json", cfg_g.SanitizeLabelForCache(targetAcc.Name)))
						_ = os.Remove(tokenPath)
					}

					fmt.Printf("%s Successfully deleted account %q and cleaned up associated token files.\n", app.PrefixSuccess, targetAcc.Name)
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli account delete temp-account"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli account list"},
			{Line: "mail_cli account new personal-gmail"},
			{Line: "mail_cli account test"},
			{Line: "mail_cli account rename old-gmail new-gmail"},
			{Line: "mail_cli account delete temp-account"},
		},
	}
}
