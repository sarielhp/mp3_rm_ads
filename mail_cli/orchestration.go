package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cfg_g"
	"mail_cli/last"
	"mail_cli/mailclient"
	"mail_cli/scan"
	"mail_cli/show"
)

// preprocessArgs rewrites flags to map flag-style subcommands (-wlist, -rule, -labels, -accounts) to standard CLI subcommands,
// and single-dash multi-character flags to double-dash flags. It also processes account index flags (-1, -2, etc.).
func preprocessArgs(args []string, config *Config) []string {
	if len(args) <= 1 {
		return args
	}

	var result []string
	result = append(result, args[0])
	aliasExpanded := false

	for i := 1; i < len(args); i++ {
		arg := args[i]

		// Process -m / --move argument if followed by a value
		if (arg == "-m" || arg == "--move" || arg == "-move") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			result = append(result, "-m="+args[i+1])
			i++
			continue
		}

		// Process -im / --inbox-move argument if followed by a value
		if (arg == "-im" || arg == "--inbox-move" || arg == "-inbox-move") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			result = append(result, "--inbox-move="+args[i+1])
			i++
			continue
		}

		// Process "ss" as "scan spam"
		if arg == "ss" {
			result = append(result, "scan", "spam")
			aliasExpanded = true
			continue
		}

		// Process "sb" as "spam bye"
		if arg == "sb" {
			result = append(result, "spam", "bye")
			aliasExpanded = true
			continue
		}

		// Convert "help" subcommand to "--help" so clihelp's Execute stops
		// after rendering help instead of continuing to the default Run handler.
		//   help           → --help
		//   help scan      → scan --help
		//   help rule add  → rule add --help
		if arg == "help" {
			if i+1 < len(args) {
				result = append(result, args[i+1:]...)
				result = append(result, "--help")
			} else {
				result = append(result, "--help")
			}
			break
		}

		// Process numeric account index flags (-1, -2, etc.)
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 1 {
			if idx, err := strconv.Atoi(arg[1:]); err == nil && idx > 0 {
				if idx <= len(config.Accounts) {
					config.SelectedAccount = config.Accounts[idx-1].Name
				} else {
					fmt.Fprintf(os.Stderr, "Error: Account index %d is out of range. You have %d configured account(s).\n", idx, len(config.Accounts))
					os.Exit(1)
				}
				continue
			}
		}

		if arg == "-wlist" || arg == "--wlist" {
			result = append(result, "wlist")
		} else if arg == "-blist" || arg == "--blist" {
			result = append(result, "blist")
		} else if arg == "-rule" || arg == "--rule" {
			result = append(result, "rule")
		} else if arg == "-labels" || arg == "--labels" {
			result = append(result, "labels")
		} else if arg == "-filter" || arg == "--filter" {
			result = append(result, "filter")
		} else if arg == "-spam" || arg == "--spam" {
			result = append(result, "spam")
		} else if arg == "-test" || arg == "--test" {
			result = append(result, "test")
		} else if arg == "-scan" || arg == "--scan" {
			result = append(result, "scan")
		} else if arg == "-show" || arg == "--show" {
			result = append(result, "show")
		} else if arg == "-accounts" || arg == "--accounts" {
			result = append(result, "account")
		} else if arg == "-learn_ham" || arg == "-learn-ham" || arg == "--learn_ham" || arg == "--learn-ham" {
			result = append(result, "learn_ham")
		} else if arg == "-unspam" || arg == "--unspam" {
			result = append(result, "unspam")
		} else if arg == "-config" || arg == "--config" {
			result = append(result, "config")
		} else if arg == "-cache" || arg == "--cache" {
			result = append(result, "cache")
		} else if arg == "-arc" || arg == "--arc" {
			result = append(result, "arc")
		} else if arg == "-caladd" || arg == "--caladd" {
			result = append(result, "caladd")
		} else if arg == "-ro" || arg == "-read-only" || arg == "--read-only" || arg == "--dry-run" {
			result = append(result, "--read-only")
		} else if arg == "-version" || arg == "--version" {
			result = append(result, "--version")
		} else if arg == "-do" {
			result = append(result, "--do")
		} else if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			result = append(result, "-"+arg)
		} else {
			result = append(result, arg)
		}
	}

	if aliasExpanded {
		displayArgs := make([]string, len(result))
		copy(displayArgs, result)
		if len(displayArgs) > 0 {
			displayArgs[0] = filepath.Base(displayArgs[0])
		}
		fmt.Printf("[%s]\n", app.ColorGreen.Sprint(strings.Join(displayArgs, " ")))
	}

	return result
}

func getClientForSelected(config *Config) (MailClient, error) {
	var err error
	var client MailClient
	ForEachAccount(config, "", func(acc AccountConfig, _ string) (bool, error) {
		client, err = NewMailClient(acc, config)
		return true, nil // stop after first
	})
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("no accounts configured in config.json")
	}
	return client, nil
}

func getClientForAccountIndex(config *Config, index int) (MailClient, error) {
	if index < 1 || index > len(config.Accounts) {
		return nil, fmt.Errorf("account index %d is out of range. You have %d configured account(s)", index, len(config.Accounts))
	}
	acc := config.Accounts[index-1]
	return NewMailClient(acc, config)
}

// AccountJob is called for each matching account with the resolved label prefix.
// Return stop=true to abort iteration early.
type AccountJob func(acc AccountConfig, labelPrefix string) (stop bool, err error)

func accountNames(config *Config) []string {
	names := make([]string, len(config.Accounts))
	for i, a := range config.Accounts {
		names[i] = a.Name
	}
	return names
}

