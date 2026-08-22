package cfg_acc

import (
	"strings"

	"mail_cli/label"
)

// AccountConfig stores configuration for an individual mail account.
type AccountConfig struct {
	Name            string   `json:"name"`
	DisplayName     string   `json:"display_name,omitempty"`
	Type            string   `json:"type"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	IMAPHost        string   `json:"imap_host,omitempty"`
	SessionURL      string   `json:"jmap_session_url,omitempty"`
	SpamFolder      string   `json:"spam_folder,omitempty"`
	ReceivedFolder  string   `json:"received_folder,omitempty"`
	SpamLearn       string   `json:"spam_learn,omitempty"`
	UnspamLearn     string   `json:"unspam_learn,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	Whitelist       []string `json:"whitelist,omitempty"`
	Blacklist       []string `json:"blacklist,omitempty"`
	Rules           []Rule   `json:"rules,omitempty"`
	CalendarManager bool     `json:"calendar_manager,omitempty"`
	AccountType     string   `json:"account_type,omitempty"`
	ReadOnly        bool     `json:"read_only,omitempty"`
}

// GetDisplayName returns the display name if configured, otherwise falls back to the internal name.
func (acc AccountConfig) GetDisplayName() string {
	if acc.DisplayName != "" {
		return acc.DisplayName
	}
	return acc.Name
}

// LabelItem is an alias for label.LabelItem.
type LabelItem = label.LabelItem

// MessageFolderRef is an alias for label.MessageFolderRef.
type MessageFolderRef = label.MessageFolderRef

// Rule defines a custom labeling and routing rule.
type Rule struct {
	Sender   string `json:"sender,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Label    string `json:"label"`
	Exported bool   `json:"exported"`
	Internal bool   `json:"internal,omitempty"`
}

// MatchRules searches a slice of rules for a match against the given sender and subject,
// returning a pointer to the matching rule or nil if none match.
func MatchRules(rules []Rule, sender, subject string) *Rule {
	cleanSender := strings.ToLower(strings.TrimSpace(sender))
	cleanSubject := strings.ToLower(strings.TrimSpace(subject))

	for i := range rules {
		r := &rules[i]
		if r.Subject != "" {
			if strings.HasPrefix(cleanSubject, strings.ToLower(strings.TrimSpace(r.Subject))) {
				return r
			}
		} else if r.Sender != "" {
			cleanRuleSender := strings.ToLower(strings.TrimSpace(r.Sender))
			if strings.EqualFold(cleanSender, cleanRuleSender) {
				return r
			}
			domain := cleanRuleSender
			domain = strings.TrimPrefix(domain, "*")
			if strings.HasPrefix(domain, "@") {
				if strings.HasSuffix(cleanSender, domain) {
					return r
				}
			} else if domain != "" && !strings.Contains(domain, "@") {
				if strings.HasSuffix(cleanSender, "@"+domain) || strings.HasSuffix(cleanSender, "."+domain) {
					return r
				}
			}
		}
	}
	return nil
}
