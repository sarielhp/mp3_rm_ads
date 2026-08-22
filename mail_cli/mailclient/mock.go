package mailclient

import (
	"strings"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

type MovedEmailRecord struct {
	MessageIDs  []string
	SourceLabel string
	DestLabel   string
}

type CopiedEmailRecord struct {
	MessageIDs  []string
	SourceLabel string
	DestLabel   string
}

type UploadedEmailRecord struct {
	RawBytes    []byte
	TargetLabel string
}

type MockMailClient struct {
	LabelItems        []cfg_acc.LabelItem
	Labels            []string
	Cfg               *cfg_g.Config
	DownloadedIDs     map[string][]string
	MovedEmails       []MovedEmailRecord
	CopiedEmails      []CopiedEmailRecord
	UploadedEmails    []UploadedEmailRecord
	AccountLatestRefs []cfg_acc.MessageFolderRef
	Err               error
	InboxFolderVal    string
	BackendTypeVal    string
}

func (m *MockMailClient) Validate() error { return m.Err }
func (m *MockMailClient) GetMatchingLabels(prefix string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if prefix == "" {
		return m.Labels, nil
	}
	var matched []string
	prefixLower := strings.ToLower(prefix)
	for _, l := range m.Labels {
		if strings.HasPrefix(strings.ToLower(l), prefixLower) {
			matched = append(matched, l)
		}
	}
	return matched, nil
}
func (m *MockMailClient) FetchAndDownloadEmails(folderName string, _ string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.DownloadedIDs == nil {
		return nil, nil
	}
	return m.DownloadedIDs[folderName], nil
}
func (m *MockMailClient) FetchLatestAccountEmails(limit int) ([]cfg_acc.MessageFolderRef, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if len(m.AccountLatestRefs) > 0 {
		refs := m.AccountLatestRefs
		if limit > 0 && len(refs) > limit {
			refs = refs[:limit]
		}
		return refs, nil
	}
	var refs []cfg_acc.MessageFolderRef
	for folder, ids := range m.DownloadedIDs {
		for _, id := range ids {
			refs = append(refs, cfg_acc.MessageFolderRef{
				MessageID: id,
				Folder:    folder,
			})
		}
	}
	if limit > 0 && len(refs) > limit {
		refs = refs[:limit]
	}
	return refs, nil
}
func (m *MockMailClient) CheckAndApplyRules(messageIDs []string, _ string, _ string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return messageIDs, nil
}
func (m *MockMailClient) ReportSpam(_ []string, _ string) error  { return m.Err }
func (m *MockMailClient) MoveToInbox(_ []string, _ string) error { return m.Err }
func (m *MockMailClient) MoveEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if m.Err != nil {
		return m.Err
	}
	m.MovedEmails = append(m.MovedEmails, MovedEmailRecord{
		MessageIDs:  messageIDs,
		SourceLabel: sourceLabelName,
		DestLabel:   destLabelName,
	})
	return nil
}
func (m *MockMailClient) CopyEmail(messageIDs []string, sourceLabelName string, destLabelName string) error {
	if m.Err != nil {
		return m.Err
	}
	m.CopiedEmails = append(m.CopiedEmails, CopiedEmailRecord{
		MessageIDs:  messageIDs,
		SourceLabel: sourceLabelName,
		DestLabel:   destLabelName,
	})
	if m.DownloadedIDs == nil {
		m.DownloadedIDs = make(map[string][]string)
	}
	m.DownloadedIDs[destLabelName] = append(m.DownloadedIDs[destLabelName], messageIDs...)
	return nil
}
func (m *MockMailClient) MoveAllSpam(_ string) error             { return m.Err }
func (m *MockMailClient) DeleteAllSpam() error                   { return m.Err }
func (m *MockMailClient) ShowPoliticalSpam(_ bool) error         { return m.Err }
func (m *MockMailClient) LearnSpam() error                       { return m.Err }
func (m *MockMailClient) ListLabels() error                      { return m.Err }
func (m *MockMailClient) RenameLabel(_, _ string) error          { return m.Err }
func (m *MockMailClient) FixLabels() error                       { return m.Err }
func (m *MockMailClient) DeleteLabel(_ string) error             { return m.Err }
func (m *MockMailClient) ExportRules() error                     { return m.Err }
func (m *MockMailClient) ListFilters() error                     { return m.Err }
func (m *MockMailClient) Config() *cfg_g.Config                  { return m.Cfg }
func (m *MockMailClient) RewriteRuleLabelCasing(_ string) string { return "" }
func (m *MockMailClient) EnsureLabelExists(_ string) error       { return m.Err }
func (m *MockMailClient) GetLabelItems() ([]cfg_acc.LabelItem, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.LabelItems, nil
}
func (m *MockMailClient) MarkAsRead(_ []string) error { return m.Err }
func (m *MockMailClient) UploadRawEmail(rawBytes []byte, targetLabel string) error {
	if m.Err != nil {
		return m.Err
	}
	m.UploadedEmails = append(m.UploadedEmails, UploadedEmailRecord{
		RawBytes:    rawBytes,
		TargetLabel: targetLabel,
	})
	if m.DownloadedIDs == nil {
		m.DownloadedIDs = make(map[string][]string)
	}
	m.DownloadedIDs[targetLabel] = append(m.DownloadedIDs[targetLabel], "msg101")
	return nil
}
func (m *MockMailClient) SendEmail(_ []byte) error { return m.Err }
func (m *MockMailClient) InboxFolder() string {
	if m.InboxFolderVal != "" {
		return m.InboxFolderVal
	}
	if m.Cfg != nil && m.Cfg.ReceivedFolder != "" {
		return m.Cfg.ReceivedFolder
	}
	return "Inbox"
}

func (m *MockMailClient) BackendType() string {
	if m.BackendTypeVal != "" {
		return m.BackendTypeVal
	}
	return "mock"
}
