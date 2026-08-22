package cfg_g

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mail_cli/cfg_acc"
)

func ResolveConfigPath(config *Config) (string, string, error) {
	if config.ConfigDir == "" {
		return "", "", fmt.Errorf("config directory not initialized")
	}
	configPath := filepath.Join(config.ConfigDir, "config.json")
	return config.ConfigDir, configPath, nil
}

func LoadConfigFile(configPath string) (*FileConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", configPath, err)
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", configPath, err)
	}

	configDir := filepath.Dir(configPath)
	if fc.Accounts != nil {
		for i := range *fc.Accounts {
			acc := &(*fc.Accounts)[i]
			sanitizedName := SanitizeLabelForCache(acc.Name)
			accountsDir := filepath.Join(configDir, "accounts", sanitizedName)
			accJsonPath := filepath.Join(accountsDir, "account.json")

			// Check if account.json exists under accounts/<name>/
			if accJsonData, err := os.ReadFile(accJsonPath); err == nil {
				var diskAcc cfg_acc.AccountConfig
				if json.Unmarshal(accJsonData, &diskAcc) == nil {
					// Populate fields from account.json
					diskAcc.Name = acc.Name
					acc.DisplayName = diskAcc.DisplayName
					acc.Type = diskAcc.Type
					acc.Username = diskAcc.Username
					acc.Password = diskAcc.Password
					acc.IMAPHost = diskAcc.IMAPHost
					acc.SessionURL = diskAcc.SessionURL
					acc.SpamFolder = diskAcc.SpamFolder
					acc.ReceivedFolder = diskAcc.ReceivedFolder
					acc.SpamLearn = diskAcc.SpamLearn
					acc.UnspamLearn = diskAcc.UnspamLearn
					acc.Aliases = diskAcc.Aliases
					acc.CalendarManager = diskAcc.CalendarManager
					acc.AccountType = diskAcc.AccountType
					acc.ReadOnly = diskAcc.ReadOnly
				}
			}

			// 1. Whitelist
			if len(acc.Whitelist) == 0 {
				whitelistPath := filepath.Join(accountsDir, "whitelist.json")
				if _, err := os.Stat(whitelistPath); os.IsNotExist(err) {
					// Fallback to legacy path
					whitelistPath = filepath.Join(configDir, fmt.Sprintf("%s_whitelist.json", sanitizedName))
				}
				if whitelistData, err := os.ReadFile(whitelistPath); err == nil {
					var whitelist []string
					if json.Unmarshal(whitelistData, &whitelist) == nil {
						acc.Whitelist = whitelist
					}
				} else if acc.Whitelist == nil {
					acc.Whitelist = []string{}
				}
			}

			// 2. Blacklist
			if len(acc.Blacklist) == 0 {
				blacklistPath := filepath.Join(accountsDir, "blacklist.json")
				if _, err := os.Stat(blacklistPath); os.IsNotExist(err) {
					// Fallback to legacy path
					blacklistPath = filepath.Join(configDir, fmt.Sprintf("%s_blacklist.json", sanitizedName))
				}
				if blacklistData, err := os.ReadFile(blacklistPath); err == nil {
					var blacklist []string
					if json.Unmarshal(blacklistData, &blacklist) == nil {
						acc.Blacklist = blacklist
					}
				} else if acc.Blacklist == nil {
					acc.Blacklist = []string{}
				}
			}

			// 3. Rules
			if len(acc.Rules) == 0 {
				rulesPath := filepath.Join(accountsDir, "rules.json")
				if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
					// Fallback to legacy path
					rulesPath = filepath.Join(configDir, fmt.Sprintf("%s_rules.json", sanitizedName))
				}
				if rulesData, err := os.ReadFile(rulesPath); err == nil {
					var rules []cfg_acc.Rule
					if json.Unmarshal(rulesData, &rules) == nil {
						acc.Rules = rules
					}
				} else if acc.Rules == nil {
					acc.Rules = []cfg_acc.Rule{}
				}
			}
		}
	}
	return &fc, nil
}

