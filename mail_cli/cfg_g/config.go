package cfg_g

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/cfg_acc"
)

const (
	AppName       = "mail_cli"
	LegacyAppName = "gmail-spam-checker"
)

// evaluateDirty audits file-based config structures, applying default parameters where necessary and reporting if changes were made.
func evaluateDirty(fc *FileConfig, raw map[string]interface{}, config *Config) bool {
	dirty := false

	if _, ok := raw["imap_host"]; !ok || fc.IMAPHost == "" {
		fc.IMAPHost = config.IMAPHost
		dirty = true
	}
	if _, ok := raw["download_dir"]; !ok || fc.DownloadDir == "" || fc.DownloadDir == "./downloaded_emails" {
		fc.DownloadDir = config.DownloadDir
		dirty = true
	}
	if strings.Contains(fc.DownloadDir, LegacyAppName) {
		fc.DownloadDir = strings.ReplaceAll(fc.DownloadDir, LegacyAppName, AppName)
		dirty = true
	}
	if _, ok := raw["limit"]; !ok || fc.Limit == nil {
		val := config.Limit
		fc.Limit = &val
		dirty = true
	}
	if _, ok := raw["score_threshold"]; !ok || fc.ScoreThreshold == nil {
		val := config.ScoreThreshold
		fc.ScoreThreshold = &val
		dirty = true
	}
	if _, ok := raw["spam_folder"]; !ok || fc.SpamFolder == "" {
		fc.SpamFolder = config.SpamFolder
		dirty = true
	}
	if _, ok := raw["received_folder"]; !ok || fc.ReceivedFolder == "" {
		fc.ReceivedFolder = config.ReceivedFolder
		dirty = true
	}
	if _, ok := raw["spam_learn"]; !ok || fc.SpamLearn == "" {
		fc.SpamLearn = config.SpamLearn
		dirty = true
	}
	if _, ok := raw["unspam_learn"]; !ok || fc.UnspamLearn == "" {
		fc.UnspamLearn = config.UnspamLearn
		dirty = true
	}
	if _, ok := raw["allowed_languages"]; !ok || fc.AllowedLanguages == nil {
		val := config.AllowedLanguages
		fc.AllowedLanguages = &val
		dirty = true
	}
	if _, ok := raw["block_political"]; !ok || fc.BlockPolitical == nil {
		val := config.BlockPolitical
		fc.BlockPolitical = &val
		dirty = true
	}
	if _, ok := raw["auto_unsubscribe"]; !ok || fc.AutoUnsubscribe == nil {
		val := config.AutoUnsubscribe
		fc.AutoUnsubscribe = &val
		dirty = true
	}
	if _, ok := raw["editor"]; !ok || fc.Editor == "" {
		fc.Editor = config.Editor
		dirty = true
	}
	if _, ok := raw["editor_args"]; !ok || fc.EditorArgs == nil {
		fc.EditorArgs = config.EditorArgs
		dirty = true
	}
	if _, ok := raw["accounts"]; !ok || fc.Accounts == nil {
		val := []cfg_acc.AccountConfig{}
		fc.Accounts = &val
		dirty = true
	}

	if len(*fc.Accounts) == 0 {
		*fc.Accounts = []cfg_acc.AccountConfig{
			{
				Name:           "default",
				Type:           "gmail",
				Username:       fc.Username,
				Password:       fc.Password,
				IMAPHost:       fc.IMAPHost,
				SpamFolder:     fc.SpamFolder,
				ReceivedFolder: fc.ReceivedFolder,
				SpamLearn:      fc.SpamLearn,
				UnspamLearn:    fc.UnspamLearn,
			},
		}
		dirty = true
	}

	for i := range *fc.Accounts {
		acc := &(*fc.Accounts)[i]
		if acc.SpamLearn == "" {
			if strings.EqualFold(acc.Type, "gmail") {
				if acc.SpamFolder != "" {
					acc.SpamLearn = acc.SpamFolder
				} else {
					acc.SpamLearn = "[Gmail]/Spam"
				}
			} else {
				acc.SpamLearn = "LearnSpam"
			}
			dirty = true
		}
		if acc.UnspamLearn == "" {
			acc.UnspamLearn = "LearnUnSpam"
			dirty = true
		}
	}

	calendarManagerCount := 0
	gmailCount := 0
	gmailIndex := -1
	for i, acc := range *fc.Accounts {
		if acc.CalendarManager {
			calendarManagerCount++
		}
		if strings.EqualFold(acc.Type, "gmail") {
			gmailCount++
			gmailIndex = i
		}
	}

	if calendarManagerCount == 0 {
		if gmailCount == 1 {
			(*fc.Accounts)[gmailIndex].CalendarManager = true
			dirty = true
		}
	} else if calendarManagerCount > 1 {
		foundFirst := false
		for i := range *fc.Accounts {
			acc := &(*fc.Accounts)[i]
			if acc.CalendarManager {
				if !foundFirst {
					foundFirst = true
				} else {
					acc.CalendarManager = false
					dirty = true
				}
			}
		}
	}

	hasGlobalEntities := (fc.Whitelist != nil && len(*fc.Whitelist) > 0) ||
		(fc.Blacklist != nil && len(*fc.Blacklist) > 0) ||
		(fc.Rules != nil && len(*fc.Rules) > 0)

	if hasGlobalEntities {
		fmt.Printf("[*] Migrating global rules/whitelist/blacklist to account-level...\n")
		for i := range *fc.Accounts {
			acc := &(*fc.Accounts)[i]
			if fc.Whitelist != nil && len(*fc.Whitelist) > 0 {
				acc.Whitelist = append(acc.Whitelist, *fc.Whitelist...)
			}
			if fc.Blacklist != nil && len(*fc.Blacklist) > 0 {
				acc.Blacklist = append(acc.Blacklist, *fc.Blacklist...)
			}
			if fc.Rules != nil && len(*fc.Rules) > 0 {
				acc.Rules = append(acc.Rules, *fc.Rules...)
			}
		}
		fc.Whitelist = nil
		fc.Blacklist = nil
		fc.Rules = nil
		dirty = true
	}

	return dirty
}

