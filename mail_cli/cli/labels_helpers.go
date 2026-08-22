package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

// labelsClient resolves and validates a MailClient from the session.
func labelsClient(session *app.Session) (mailclient.MailClient, error) {
	if session.GetClient == nil {
		return nil, ErrClientNotConfigured()
	}
	client, err := session.GetClient(session.Config)
	if err != nil {
		return nil, err
	}
	if err := client.Validate(); err != nil {
		return nil, err
	}
	return client, nil
}

func parseExtNoZero(args []string, isExt, isNoZero bool) (bool, bool) {
	for _, arg := range args {
		if strings.EqualFold(arg, "ext") {
			isExt = true
		} else if strings.EqualFold(arg, "nozero") {
			isNoZero = true
		}
	}
	return isExt, isNoZero
}

func cfgResolveAccount(session *app.Session) (*cfg_g.FileConfig, *cfg_acc.AccountConfig, string, string, error) {
	fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return nil, nil, "", "", err
	}
	return fc, targetAcc, "", configPath, nil
}

func cfgSaveConfig(configPath string, fc *cfg_g.FileConfig) error {
	return cfg_g.SaveConfigFile(configPath, fc)
}

func updateRuleLabels(fc *cfg_g.FileConfig, targetAcc *cfg_acc.AccountConfig, oldName, newName string) {
	for i, rule := range targetAcc.Rules {
		if strings.EqualFold(rule.Label, oldName) {
			targetAcc.Rules[i].Label = newName
			fmt.Printf("%s Updated routing rule '%s' to reference renamed label %q\n", app.PrefixSuccess, rule.Sender, newName)
		}
	}
}

// ResolveCacheDir returns the cache directory path for the current account.
func ResolveCacheDir(config *cfg_g.Config) (string, error) {
	_, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return "", err
	}
	accountDir := cfg_g.SanitizeLabelForCache(targetAcc.Name)
	if accountDir == "" {
		accountDir = "default"
	}
	return filepath.Join(config.DownloadDir, accountDir), nil
}

// SearchLabels searches cached labels matching all given substring patterns.
func SearchLabels(config *cfg_g.Config, patterns []string) ([]string, error) {
	cacheDir, err := ResolveCacheDir(config)
	if err != nil {
		return nil, err
	}
	cs := &cache.DiskCacheStore{DownloadDir: cacheDir}
	items, cacheErr := cs.GetLabelItems()
	_, ageErr := cs.CachedLabelItemsAge()

	if cacheErr != nil || ageErr != nil {
		return nil, fmt.Errorf("labels cache not available: %w", cacheErr)
	}

	var lowerPatterns []string
	for _, p := range patterns {
		lowerPatterns = append(lowerPatterns, strings.ToLower(p))
	}

	var matches []string
	for _, item := range items {
		lowerName := strings.ToLower(item.FullName)
		allMatched := true
		for _, lp := range lowerPatterns {
			if !strings.Contains(lowerName, lp) {
				allMatched = false
				break
			}
		}
		if allMatched {
			matches = append(matches, item.FullName)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// ResolveUniqueMatch takes a search pattern and a list of matching candidates.
func ResolveUniqueMatch(pattern string, candidates []string) string {
	for _, c := range candidates {
		if strings.EqualFold(c, pattern) {
			return c
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func runLabelsListFull(session *app.Session, isExt bool, isNoZero bool, realOnly bool) error {
	client, err := labelsClient(session)
	if err != nil {
		return err
	}

	var items []cfg_acc.LabelItem
	if realOnly {
		items, err = getLabelItemsReal(client)
	} else {
		items, err = client.GetLabelItems()
	}
	if err != nil {
		return err
	}

	hideZero := isNoZero || !app.FlagLabelsListAll
	if client.Config() != nil && client.Config().HideZeroLabels && !app.FlagLabelsListAll {
		hideZero = true
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].FullName < items[j].FullName
	})

	for _, item := range items {
		if (hideZero || isNoZero) && item.MessagesTotal == 0 {
			continue
		}
		if isExt {
			fmt.Printf("%s (%d/%d)\n", item.FullName, item.MessagesUnread, item.MessagesTotal)
		} else {
			fmt.Println(item.FullName)
		}
	}
	return nil
}

func getLabelItemsReal(client mailclient.MailClient) ([]cfg_acc.LabelItem, error) {
	if cc, ok := client.(*mailclient.CheckingMailClient); ok {
		items, err := cc.Delegate.GetLabelItems()
		if err != nil {
			return nil, fmt.Errorf("server error fetching labels: %w", err)
		}
		return items, nil
	}
	items, err := client.GetLabelItems()
	if err != nil {
		return nil, fmt.Errorf("server error fetching labels: %w", err)
	}
	return items, nil
}

func findArg(args []string, target string) bool {
	for _, a := range args {
		if strings.EqualFold(a, target) {
			return true
		}
	}
	return false
}