func SaveConfigFile(configPath string, fc *FileConfig) error {
	configDir := filepath.Dir(configPath)
	_ = os.MkdirAll(configDir, 0700)

	// 1. Save separate files for each account
	if fc.Accounts != nil {
		for i := range *fc.Accounts {
			acc := &(*fc.Accounts)[i]
			sanitizedName := SanitizeLabelForCache(acc.Name)
			accountsDir := filepath.Join(configDir, "accounts", sanitizedName)
			_ = os.MkdirAll(accountsDir, 0700)

			// Save core account config to account.json (without lists, which live in separate files)
			accDisk := *acc
			accDisk.Whitelist = nil
			accDisk.Blacklist = nil
			accDisk.Rules = nil
			if accData, err := json.MarshalIndent(accDisk, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(accountsDir, "account.json"), accData, 0600)
			}

			// Save Whitelist
			whitelistPath := filepath.Join(accountsDir, "whitelist.json")
			wList := acc.Whitelist
			if wList == nil {
				wList = []string{}
			}
			wData, err := json.MarshalIndent(wList, "", "  ")
			if err == nil {
				_ = os.WriteFile(whitelistPath, wData, 0600)
			}

			// Save Blacklist
			blacklistPath := filepath.Join(accountsDir, "blacklist.json")
			bList := acc.Blacklist
			if bList == nil {
				bList = []string{}
			}
			bData, err := json.MarshalIndent(bList, "", "  ")
			if err == nil {
				_ = os.WriteFile(blacklistPath, bData, 0600)
			}

			// Save Rules
			rulesPath := filepath.Join(accountsDir, "rules.json")
			rList := acc.Rules
			if rList == nil {
				rList = []cfg_acc.Rule{}
			}
			rData, err := json.MarshalIndent(rList, "", "  ")
			if err == nil {
				_ = os.WriteFile(rulesPath, rData, 0600)
			}

			// Clean up old legacy files from configDir if they exist
			legacyWPath := filepath.Join(configDir, fmt.Sprintf("%s_whitelist.json", sanitizedName))
			_ = os.Remove(legacyWPath)
			legacyBPath := filepath.Join(configDir, fmt.Sprintf("%s_blacklist.json", sanitizedName))
			_ = os.Remove(legacyBPath)
			legacyRPath := filepath.Join(configDir, fmt.Sprintf("%s_rules.json", sanitizedName))
			_ = os.Remove(legacyRPath)
		}
	}

	// 2. Clone FileConfig and remove Whitelist, Blacklist, Rules, and full configurations from the accounts
	// to prevent them from being written to config.json
	var fcClone FileConfig
	cloneData, err := json.Marshal(fc)
	if err != nil {
		return fmt.Errorf("failed to clone config: %w", err)
	}
	if err := json.Unmarshal(cloneData, &fcClone); err != nil {
		return fmt.Errorf("failed to restore clone config: %w", err)
	}

	if fcClone.Accounts != nil {
		for i := range *fcClone.Accounts {
			acc := &(*fcClone.Accounts)[i]
			*acc = cfg_acc.AccountConfig{
				Name: acc.Name,
				Type: acc.Type,
			}
		}
	}
	fcClone.Whitelist = nil
	fcClone.Blacklist = nil
	fcClone.Rules = nil

	tBytes, err := json.MarshalIndent(fcClone, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, tBytes, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}
	return nil
}

func FindAccountLocally(fc *FileConfig, selectedName string) *cfg_acc.AccountConfig {
	if fc == nil || fc.Accounts == nil || len(*fc.Accounts) == 0 {
		return nil
	}
	if selectedName != "" {
		for i := range *fc.Accounts {
			if strings.EqualFold((*fc.Accounts)[i].Name, selectedName) {
				return &(*fc.Accounts)[i]
			}
		}
		return nil
	}
	return &(*fc.Accounts)[0]
}

func ResolveAccountFromConfig(config *Config) (*FileConfig, *cfg_acc.AccountConfig, string, string, error) {
	configDir, configPath, err := ResolveConfigPath(config)
	if err != nil {
		return nil, nil, "", "", err
	}
	fc, err := LoadConfigFile(configPath)
	if err != nil {
		return nil, nil, "", "", err
	}
	if fc.Accounts == nil || len(*fc.Accounts) == 0 {
		return nil, nil, configDir, configPath, fmt.Errorf("no accounts configured")
	}
	selectedAccName := config.SelectedAccount
	if selectedAccName == "" && len(*fc.Accounts) > 0 {
		selectedAccName = (*fc.Accounts)[0].Name
	}
	targetAcc := FindAccountLocally(fc, selectedAccName)
	if targetAcc == nil {
		targetAcc = &(*fc.Accounts)[0]
	}
	return fc, targetAcc, configDir, configPath, nil
}

