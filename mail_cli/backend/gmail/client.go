package gmail

import (
	"fmt"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"sort"
	"strings"
)

// GmailClient implements the MailClient interface using the Gmail REST API wrapper.
type GmailClient struct {
	config  *Config
	account AccountConfig
}

func NewGmailClient(acc AccountConfig, config *Config) *GmailClient {
	return &GmailClient{config: config, account: acc}
}

func (c *GmailClient) Validate() error {
	if err := cfg_g.ValidateAccountParams(c.account); err != nil {
		return err
	}
	srv, err := GetGmailService(c.config)
	if err != nil {
		return err
	}
	if _, err := srv.Users.Labels.List("me").Do(); err != nil {
		return err
	}
	return nil
}

func (c *GmailClient) GetMatchingLabels(prefix string) ([]string, error) {
	return GetMatchingLabels(c.config, prefix)
}

func (c *GmailClient) FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error) {
	return fetchAndDownloadEmailsREST(c.config, folderName, cacheSubdir)
}

func (c *GmailClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	return fetchLatestAccountEmailsREST(c.config, limit)
}

func (c *GmailClient) CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error) {
	return checkAndApplyRulesREST(messageIDs, c.config, sourceLabelName, cacheSubdir)
}

func (c *GmailClient) ReportSpam(messageIDs []string, sourceLabelName string) error {
	return reportSpamToGmailREST(messageIDs, c.config, sourceLabelName)
}

func (c *GmailClient) MoveToInbox(messageIDs []string, sourceLabelName string) error {
	return moveToInboxGmailREST(messageIDs, c.config, sourceLabelName)
}

func (c *GmailClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	return moveEmailGmailREST(messageIDs, c.config, sourceLabelName, destLabelName)
}

func (c *GmailClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	return copyEmailGmailREST(c.config, messageIDs, sourceLabelName, destLabelName)
}

func (c *GmailClient) MoveAllSpam(destLabel string) error {
	return moveAllSpamInGmailREST(c.config, destLabel)
}

func (c *GmailClient) DeleteAllSpam() error {
	return deleteAllSpamInGmailREST(c.config)
}

func (c *GmailClient) ShowPoliticalSpam(autoBlacklist bool) error {
	return showPoliticalSpamInGmailREST(c.config, autoBlacklist)
}

func (c *GmailClient) LearnSpam() error {
	return learnSpamFromGmailREST(c.config)
}

func (c *GmailClient) ListLabels() error {
	return listGmailLabelsREST(c.config)
}

func (c *GmailClient) GetLabelItems() ([]LabelItem, error) {
	return getLabelItemsGmailREST(c.config)
}

func (c *GmailClient) MarkAsRead(messageIDs []string) error {
	return markAsReadGmailREST(c.config, messageIDs)
}

func (c *GmailClient) RenameLabel(oldName, newName string) error {
	localCfg := *c.config
	localCfg.LabelsValOld = oldName
	localCfg.LabelsValNew = newName
	return renameGmailLabelREST(&localCfg)
}

func (c *GmailClient) FixLabels() error {
	return fixGmailLabelsREST(c.config)
}

func (c *GmailClient) DeleteLabel(name string) error {
	localCfg := *c.config
	localCfg.LabelsValDel = name
	return deleteGmailLabelREST(&localCfg)
}

func (c *GmailClient) ExportRules() error {
	return exportRulesToGmailREST(c.config)
}

func (c *GmailClient) ListFilters() error {
	return listGmailFiltersREST(c.config)
}

func (c *GmailClient) Config() *Config {
	return c.config
}

func (c *GmailClient) RewriteRuleLabelCasing(label string) string {
	return rewriteRuleLabelCasing(c.config, label)
}

func (c *GmailClient) InboxFolder() string {
	return "INBOX"
}

func (c *GmailClient) BackendType() string { return "gmail" }

// GetMatchingLabels fetches Gmail labels matching a prefix.
func GetMatchingLabels(config *Config, prefix string) ([]string, error) {
	srv, err := GetGmailService(config)
	if err != nil {
		return nil, err
	}

	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list Gmail labels: %w", err)
	}

	var matched []string
	prefixLower := strings.ToLower(prefix)
	for _, l := range labelsRes.Labels {
		if strings.HasPrefix(strings.ToLower(l.Name), prefixLower) {
			matched = append(matched, l.Name)
		}
	}

	if len(matched) == 0 {
		switch prefixLower {
		case "inbox", "[gmail]/inbox", "[google mail]/inbox":
			matched = append(matched, "INBOX")
		case "spam", "[gmail]/spam", "[google mail]/spam":
			matched = append(matched, "SPAM")
		case "sent", "sent mail", "[gmail]/sent mail", "[gmail]/sent", "[google mail]/sent mail":
			matched = append(matched, "SENT")
		case "trash", "bin", "[gmail]/trash", "[gmail]/bin", "[google mail]/trash":
			matched = append(matched, "TRASH")
		case "drafts", "draft", "[gmail]/drafts", "[google mail]/drafts":
			matched = append(matched, "DRAFT")
		}
	}

	sort.Strings(matched)
	return matched, nil
}
