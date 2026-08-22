package cfghandler

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

func HandleConfigShow(config *cfg_g.Config) error {
	fc, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	fmt.Println("======================================================================")
	app.ColorBoldCyan.Println("                        MAIL_CLI CONFIGURATION")
	fmt.Println("======================================================================")
	fmt.Printf("  Download/Cache Dir:  %s\n", config.DownloadDir)
	fmt.Printf("  Score Threshold:     %.2f\n", config.ScoreThreshold)
	fmt.Printf("  Limit:               %d\n", config.Limit)
	fmt.Printf("  Block Political:     %t\n", config.BlockPolitical)
	fmt.Printf("  Auto Unsubscribe:    %t\n", config.AutoUnsubscribe)
	browserDisp := "system default"
	if config.Browser != "" {
		browserDisp = config.Browser
	}
	fmt.Printf("  Browser Command:     %s\n", browserDisp)
	fmt.Println()
	app.ColorBoldGreen.Println("  ACCOUNTS:")
	for _, acc := range *fc.Accounts {
		prefix := "  "
		if strings.EqualFold(acc.Name, targetAcc.Name) {
			prefix = "* "
		}
		fmt.Printf("  %sAccount: %s (%s)\n", prefix, acc.Name, acc.Type)
		fmt.Printf("      Username:        %s\n", acc.Username)
		fmt.Printf("      Spam Folder:     %s\n", acc.SpamFolder)
		fmt.Printf("      Spam Learn:      %s\n", acc.SpamLearn)
		fmt.Printf("      Unspam Learn:    %s\n", acc.UnspamLearn)
		if len(acc.Aliases) > 0 {
			fmt.Printf("      Aliases:         %s\n", strings.Join(acc.Aliases, ", "))
		}
		fmt.Println()
	}
	fmt.Println("======================================================================")
	return nil
}

func PerformConfigValidation(config *cfg_g.Config) error {
	fc, _, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to read/parse config: %w", err)
	}

	app.ColorBoldCyan.Println("\n========================================================")
	app.ColorBoldCyan.Println("              MAIL_CLI CONFIG VALIDATION")
	app.ColorBoldCyan.Println("========================================================")
	fmt.Printf("Config file path: %s\n\n", configPath)

	hasErrors := false

	fmt.Printf("[*] Checking Download/Cache directory: %s\n", fc.DownloadDir)
	if fc.DownloadDir == "" {
		app.ColorBoldRed.Println("  [FAIL] download_dir is empty")
		hasErrors = true
	} else {
		if err := os.MkdirAll(fc.DownloadDir, 0700); err != nil {
			app.ColorBoldRed.Printf("  [FAIL] failed to create download_dir: %v\n", err)
			hasErrors = true
		} else {
			app.ColorBoldGreen.Println("  [OK] Cache directory exists and is writable")
		}
	}
	fmt.Println()

	if fc.Accounts == nil || len(*fc.Accounts) == 0 {
		app.ColorBoldRed.Println("[FAIL] No accounts configured in config.json")
		hasErrors = true
	} else {
		fmt.Printf("[*] Checking %d account(s)...\n", len(*fc.Accounts))
		for _, acc := range *fc.Accounts {
			fmt.Printf("  Account %q (%s):\n", acc.Name, acc.Type)
			if err := cfg_g.ValidateAccountParams(acc); err != nil {
				app.ColorBoldRed.Printf("    [FAIL] Parameters invalid: %v\n", err)
				hasErrors = true
			} else {
				app.ColorBoldGreen.Println("    [OK] Parameters are valid")
			}
			var host string
			if strings.EqualFold(acc.Type, "gmail") {
				host = acc.IMAPHost
			} else if strings.EqualFold(acc.Type, "jmap") {
				if u, err := url.Parse(acc.SessionURL); err == nil {
					host = u.Host
				}
			}
			if host != "" {
				if strings.Contains(host, ":") {
					host, _, _ = net.SplitHostPort(host)
				}
				fmt.Printf("    Checking reachability of host: %s...\n", host)
				addrs, err := net.LookupHost(host)
				if err != nil || len(addrs) == 0 {
					app.ColorBoldRed.Printf("    [FAIL] Could not resolve host name: %v\n", err)
					hasErrors = true
				} else {
					app.ColorBoldGreen.Printf("    [OK] Resolved host to: %s\n", strings.Join(addrs, ", "))
				}
			}
		}
	}
	fmt.Println()

	fmt.Println("[*] Checking local Bogofilter installation...")
	bogoPath, err := exec.LookPath("bogofilter")
	if err != nil {
		app.ColorBoldRed.Println("  [FAIL] Bogofilter executable not found in PATH.")
		app.ColorBoldRed.Println("         Please install bogofilter on your system.")
		app.ColorBoldRed.Println("         On Debian/Ubuntu, run: sudo apt-get install bogofilter")
		hasErrors = true
	} else {
		cmd := exec.Command(bogoPath, "-V")
		output, errRun := cmd.CombinedOutput()
		if errRun != nil {
			app.ColorBoldRed.Printf("  [FAIL] Bogofilter version check failed: %v\n", errRun)
			hasErrors = true
		} else {
			versionLine := ""
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				versionLine = strings.TrimSpace(lines[0])
			}
			app.ColorBoldGreen.Printf("  [OK] Bogofilter is available: %s (%s)\n", bogoPath, versionLine)
		}
	}

	app.ColorBoldCyan.Println("\n========================================================")
	if hasErrors {
		app.ColorBoldRed.Println("  VALIDATION FAILED: Please fix configuration errors above.")
	} else {
		app.ColorBoldGreen.Println("  VALIDATION SUCCESSFUL: Configuration is healthy!")
	}
	app.ColorBoldCyan.Println("========================================================")
	fmt.Println()

	if hasErrors {
		return fmt.Errorf("configuration validation failed")
	}
	return nil
}

