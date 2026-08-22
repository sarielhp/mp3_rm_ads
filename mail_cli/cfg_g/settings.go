package cfg_g

import "mail_cli/cfg_acc"

// AppSettings is the subset of Config fields that backends actually consume.
// Backend constructors receive this alongside AccountConfig instead of a full *Config.
type AppSettings struct {
	DownloadDir     string
	Verbose         bool
	Limit           int
	Rules           []cfg_acc.Rule
	Whitelist       []string
	Blacklist       []string
	SpamFolder      string
	UnspamLearn     string
	SelectedAccount string
	AccountType     string
}
