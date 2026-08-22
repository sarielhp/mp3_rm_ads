package cfg_g

import "mail_cli/cfg_acc"

// Config stores all configuration parameters for the application.
type Config struct {
	IMAPHost         string
	Username         string
	Password         string
	DownloadDir      string
	ConfigDir        string
	Limit            int
	ScoreThreshold   float64
	LearnSpam        bool
	ForceLearn       bool
	SpamFolder       string
	ReceivedFolder   string
	SpamLearn        string
	UnspamLearn      string
	AllowedLanguages []string
	BlockPolitical   bool
	ShowPolitical    bool
	DeleteAllSpam    bool
	AutoUnsubscribe  bool
	Verbose          bool
	WListCmd         string
	WListVal         string
	Whitelist        []string
	BListCmd         string
	BListVal         string
	Blacklist        []string
	Rules            []cfg_acc.Rule
	RuleCmd          string
	RuleValEmail     string
	RuleValSubject   string
	RuleValLabel     string
	LabelsCmd        string
	LabelsValOld     string
	LabelsValNew     string
	LabelsValDel     string
	HideZeroLabels   bool
	FilterCmd        string
	Accounts         []cfg_acc.AccountConfig
	SelectedAccount  string
	AccountType      string
	Browser          string
	Quiet            bool
	FixConfig        bool
	Editor           string
	EditorArgs       []string
	ReadOnly         bool
}

// FileConfig defines the fields structured inside ~/.config/mail_cli/config.json
type FileConfig struct {
	Username         string                   `json:"username"`
	Password         string                   `json:"password"`
	IMAPHost         string                   `json:"imap_host"`
	DownloadDir      string                   `json:"download_dir"`
	Limit            *int                     `json:"limit"`
	ScoreThreshold   *float64                 `json:"score_threshold"`
	SpamFolder       string                   `json:"spam_folder"`
	ReceivedFolder   string                   `json:"received_folder"`
	SpamLearn        string                   `json:"spam_learn,omitempty"`
	UnspamLearn      string                   `json:"unspam_learn,omitempty"`
	AllowedLanguages *[]string                `json:"allowed_languages"`
	BlockPolitical   *bool                    `json:"block_political"`
	AutoUnsubscribe  *bool                    `json:"auto_unsubscribe"`
	Whitelist        *[]string                `json:"whitelist,omitempty"`
	Blacklist        *[]string                `json:"blacklist,omitempty"`
	Rules            *[]cfg_acc.Rule          `json:"rules,omitempty"`
	Accounts         *[]cfg_acc.AccountConfig `json:"accounts"`
	Browser          string                   `json:"browser,omitempty"`
	Editor           string                   `json:"editor,omitempty"`
	EditorArgs       []string                 `json:"editor_args,omitempty"`
	ReadOnly         *bool                    `json:"read_only,omitempty"`
}

// GetSelectedAccountDisplayName returns the display name of the selected account, or SelectedAccount if not found.
func (c *Config) GetSelectedAccountDisplayName() string {
	for _, acc := range c.Accounts {
		if acc.Name == c.SelectedAccount {
			return acc.GetDisplayName()
		}
	}
	return c.SelectedAccount
}
