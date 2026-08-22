package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"

	"github.com/sarielhp/clihelp"
)

func promptUser(prompt string, defaultValue string) string {
	fmt.Printf("%s", prompt)
	if defaultValue != "" {
		fmt.Printf(" [%s]", defaultValue)
	}
	fmt.Printf(": ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

var (
	flagAccountNewType string
	flagAccountNewTest bool
)

func accountNewCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "new",
		Title:       "account new <name> [type]",
		Description: "Add a new account template to config.json.",
		UsageLine:   "mail_cli account new <jmap|gmail|outlook> [name] [flags]",
		Parameters: []clihelp.Param{
			{Name: "<jmap|gmail|outlook>", Description: "Account backend provider type."},
			{Name: "[name]", Description: "Unique name for the account."},
		},
		Options: []clihelp.Option{
			clihelp.String(&flagAccountNewType, "--type <type>", "regular", "Account type: regular or test"),
			clihelp.Bool(&flagAccountNewTest, "--test", false, "Mark as test account (shortcut for --type test)"),
		},
		Args: clihelp.RangeArgs(1, 2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			acctType := strings.ToLower(args[0])
			name := ""
			if len(args) > 1 {
				name = args[1]
			}

			_, configPath, err := cfg_g.ResolveConfigPath(session.Config)
			if err != nil {
				return err
			}
			fc, err := cfg_g.LoadConfigFile(configPath)
			if err != nil {
				return err
			}

			acctMode := flagAccountNewType
			if flagAccountNewTest {
				acctMode = "test"
			}
			if acctMode != "regular" && acctMode != "test" {
				return ErrInvalidAccountMode(acctMode)
			}

			if name == "" {
				name = promptUser("Account name (unique identifier)", "")
				if name == "" {
					return fmt.Errorf("account name is required")
				}
			}

			// Check for duplicate account name
			for _, acc := range *fc.Accounts {
				if strings.EqualFold(acc.Name, name) {
					return ErrAccountExists(name)
				}
			}

			newAcc := cfg_acc.AccountConfig{
				Name:        name,
				Type:        acctType,
				AccountType: acctMode,
			}

			switch acctType {
			case "jmap":
				newAcc.Username = promptUser("Username (JMAP Email)", "")
				newAcc.Password = promptUser("Password (JMAP API Token)", "your-jmap-api-token")
				newAcc.SessionURL = promptUser("JMAP Session URL", "https://api.fastmail.com/jmap/session")
				newAcc.SpamFolder = "Spam"
				newAcc.UnspamLearn = "Inbox"
				newAcc.ReceivedFolder = "Inbox"
				newAcc.Whitelist = []string{}
				newAcc.Blacklist = []string{}
				newAcc.Rules = []cfg_acc.Rule{}

			case "gmail":
				newAcc.Username = promptUser("Username (Gmail Email)", "")
				newAcc.Password = promptUser("Password (or dummy value like 'oauth-auth')", "oauth-auth")
				newAcc.IMAPHost = promptUser("IMAP Host", "imap.gmail.com:993")
				newAcc.SpamFolder = "[Gmail]/Spam"
				newAcc.UnspamLearn = "INBOX"
				newAcc.ReceivedFolder = "received"

			case "outlook":
				newAcc.Username = promptUser("Username (Outlook Email)", "")
				newAcc.SpamFolder = "Junk Email"
				newAcc.UnspamLearn = "Inbox"
				newAcc.ReceivedFolder = "Archive"

			default:
				return ErrInvalidAccountType(acctType)
			}

			*fc.Accounts = append(*fc.Accounts, newAcc)
			if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
				return err
			}

			fmt.Printf("%s Successfully added new %s account entry %q to config file.\n", app.PrefixSuccess, strings.ToUpper(acctType), name)
			if acctType == "outlook" || acctType == "gmail" {
				fmt.Printf("Please run: %s account login %s to perform OAuth authentication\n", os.Args[0], name)
			} else {
				fmt.Printf("Please run: %s account test %s to verify connection\n", os.Args[0], name)
			}
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli account new personal-gmail"},
			{Line: "mail_cli account new work-fastmail jmap"},
		},
	}
}
