package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"mail_cli/backend/gmail"
	"mail_cli/backend/jmap"
	"mail_cli/backend/outlook"
	"mail_cli/cache"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

type MailClient = mailclient.MailClient

func NewMailClient(acc AccountConfig, config *Config) (MailClient, error) {
	localCfg := *config
	if acc.Username != "" {
		localCfg.Username = acc.Username
	}
	if acc.Password != "" {
		localCfg.Password = acc.Password
	}
	if acc.IMAPHost != "" {
		localCfg.IMAPHost = acc.IMAPHost
	}
	if acc.SpamFolder != "" {
		localCfg.SpamFolder = acc.SpamFolder
	}
	if acc.ReceivedFolder != "" {
		localCfg.ReceivedFolder = acc.ReceivedFolder
	}
	localCfg.SelectedAccount = acc.Name
	localCfg.AccountType = acc.AccountType
	if localCfg.AccountType == "" {
		localCfg.AccountType = "regular"
	}
	if acc.Rules != nil {
		localCfg.Rules = acc.Rules
	}
	if acc.Whitelist != nil {
		localCfg.Whitelist = acc.Whitelist
	}
	if acc.Blacklist != nil {
		localCfg.Blacklist = acc.Blacklist
	}
	accountDir := cfg_g.SanitizeLabelForCache(acc.Name)
	if accountDir == "" {
		accountDir = "default"
	}
	localCfg.DownloadDir = filepath.Join(config.DownloadDir, accountDir)

	if acc.Rules == nil {
		localCfg.Rules = []Rule{}
	}
	if acc.Whitelist == nil {
		localCfg.Whitelist = []string{}
	}
	if acc.Blacklist == nil {
		localCfg.Blacklist = []string{}
	}

	var client MailClient
	switch strings.ToLower(acc.Type) {
	case "gmail":
		client = gmail.NewGmailClient(acc, &localCfg)
	case "jmap":
		client = jmap.NewJMAPClient(acc, &localCfg)
	case "outlook":
		client = outlook.NewOutlookClient(acc, &localCfg)
	default:
		return nil, fmt.Errorf("unsupported account type: %q", acc.Type)
	}

	if localCfg.ReadOnly || acc.ReadOnly {
		client = mailclient.NewReadOnlyMailClient(client, acc.Name)
	}

	var isSel bool
	if config.SelectedAccount != "" {
		isSel = strings.EqualFold(acc.Name, config.SelectedAccount)
	} else {
		isSel = len(config.Accounts) > 0 && strings.EqualFold(acc.Name, config.Accounts[0].Name)
	}

	var cs cache.CacheStore
	if isSel || config.SelectedAccount == "" {
		cs = &cache.DiskCacheStore{DownloadDir: localCfg.DownloadDir}
	} else {
		cs = &cache.NoOpCacheStore{}
	}

	return &mailclient.CheckingMailClient{
		Delegate:   client,
		AccName:    acc.Name,
		CacheStore: cs,
	}, nil
}
