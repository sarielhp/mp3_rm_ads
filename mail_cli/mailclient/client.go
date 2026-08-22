package mailclient

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

type CheckingMailClient struct {
	Delegate   MailClient
	AccName    string
	CacheStore cache.CacheStore
}

func (c *CheckingMailClient) GetCacheStore() cache.CacheStore {
	if c.CacheStore == nil {
		return &cache.NoOpCacheStore{}
	}
	return c.CacheStore
}

func (c *CheckingMailClient) IsInactive() bool {
	return !c.GetCacheStore().IsActive()
}

func (c *CheckingMailClient) Validate() error {
	if err := c.fixReceivedFolder(); err != nil {
		return err
	}
	return c.Delegate.Validate()
}

// fixReceivedFolder checks if received_folder is incorrectly set to "inbox"
// and auto-fixes it when FixConfig is enabled.
func (c *CheckingMailClient) fixReceivedFolder() error {
	config := c.Config()
	if config == nil {
		return nil
	}
	if !strings.EqualFold(config.ReceivedFolder, "inbox") {
		return nil
	}
	if config.FixConfig {
		fc, targetAcc, _, configPath, err := cfg_g.ResolveAccountFromConfig(config)
		if err != nil {
			return fmt.Errorf("failed to resolve account config for fixing: %w", err)
		}
		appropriate := "received"
		if strings.EqualFold(targetAcc.Type, "outlook") {
			appropriate = "Archive"
		}
		targetAcc.ReceivedFolder = appropriate
		if err := cfg_g.SaveConfigFile(configPath, fc); err != nil {
			return fmt.Errorf("failed to save config file after fix: %w", err)
		}
		config.ReceivedFolder = appropriate
		slog.Info("Automatically fixed received_folder in config", slog.String("account", targetAcc.Name), slog.String("fixed_to", appropriate))
		return nil
	}
	return fmt.Errorf("received_folder cannot be inbox")
}

func (c *CheckingMailClient) GetMatchingLabels(prefix string) ([]string, error) {
	return c.Delegate.GetMatchingLabels(prefix)
}

func (c *CheckingMailClient) FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error) {
	if c.IsInactive() {
		return nil, nil
	}
	ids, err := c.Delegate.FetchAndDownloadEmails(folderName, cacheSubdir)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if config := c.Config(); config != nil {
			if err := label.ReplaceAll(config.DownloadDir, folderName, nil); err != nil {
				slog.Error("CheckingMailClient.FetchAndDownloadEmails: ReplaceAll failed", slog.String("folder", folderName), slog.Any("error", err))
			}
		}
	}
	return ids, nil
}

func (c *CheckingMailClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	if c.IsInactive() {
		return nil, nil
	}
	return c.Delegate.FetchLatestAccountEmails(limit)
}

func (c *CheckingMailClient) CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error) {
	if c.IsInactive() {
		return nil, nil
	}
	return c.Delegate.CheckAndApplyRules(messageIDs, sourceLabelName, cacheSubdir)
}

func (c *CheckingMailClient) ReportSpam(messageIDs []string, sourceLabelName string) error {
	return c.Delegate.ReportSpam(messageIDs, sourceLabelName)
}

func (c *CheckingMailClient) MoveToInbox(messageIDs []string, sourceLabelName string) error {
	return c.Delegate.MoveToInbox(messageIDs, sourceLabelName)
}

func (c *CheckingMailClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	return c.Delegate.MoveEmail(messageIDs, sourceLabelName, destLabelName)
}

func (c *CheckingMailClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	return c.Delegate.CopyEmail(messageIDs, sourceLabelName, destLabelName)
}

func (c *CheckingMailClient) MoveAllSpam(destLabel string) error {
	return c.Delegate.MoveAllSpam(destLabel)
}

func (c *CheckingMailClient) DeleteAllSpam() error {
	return c.Delegate.DeleteAllSpam()
}

func (c *CheckingMailClient) ShowPoliticalSpam(autoBlacklist bool) error {
	return c.Delegate.ShowPoliticalSpam(autoBlacklist)
}

func (c *CheckingMailClient) LearnSpam() error {
	return c.Delegate.LearnSpam()
}

func (c *CheckingMailClient) ListLabels() error {
	return c.Delegate.ListLabels()
}

func (c *CheckingMailClient) RenameLabel(oldName, newName string) error {
	return c.Delegate.RenameLabel(oldName, newName)
}

func (c *CheckingMailClient) FixLabels() error {
	return c.Delegate.FixLabels()
}

func (c *CheckingMailClient) DeleteLabel(name string) error {
	return c.Delegate.DeleteLabel(name)
}

func (c *CheckingMailClient) ExportRules() error {
	return c.Delegate.ExportRules()
}

func (c *CheckingMailClient) ListFilters() error {
	return c.Delegate.ListFilters()
}

func (c *CheckingMailClient) Config() *cfg_g.Config {
	return c.Delegate.Config()
}

func (c *CheckingMailClient) RewriteRuleLabelCasing(label string) string {
	return c.Delegate.RewriteRuleLabelCasing(label)
}

func (c *CheckingMailClient) EnsureLabelExists(name string) error {
	if c.IsInactive() {
		return nil
	}
	return c.Delegate.EnsureLabelExists(name)
}

const maxLabelCacheAge = 24 * time.Hour

func (c *CheckingMailClient) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	items, err := c.Delegate.GetLabelItems()
	if err == nil {
		slog.Info("GetLabelItems saving cache",
			slog.String("accName", c.AccName),
			slog.Int("items", len(items)))
		_ = c.GetCacheStore().SaveLabelItems(items)
		return items, nil
	}

	slog.Warn("GetLabelItems: server fetch failed, checking cache", slog.Any("error", err))

	if age, ageErr := c.GetCacheStore().CachedLabelItemsAge(); ageErr == nil {
		if age > maxLabelCacheAge {
			slog.Warn("GetLabelItems: cache too old, not using fallback", slog.Duration("age", age))
			return nil, fmt.Errorf("label fetch failed and cache is %v old (max %v): %w", age, maxLabelCacheAge, err)
		}
		slog.Info("GetLabelItems: using cached data", slog.Duration("age", age))
	}

	if cachedItems, errRead := c.GetCacheStore().GetLabelItems(); errRead == nil {
		slog.Warn("GetLabelItems: using cached data (server fetch failed)", slog.Any("error", err))
		return cachedItems, nil
	}

	return nil, err
}

func (c *CheckingMailClient) MarkAsRead(messageIDs []string) error {
	return c.Delegate.MarkAsRead(messageIDs)
}

func (c *CheckingMailClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	return c.Delegate.UploadRawEmail(rawBytes, targetLabel)
}

func (c *CheckingMailClient) SendEmail(rawBytes []byte) error {
	return c.Delegate.SendEmail(rawBytes)
}

func (c *CheckingMailClient) InboxFolder() string {
	return c.Delegate.InboxFolder()
}

func (c *CheckingMailClient) BackendType() string { return c.Delegate.BackendType() }
