package mailclient

import (
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

// Role interfaces — consumers depend on the smallest relevant one.

type LabelReader interface {
	GetMatchingLabels(prefix string) ([]string, error)
	GetLabelItems() ([]cfg_acc.LabelItem, error)
	ListLabels() error
	InboxFolder() string
}

type LabelWriter interface {
	RenameLabel(oldName, newName string) error
	FixLabels() error
	DeleteLabel(name string) error
	EnsureLabelExists(name string) error
}

type EmailFetcher interface {
	FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error)
	FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error)
}

type EmailWriter interface {
	MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error
	CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error
	MarkAsRead(messageIDs []string) error
	MoveToInbox(messageIDs []string, sourceLabelName string) error
	UploadRawEmail(rawBytes []byte, targetLabel string) error
}

type SpamManager interface {
	ReportSpam(messageIDs []string, sourceLabelName string) error
	MoveAllSpam(destLabel string) error
	DeleteAllSpam() error
	ShowPoliticalSpam(autoBlacklist bool) error
	LearnSpam() error
}

type RuleManager interface {
	CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error)
	ExportRules() error
	ListFilters() error
	RewriteRuleLabelCasing(label string) string
}

// ConfigProvider gives access to the global config.
type ConfigProvider interface {
	Config() *cfg_g.Config
}

// BackendInfo exposes backend identity.
type BackendInfo interface {
	BackendType() string
}

// MailClient is the full union interface for all backend operations.
// Consumers should prefer the narrower role interfaces above.
type MailClient interface {
	Validate() error
	LabelReader
	LabelWriter
	EmailFetcher
	EmailWriter
	SpamManager
	RuleManager
	ConfigProvider
	BackendInfo
	SendEmail(rawBytes []byte) error
}