// ForEachAccount resolves labelSpec (which may contain "N:" prefix for account index
// or "accountName:" prefix for account name) and iterates over matching accounts.
func ForEachAccount(config *Config, labelSpec string, fn AccountJob) error {
	spec, err := cfg_g.ParseAccountLabelSpec(labelSpec)
	if err != nil {
		return err
	}
	var accounts []AccountConfig

	switch {
	case spec.AccountName != "":
		found := false
		for _, a := range config.Accounts {
			if strings.EqualFold(a.Name, spec.AccountName) {
				accounts = []AccountConfig{a}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("account %q not found (use one of: %s)",
				spec.AccountName, strings.Join(accountNames(config), ", "))
		}
	case config.SelectedAccount != "":
		found := false
		for _, a := range config.Accounts {
			if strings.EqualFold(a.Name, config.SelectedAccount) {
				accounts = []AccountConfig{a}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("account %q not found in config.json", config.SelectedAccount)
		}
	default:
		if len(config.Accounts) == 0 {
			return fmt.Errorf("no accounts configured in config.json")
		}
		accounts = config.Accounts
	}

	for _, acc := range accounts {
		if stop, err := fn(acc, spec.Label); stop || err != nil {
			return err
		}
	}
	return nil
}

func runScanOnAccounts(config *Config, labelPrefix string, moveSpam string, moveInbox string) (int, error) {
	totalMoved := 0
	err := ForEachAccount(config, labelPrefix, func(acc AccountConfig, prefix string) (bool, error) {
		if config.Verbose {
			fmt.Printf("%s Running scan for account: %s (%s)...\n", app.PrefixInfo, acc.Name, acc.Type)
		}
		client, err := NewMailClient(acc, config)
		if err != nil {
			return true, err
		}
		if err := client.Validate(); err != nil {
			return true, err
		}
		realPrefix := prefix
		if strings.EqualFold(realPrefix, "inbox") {
			realPrefix = client.InboxFolder()
		}
		resolvedPrefix, err := mailclient.ResolveLabel(client, realPrefix)
		if err != nil {
			return true, err
		}
		moved, err := scan.Perform(client, client.Config(), resolvedPrefix, moveSpam, moveInbox)
		if err != nil {
			return true, err
		}
		totalMoved += moved
		return false, nil
	})
	return totalMoved, err
}

func runShowOnAccounts(config *Config, labelPrefix string, targetMsgID string) error {
	err := ForEachAccount(config, labelPrefix, func(acc AccountConfig, prefix string) (bool, error) {
		client, err := NewMailClient(acc, config)
		if err != nil {
			return true, err
		}
		if err := client.Validate(); err != nil {
			return true, err
		}
		if targetMsgID == "" {
			if msgID, folderName, errFind := cache.FindCachedEmailByID(client.Config().DownloadDir, prefix); errFind == nil {
				if errShow := show.ByID(client, client.Config(), msgID, folderName); errShow != nil {
					return true, errShow
				}
				return true, nil // found via cache lookup, done
			}
		}

		realPrefix := prefix
		if strings.EqualFold(realPrefix, "inbox") {
			realPrefix = client.InboxFolder()
		}
		resolvedPrefix, err := mailclient.ResolveLabel(client, realPrefix)
		if err != nil {
			return true, err
		}
		if err := show.Perform(client, client.Config(), resolvedPrefix, targetMsgID); err != nil {
			return true, err
		}
		return false, nil
	})
	return err
}

func runLastOnAccounts(config *Config, n int) error {
	client, err := getClientForSelected(config)
	if err != nil {
		return err
	}
	if err := client.Validate(); err != nil {
		return err
	}
	return last.Perform(client, client.Config(), n)
}

func exportRulesToFile(config *Config, filePath string) error {
	_, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	rules := targetAcc.Rules
	if rules == nil {
		rules = []Rule{}
	}

	rulesBytes, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	if err := os.WriteFile(filePath, rulesBytes, 0600); err != nil {
		return fmt.Errorf("failed to write rules to file %s: %w", filePath, err)
	}

	fmt.Printf("%s Successfully exported %d rule(s) to %s\n", app.PrefixSuccess, len(rules), filePath)
	return nil
}

func importRulesFromFile(config *Config, filePath string) error {
	importData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read import file %s: %w", filePath, err)
	}

	var importedRules []Rule
	if err := json.Unmarshal(importData, &importedRules); err != nil {
		return fmt.Errorf("failed to parse import file %s: %w", filePath, err)
	}

	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	if targetAcc.Rules == nil {
		targetAcc.Rules = []Rule{}
	}

	existingMap := make(map[string]bool)
	for _, r := range targetAcc.Rules {
		existingMap[strings.ToLower(strings.TrimSpace(r.Sender))] = true
	}

	importedCount := 0
	ignoredCount := 0

	for _, r := range importedRules {
		cleanSender := strings.ToLower(strings.TrimSpace(r.Sender))
		if cleanSender == "" {
			continue
		}
		if existingMap[cleanSender] {
			ignoredCount++
		} else {
			targetAcc.Rules = append(targetAcc.Rules, r)
			existingMap[cleanSender] = true
			importedCount++
		}
	}

	err = cfg_g.SaveConfigFile(configPath, fc)
	if err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	fmt.Printf("%s Successfully imported %d rule(s) from %s (ignored %d already existing rule(s))\n", app.PrefixSuccess, importedCount, filePath, ignoredCount)
	return nil
}
