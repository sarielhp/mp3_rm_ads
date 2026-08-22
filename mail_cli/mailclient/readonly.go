package mailclient

import (
	"errors"
	"log/slog"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

var ErrReadOnlyOperationBlocked = errors.New("operation blocked: account is in read-only mode")

// ReadOnlyMailClient decorates a MailClient to prevent any remote mutations.
type ReadOnlyMailClient struct {
	Delegate    MailClient
	AccountName string
}

func NewReadOnlyMailClient(delegate MailClient, accountName string) *ReadOnlyMailClient {
	return &ReadOnlyMailClient{
		Delegate:    delegate,
		AccountName: accountName,
	}
}

func (r *ReadOnlyMailClient) Validate() error {
	return r.Delegate.Validate()
}

func (r *ReadOnlyMailClient) GetMatchingLabels(prefix string) ([]string, error) {
	return r.Delegate.GetMatchingLabels(prefix)
}

func (r *ReadOnlyMailClient) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	return r.Delegate.GetLabelItems()
}

func (r *ReadOnlyMailClient) ListLabels() error {
	return r.Delegate.ListLabels()
}

func (r *ReadOnlyMailClient) InboxFolder() string {
	return r.Delegate.InboxFolder()
}

func (r *ReadOnlyMailClient) RenameLabel(oldName, newName string) error {
	slog.Info("[READ-ONLY] Simulated RenameLabel", slog.String("account", r.AccountName), slog.String("old", oldName), slog.String("new", newName))
	return nil
}

func (r *ReadOnlyMailClient) FixLabels() error {
	slog.Info("[READ-ONLY] Simulated FixLabels", slog.String("account", r.AccountName))
	return nil
}

func (r *ReadOnlyMailClient) DeleteLabel(name string) error {
	slog.Warn("[READ-ONLY] DeleteLabel blocked", slog.String("account", r.AccountName), slog.String("label", name))
	return ErrReadOnlyOperationBlocked
}

func (r *ReadOnlyMailClient) EnsureLabelExists(name string) error {
	slog.Info("[READ-ONLY] Simulated EnsureLabelExists", slog.String("account", r.AccountName), slog.String("label", name))
	return nil
}

func (r *ReadOnlyMailClient) FetchAndDownloadEmails(folderName string, cacheSubdir string) ([]string, error) {
	return r.Delegate.FetchAndDownloadEmails(folderName, cacheSubdir)
}

func (r *ReadOnlyMailClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	return r.Delegate.FetchLatestAccountEmails(limit)
}

func (r *ReadOnlyMailClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	slog.Info("[READ-ONLY] Simulated MoveEmail", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)), slog.String("from", sourceLabelName), slog.String("to", destLabelName))
	return nil
}

func (r *ReadOnlyMailClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	slog.Info("[READ-ONLY] Simulated CopyEmail", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)), slog.String("from", sourceLabelName), slog.String("to", destLabelName))
	return nil
}

func (r *ReadOnlyMailClient) MarkAsRead(messageIDs []string) error {
	slog.Debug("[READ-ONLY] Simulated MarkAsRead", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)))
	return nil
}

func (r *ReadOnlyMailClient) MoveToInbox(messageIDs []string, sourceLabelName string) error {
	slog.Info("[READ-ONLY] Simulated MoveToInbox", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)), slog.String("from", sourceLabelName))
	return nil
}

func (r *ReadOnlyMailClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	slog.Warn("[READ-ONLY] UploadRawEmail blocked", slog.String("account", r.AccountName))
	return ErrReadOnlyOperationBlocked
}

func (r *ReadOnlyMailClient) ReportSpam(messageIDs []string, sourceLabelName string) error {
	slog.Info("[READ-ONLY] Simulated ReportSpam", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)), slog.String("from", sourceLabelName))
	return nil
}

func (r *ReadOnlyMailClient) MoveAllSpam(destLabel string) error {
	slog.Info("[READ-ONLY] Simulated MoveAllSpam", slog.String("account", r.AccountName), slog.String("dest", destLabel))
	return nil
}

func (r *ReadOnlyMailClient) DeleteAllSpam() error {
	slog.Warn("[READ-ONLY] DeleteAllSpam blocked", slog.String("account", r.AccountName))
	return ErrReadOnlyOperationBlocked
}

func (r *ReadOnlyMailClient) ShowPoliticalSpam(autoBlacklist bool) error {
	return r.Delegate.ShowPoliticalSpam(false)
}

func (r *ReadOnlyMailClient) LearnSpam() error {
	slog.Info("[READ-ONLY] Simulated LearnSpam", slog.String("account", r.AccountName))
	return nil
}

func (r *ReadOnlyMailClient) CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error) {
	slog.Info("[READ-ONLY] Simulated CheckAndApplyRules", slog.String("account", r.AccountName), slog.Int("count", len(messageIDs)))
	return messageIDs, nil
}

func (r *ReadOnlyMailClient) ExportRules() error {
	return r.Delegate.ExportRules()
}

func (r *ReadOnlyMailClient) ListFilters() error {
	return r.Delegate.ListFilters()
}

func (r *ReadOnlyMailClient) RewriteRuleLabelCasing(label string) string {
	return r.Delegate.RewriteRuleLabelCasing(label)
}

func (r *ReadOnlyMailClient) Config() *cfg_g.Config {
	return r.Delegate.Config()
}

func (r *ReadOnlyMailClient) BackendType() string {
	return r.Delegate.BackendType()
}

func (r *ReadOnlyMailClient) SendEmail(rawBytes []byte) error {
	slog.Warn("[READ-ONLY] SendEmail blocked", slog.String("account", r.AccountName))
	return ErrReadOnlyOperationBlocked
}
