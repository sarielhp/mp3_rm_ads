package outlook

import (
	"fmt"
	"net/http"
	"strings"

	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/mailclient"
)

type OutlookClient struct {
	config  *cfg_g.Config
	account cfg_acc.AccountConfig
	client  *http.Client
}

func NewOutlookClient(acc cfg_acc.AccountConfig, config *cfg_g.Config) mailclient.MailClient {
	return &OutlookClient{
		config:  config,
		account: acc,
	}
}

func (c *OutlookClient) init() error {
	if c.client != nil {
		return nil
	}
	c.client = GetOutlookClient(c.config, c.account.Name)
	return nil
}

func (c *OutlookClient) Validate() error {
	if err := cfg_g.ValidateAccountParams(c.account); err != nil {
		return err
	}
	if err := c.init(); err != nil {
		return err
	}

	resp, err := c.client.Get("https://graph.microsoft.com/v1.0/me/mailFolders")
	if err != nil {
		return fmt.Errorf("outlook validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("outlook API returned status code %d", resp.StatusCode)
	}

	return nil
}

func (c *OutlookClient) Config() *cfg_g.Config {
	return c.config
}

func (c *OutlookClient) InboxFolder() string {
	return "Inbox"
}

func (c *OutlookClient) RewriteRuleLabelCasing(label string) string {
	if strings.EqualFold(label, "inbox") {
		return "Inbox"
	}
	return label
}

func (c *OutlookClient) BackendType() string { return "outlook" }