func ValidateAccountParams(acc cfg_acc.AccountConfig) error {
	if acc.Name == "" {
		return fmt.Errorf("account name is missing")
	}
	if acc.Type == "" {
		return fmt.Errorf("account type is missing")
	}
	if acc.Username == "" || acc.Username == "your-email@gmail.com" || acc.Username == "your-email@example.com" {
		return fmt.Errorf("account username is missing or contains placeholder value")
	}
	if !strings.EqualFold(acc.Type, "outlook") {
		if acc.Password == "" || acc.Password == "your-app-password" || acc.Password == "your-jmap-api-token" {
			return fmt.Errorf("account password is missing or contains placeholder value")
		}
	}
	if strings.EqualFold(acc.Type, "gmail") {
		if acc.IMAPHost == "" {
			return fmt.Errorf("imap_host is missing")
		}
	} else if strings.EqualFold(acc.Type, "jmap") {
		if acc.SessionURL == "" {
			return fmt.Errorf("jmap_session_url is missing")
		}
	} else if strings.EqualFold(acc.Type, "outlook") {
		// Outlook only needs Username (email) and OAuth token (stored on disk).
	} else {
		return fmt.Errorf("unsupported account type %q", acc.Type)
	}
	if acc.SpamFolder == "" {
		return fmt.Errorf("spam_folder is missing")
	}
	if acc.ReceivedFolder == "" {
		return fmt.Errorf("received_folder is missing")
	}
	return nil
}

func SanitizeLabelForCache(label string) string {
	var sb strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return strings.ToLower(sb.String())
}

// AutoBlacklistInternal adds an internal blacklist rule for the sender if not already present.
func AutoBlacklistInternal(config *Config, senderEmail string) error {
	fc, targetAcc, _, configPath, err := ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	// 1. Check if the sender is already whitelisted
	for _, w := range targetAcc.Whitelist {
		if strings.EqualFold(w, senderEmail) {
			return nil
		}
	}

	// 2. Determine target label for Spam Learn
	targetLabel := targetAcc.SpamLearn
	if targetLabel == "" {
		targetLabel = "LearnSpam"
	}

	// 3. Check if a rule for this sender already exists
	for _, r := range targetAcc.Rules {
		if strings.EqualFold(r.Sender, senderEmail) {
			return nil
		}
	}

	// 4. Create internal rule
	targetAcc.Rules = append(targetAcc.Rules, cfg_acc.Rule{
		Sender:   senderEmail,
		Label:    targetLabel,
		Internal: true,
	})

	fmt.Printf("[+] Auto-blacklisted sender '%s' locally (internal rule pointing to %s).\n",
		senderEmail, targetLabel)

	// 5. Save back to config
	return SaveConfigFile(configPath, fc)
}

// AccountLabelSpec holds a parsed label specification with an optional account indicator.
type AccountLabelSpec struct {
	AccountIndex int    // 1-based account index (1, 2, ...), or 0 if omitted.
	AccountName  string // Account name, set when prefix is not a number (e.g. "GMail:folder").
	Label        string // Clean label path without "account:" prefix.
}

// ParseAccountLabelSpec parses a string like "GMail:inbox" or "inbox".
// If raw starts with an account name followed by a colon, the two are separated.
// Only prefixes starting with a letter are treated as account names.
// Numeric prefixes like "1:label" are rejected since account indices are fragile.
// Returns an error if a numeric prefix is used.
func ParseAccountLabelSpec(raw string) (AccountLabelSpec, error) {
	if idx := strings.Index(raw, ":"); idx > 0 {
		prefix := raw[:idx]
		if num, err := strconv.Atoi(prefix); err == nil && num > 0 {
			return AccountLabelSpec{}, fmt.Errorf("numeric account index (%d:) is no longer supported — use account name instead (e.g. %q)", num, raw[idx+1:])
		}
		// Non-numeric prefix — treat as account name if it starts with a letter
		if len(prefix) > 0 && ((prefix[0] >= 'a' && prefix[0] <= 'z') || (prefix[0] >= 'A' && prefix[0] <= 'Z')) {
			return AccountLabelSpec{
				AccountName: prefix,
				Label:       raw[idx+1:],
			}, nil
		}
	}
	return AccountLabelSpec{
		Label: raw,
	}, nil
}