// LoadConfig reads configuration from file, overrides with environment variables, then returns config.
func LoadConfig() (*Config, error) {
	homeDir, errHome := os.UserHomeDir()
	if errHome != nil {
		return nil, fmt.Errorf("failed to detect user home directory: %w", errHome)
	}

	oldConfigDir := filepath.Join(homeDir, ".config", LegacyAppName)
	newConfigDir := filepath.Join(homeDir, ".config", AppName)
	if _, errStat := os.Stat(oldConfigDir); errStat == nil {
		if _, errNew := os.Stat(newConfigDir); os.IsNotExist(errNew) {
			_ = os.Rename(oldConfigDir, newConfigDir)
		}
	}

	var oldCacheDir, newCacheDir string
	cacheDir, errCache := os.UserCacheDir()
	if errCache == nil {
		oldCacheDir = filepath.Join(cacheDir, LegacyAppName)
		newCacheDir = filepath.Join(cacheDir, AppName)
	} else {
		oldCacheDir = filepath.Join(homeDir, ".cache", LegacyAppName)
		newCacheDir = filepath.Join(homeDir, ".cache", AppName)
	}
	if _, errStat := os.Stat(oldCacheDir); errStat == nil {
		if _, errNew := os.Stat(newCacheDir); os.IsNotExist(errNew) {
			_ = os.Rename(oldCacheDir, newCacheDir)
		}
	}

	configDir := filepath.Join(homeDir, ".config", AppName)
	configPath := filepath.Join(configDir, "config.json")

	var defaultDownloadDir string
	if errCache == nil {
		defaultDownloadDir = filepath.Join(cacheDir, AppName)
	} else {
		defaultDownloadDir = filepath.Join(homeDir, ".cache", AppName)
	}

	config := &Config{
		IMAPHost:         "imap.gmail.com:993",
		Username:         os.Getenv("GMAIL_USER"),
		Password:         os.Getenv("GMAIL_PASS"),
		DownloadDir:      defaultDownloadDir,
		Limit:            1000,
		ScoreThreshold:   0.0,
		LearnSpam:        false,
		ForceLearn:       false,
		SpamFolder:       "[Gmail]/Spam",
		ReceivedFolder:   "received",
		SpamLearn:        "LearnSpam",
		UnspamLearn:      "LearnUnSpam",
		AllowedLanguages: []string{"english", "hebrew", "german", "french"},
		BlockPolitical:   true,
		AutoUnsubscribe:  false,
		Verbose:          false,
		Whitelist:        []string{},
		Blacklist:        []string{},
		Rules:            []cfg_acc.Rule{},
		Editor:           "emacs",
		EditorArgs:       []string{"-nw"},
	}

	_ = os.MkdirAll(configDir, 0700)

	configExists := true
	if _, errStat := os.Stat(configPath); os.IsNotExist(errStat) {
		configExists = false
	}

	var fc FileConfig
	dirty := false

	if !configExists {
		fc = FileConfig{
			Username:         "your-email@gmail.com",
			Password:         "your-app-password",
			IMAPHost:         config.IMAPHost,
			DownloadDir:      config.DownloadDir,
			Limit:            &config.Limit,
			ScoreThreshold:   &config.ScoreThreshold,
			SpamFolder:       config.SpamFolder,
			ReceivedFolder:   config.ReceivedFolder,
			SpamLearn:        config.SpamLearn,
			UnspamLearn:      config.UnspamLearn,
			AllowedLanguages: &config.AllowedLanguages,
			BlockPolitical:   &config.BlockPolitical,
			AutoUnsubscribe:  &config.AutoUnsubscribe,
			Whitelist:        nil,
			Blacklist:        nil,
			Rules:            nil,
			Accounts:         &config.Accounts,
		}
		dirty = true
	} else {
		data, errRead := os.ReadFile(configPath)
		if errRead != nil {
			return nil, fmt.Errorf("error reading config file %s: %w", configPath, errRead)
		}
		loadedFc, errLoad := LoadConfigFile(configPath)
		if errLoad != nil {
			return nil, fmt.Errorf("error loading config: %w", errLoad)
		}
		fc = *loadedFc

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			slog.Warn("failed to parse config for dirty check", slog.Any("error", err))
		}
		if raw == nil {
			raw = make(map[string]interface{})
		}

		dirty = evaluateDirty(&fc, raw, config)
	}

	if dirty {
		if errSave := SaveConfigFile(configPath, &fc); errSave != nil {
			return nil, fmt.Errorf("failed to write config file: %w", errSave)
		}

		// Reload immediately to verify the change worked
		verifyFc, errReload := LoadConfigFile(configPath)
		if errReload != nil {
			return nil, fmt.Errorf("failed to reload config for verification: %w", errReload)
		}

		verifyData, errReadVerify := os.ReadFile(configPath)
		if errReadVerify != nil {
			return nil, fmt.Errorf("failed to read config for verification: %w", errReadVerify)
		}
		var verifyRaw map[string]interface{}
		if err := json.Unmarshal(verifyData, &verifyRaw); err != nil {
			slog.Warn("failed to parse verified config", slog.Any("error", err))
		}
		if verifyRaw == nil {
			verifyRaw = make(map[string]interface{})
		}

		verifyDirty := evaluateDirty(verifyFc, verifyRaw, config)
		if verifyDirty {
			return nil, fmt.Errorf("config update verification failed: configuration remains dirty after writing defaults")
		}

		fc = *verifyFc

		if !configExists {
			fmt.Printf("[*] Config file automatically created at: %s\n", configPath)
			fmt.Printf("[*] Please configure your credentials inside this file before running again.\n")
		} else {
			fmt.Printf("[*] Updated config file at %s with missing defaults\n", configPath)
		}
	}

	if fc.Username != "" {
		config.Username = fc.Username
	}
	if fc.Password != "" {
		config.Password = fc.Password
	}
	if fc.IMAPHost != "" {
		config.IMAPHost = fc.IMAPHost
	}
	if fc.DownloadDir != "" {
		config.DownloadDir = fc.DownloadDir
	}
	if fc.Limit != nil {
		config.Limit = *fc.Limit
	}
	if fc.ScoreThreshold != nil {
		config.ScoreThreshold = *fc.ScoreThreshold
	}
	if fc.SpamFolder != "" {
		config.SpamFolder = fc.SpamFolder
	}
	if fc.ReceivedFolder != "" {
		config.ReceivedFolder = fc.ReceivedFolder
	}
	if fc.SpamLearn != "" {
		config.SpamLearn = fc.SpamLearn
	} else {
		config.SpamLearn = "LearnSpam"
	}
	if fc.UnspamLearn != "" {
		config.UnspamLearn = fc.UnspamLearn
	} else {
		config.UnspamLearn = "LearnUnSpam"
	}
	if fc.AllowedLanguages != nil {
		config.AllowedLanguages = *fc.AllowedLanguages
	}
	if fc.BlockPolitical != nil {
		config.BlockPolitical = *fc.BlockPolitical
	}
	if fc.AutoUnsubscribe != nil {
		config.AutoUnsubscribe = *fc.AutoUnsubscribe
	}
	if fc.Accounts != nil {
		config.Accounts = *fc.Accounts
	}
	config.Browser = fc.Browser
	if fc.Editor != "" {
		config.Editor = fc.Editor
	}
	if fc.EditorArgs != nil {
		config.EditorArgs = fc.EditorArgs
	}
	if fc.ReadOnly != nil {
		config.ReadOnly = *fc.ReadOnly
	}

	if len(config.Accounts) == 0 {
		config.Accounts = []cfg_acc.AccountConfig{
			{
				Name:            "default",
				Type:            "gmail",
				Username:        config.Username,
				Password:        config.Password,
				IMAPHost:        config.IMAPHost,
				SpamFolder:      config.SpamFolder,
				ReceivedFolder:  config.ReceivedFolder,
				SpamLearn:       config.SpamFolder,
				UnspamLearn:     config.UnspamLearn,
				CalendarManager: true,
			},
		}
	}

	progName := filepath.Base(os.Args[0])
	if progName != "mail_cli" && progName != "main" {
	AccountLoop:
		for _, acc := range config.Accounts {
			for _, alias := range acc.Aliases {
				if strings.EqualFold(alias, progName) {
					config.SelectedAccount = acc.Name
					break AccountLoop
				}
			}
		}
	}

	return config, nil
}

func listContains(sender string, list []string) bool {
	cleanSender := strings.ToLower(strings.TrimSpace(sender))
	for _, item := range list {
		if cleanSender == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func IsWhitelisted(sender string, whitelist []string) bool {
	return listContains(sender, whitelist)
}

func IsBlacklisted(sender string, blacklist []string) bool {
	return listContains(sender, blacklist)
}