func HandleConfigSet(config *cfg_g.Config, key, value string, accountSpecific bool) error {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	if key == "browser" {
		fc.Browser = value
		if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
			return err
		}
		config.Browser = value
		fmt.Printf("%s Successfully updated %s to %q.\n", app.PrefixSuccess, key, value)
		return nil
	}

	var accToModify *cfg_acc.AccountConfig
	for i := range *fc.Accounts {
		if strings.EqualFold((*fc.Accounts)[i].Name, targetAcc.Name) {
			accToModify = &(*fc.Accounts)[i]
			break
		}
	}
	if accToModify == nil {
		return fmt.Errorf("could not find account %q in config file", targetAcc.Name)
	}

	switch key {
	case "spam_learn":
		accToModify.SpamLearn = value
	case "unspam_learn":
		accToModify.UnspamLearn = value
	default:
		return fmt.Errorf("unsupported config key %q. Supported keys: spam_learn, unspam_learn, browser", key)
	}

	if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
		return err
	}
	if strings.EqualFold(config.SelectedAccount, accToModify.Name) || (config.SelectedAccount == "" && len(config.Accounts) > 0 && strings.EqualFold(config.Accounts[0].Name, accToModify.Name)) {
		if key == "spam_learn" {
			config.SpamLearn = value
		} else if key == "unspam_learn" {
			config.UnspamLearn = value
		}
	}
	fmt.Printf("%s Successfully updated %s to %q for account %s.\n", app.PrefixSuccess, key, value, accToModify.Name)
	return nil
}

func HandleConfigReset(config *cfg_g.Config, key string) error {
	fc, _, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	switch key {
	case "browser":
		fc.Browser = ""
		config.Browser = ""
	default:
		return fmt.Errorf("unsupported config reset key %q. Supported keys: browser", key)
	}

	if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
		return err
	}
	fmt.Printf("%s Successfully reset %s to system default.\n", app.PrefixSuccess, key)
	return nil
}
