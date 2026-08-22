package app

import (
	"fmt"
	"strings"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

type Session struct {
	Config      *cfg_g.Config
	FileConfig  *cfg_g.FileConfig
	Account     *cfg_acc.AccountConfig
	ConfigPath  string
	DownloadDir string
	MailClient  mailclient.MailClient

	// Deprecated: Keep for backward compatibility
	GetClient                func(config *cfg_g.Config) (mailclient.MailClient, error)
	GetClientForAccountIndex func(config *cfg_g.Config, index int) (mailclient.MailClient, error)
	PreCheck                 func(config *cfg_g.Config) error
	RunScan                  func(config *cfg_g.Config, labelPrefix, moveSpam, moveInbox string) (int, error)
	RunShow                  func(config *cfg_g.Config, labelPrefix, msgID string) error
	RunUnspam                func(config *cfg_g.Config, id string) error
	RunUnspamFolder          func(config *cfg_g.Config, folder string) error
	RunLearningReset         func(config *cfg_g.Config) error
	RunTests                 func(config *cfg_g.Config) error
	ExportRules              func(config *cfg_g.Config, filePath string) error
	ImportRules              func(config *cfg_g.Config, filePath string) error
	ExportSieve              func(config *cfg_g.Config, outputPath string) error
	MarkSpam                 func(config *cfg_g.Config, id string) error
	LearnHam                 func(config *cfg_g.Config, folder string, force bool) error
	ArchiveAll               func(config *cfg_g.Config, client mailclient.MailClient, sourcePrefix, targetFolder string) error
	ArchiveByID              func(config *cfg_g.Config, client mailclient.MailClient, targetFolder, id string) error
	ResolveArchiveTarget     func(client mailclient.MailClient) (string, error)
	ResolveTrashTarget       func(client mailclient.MailClient) (string, error)
	CalendarAdd              func(config *cfg_g.Config, client mailclient.MailClient, labelPrefix, msgID string) error
	CalendarWeek             func(config *cfg_g.Config) error
	CalAddAll                func(config *cfg_g.Config, client mailclient.MailClient) error
	ConfigShow               func(config *cfg_g.Config) error
	ConfigValidate           func(config *cfg_g.Config) error
	ConfigSet                func(config *cfg_g.Config, key, value string, accountSpecific bool) error
	ConfigReset              func(config *cfg_g.Config, key string) error
	InitTUI                  func(config *cfg_g.Config, labelPrefix string) error
	RunLast                  func(config *cfg_g.Config, n int) error
}

// ResolveClientAndLabel resolves the MailClient and clean label name from a raw label spec string
// (e.g. "1:folder", "GMail:folder", or "folder").
func (s *Session) ResolveClientAndLabel(rawSpec string) (mailclient.MailClient, string, error) {
	spec, err := cfg_g.ParseAccountLabelSpec(rawSpec)
	if err != nil {
		return nil, "", err
	}
	if spec.AccountName != "" {
		if s.GetClient != nil {
			oldSelected := s.Config.SelectedAccount
			s.Config.SelectedAccount = spec.AccountName
			client, err := s.GetClient(s.Config)
			s.Config.SelectedAccount = oldSelected
			if err != nil {
				return nil, "", err
			}
			return client, spec.Label, nil
		}
		return nil, "", fmt.Errorf("account %q not found", spec.AccountName)
	}
	if s.GetClient == nil {
		return nil, "", fmt.Errorf("client not configured")
	}
	client, err := s.GetClient(s.Config)
	if err != nil {
		return nil, "", err
	}
	return client, spec.Label, nil
}

// FormatAccountLabel formats a label string with its associated account display name indicator (e.g. "[GMail] INBOX").
func FormatAccountLabel(session *Session, client mailclient.MailClient, label string) string {
	if session == nil || session.Config == nil {
		return label
	}
	accName := ""
	if client != nil && client.Config() != nil {
		accName = client.Config().SelectedAccount
	}
	if accName == "" {
		accName = session.Config.SelectedAccount
	}
	for _, a := range session.Config.Accounts {
		if strings.EqualFold(a.Name, accName) || strings.EqualFold(a.GetDisplayName(), accName) {
			displayName := a.GetDisplayName()
			if displayName != "" {
				return fmt.Sprintf("[%s] %s", displayName, label)
			}
		}
	}
	if accName == "" || strings.EqualFold(accName, "default") {
		if _, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(session.Config); err == nil && targetAcc != nil {
			displayName := targetAcc.GetDisplayName()
			if displayName != "" {
				return fmt.Sprintf("[%s] %s", displayName, label)
			}
		} else if len(session.Config.Accounts) > 0 {
			displayName := session.Config.Accounts[0].GetDisplayName()
			if displayName != "" {
				return fmt.Sprintf("[%s] %s", displayName, label)
			}
		}
	}
	if accName != "" {
		return fmt.Sprintf("[%s] %s", accName, label)
	}
	return label
}
