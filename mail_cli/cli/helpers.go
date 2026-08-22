package cli

import (
	"fmt"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

// AddToList adds an email to a list (whitelist or blacklist).
func AddToList(targetAcc *cfg_acc.AccountConfig, field, email string) {
	switch field {
	case "whitelist":
		targetAcc.Whitelist = append(targetAcc.Whitelist, email)
		fmt.Printf("%s Added '%s' to whitelist for account %s.\n", app.PrefixSuccess, email, targetAcc.Name)
	case "blacklist":
		targetAcc.Blacklist = append(targetAcc.Blacklist, email)
		fmt.Printf("%s Added '%s' to blacklist for account %s.\n", app.PrefixSuccess, email, targetAcc.Name)
	}
}

// ListEntries lists entries from a list (whitelist or blacklist).
func ListEntries(targetAcc *cfg_acc.AccountConfig, field string) {
	var entries []string
	switch field {
	case "whitelist":
		entries = targetAcc.Whitelist
		fmt.Printf("Whitelist for account %s:\n", targetAcc.Name)
	case "blacklist":
		entries = targetAcc.Blacklist
		fmt.Printf("Blacklist for account %s:\n", targetAcc.Name)
	}
	if len(entries) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, e := range entries {
		fmt.Printf("  - %s\n", e)
	}
}

// RemoveFromList removes an email from a list (whitelist or blacklist).
func RemoveFromList(targetAcc *cfg_acc.AccountConfig, field, email string) {
	var entries *[]string
	var name string
	switch field {
	case "whitelist":
		entries = &targetAcc.Whitelist
		name = "whitelist"
	case "blacklist":
		entries = &targetAcc.Blacklist
		name = "blacklist"
	default:
		return
	}
	for i, e := range *entries {
		if strings.EqualFold(strings.TrimSpace(e), strings.TrimSpace(email)) {
			*entries = append((*entries)[:i], (*entries)[i+1:]...)
			fmt.Printf("%s Removed '%s' from %s for account %s.\n", app.PrefixSuccess, email, name, targetAcc.Name)
			return
		}
	}
	fmt.Printf("%s Email '%s' is not in the %s.\n", app.PrefixWarn, email, name)
}

// Error helpers for consistent error messages

// ErrClientNotConfigured returns a standardized error for missing client configuration.
func ErrClientNotConfigured() error {
	return fmt.Errorf("client not configured")
}

// GetValidatedClient is a helper that gets a client and validates it in one call.
// Replaces the 5-line boilerplate pattern repeated across CLI handlers.
func GetValidatedClient(session *app.Session) (mailclient.MailClient, error) {
	if session.GetClient == nil {
		return nil, ErrClientNotConfigured()
	}
	client, err := session.GetClient(session.Config)
	if err != nil {
		return nil, err
	}
	if err := client.Validate(); err != nil {
		return nil, err
	}
	return client, nil
}

// ErrAccountNotFound returns a standardized error for missing account.
func ErrAccountNotFound(name string) error {
	return fmt.Errorf("account %q not found in config", name)
}

// ErrAccountNotFoundAlt returns an alternative error message for missing account.
func ErrAccountNotFoundAlt(name string) error {
	return fmt.Errorf("account %q not found", name)
}

// ErrAccountExists returns a standardized error for duplicate account.
func ErrAccountExists(name string) error {
	return fmt.Errorf("an account named %q already exists", name)
}

// ErrInvalidAccountType returns a standardized error for invalid account type.
func ErrInvalidAccountType(acctType string) error {
	return fmt.Errorf("unknown account type %q (expected 'jmap', 'gmail', or 'outlook')", acctType)
}

// ErrInvalidAccountMode returns a standardized error for invalid account mode.
func ErrInvalidAccountMode(mode string) error {
	return fmt.Errorf("invalid account type %q (must be 'regular' or 'test')", mode)
}

// ErrConfigNotConfigured returns a standardized error for missing config.
func ErrConfigNotConfigured(feature string) error {
	return fmt.Errorf("%s not configured", feature)
}

// ErrInvalidArgs returns a standardized error for invalid arguments.
func ErrInvalidArgs(cmd, expected string) error {
	return fmt.Errorf("%w: too many arguments for '%s' (expected: %s)", app.ErrUsage, cmd, expected)
}

// SanitizeLabelCache caches sanitized label names to avoid repeated computation.
// Use this when the same label is used multiple times in a single operation.
type SanitizeLabelCache map[string]string

// Get returns the sanitized label, computing it if not already cached.
func (c SanitizeLabelCache) Get(label string) string {
	if c == nil {
		return cfg_g.SanitizeLabelForCache(label)
	}
	sanitized, ok := c[label]
	if !ok {
		sanitized = cfg_g.SanitizeLabelForCache(label)
		c[label] = sanitized
	}
	return sanitized
}

// NewSanitizeLabelCache creates a new cache for sanitized labels.
func NewSanitizeLabelCache() SanitizeLabelCache {
	return make(SanitizeLabelCache)
}
