package outlook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	myme "mail_cli/email"
)

type graphRule struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Sequence    int    `json:"sequence"`
	IsEnabled   bool   `json:"isEnabled"`
	Conditions  struct {
		SenderContains  []string `json:"senderContains,omitempty"`
		SubjectContains []string `json:"subjectContains,omitempty"`
	} `json:"conditions"`
	Actions struct {
		MoveToFolder string `json:"moveToFolder,omitempty"`
	} `json:"actions"`
}

func (c *OutlookClient) CheckAndApplyRules(messageIDs []string, sourceLabelName string, cacheSubdir string) ([]string, error) {
	if len(messageIDs) == 0 {
		return messageIDs, nil
	}
	if err := c.init(); err != nil {
		return nil, err
	}

	var remainingIDs []string
	dec := new(mime.WordDecoder)

	for _, id := range messageIDs {
		data, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			remainingIDs = append(remainingIDs, id)
			continue
		}

		localEmail, errParse := mail.ReadMessage(bytes.NewReader(data))
		if errParse != nil {
			remainingIDs = append(remainingIDs, id)
			continue
		}

		fromHeader := localEmail.Header.Get("From")
		sender := myme.ParseEmailAddress(fromHeader)
		subject := myme.DecodeHeader(dec, localEmail.Header.Get("Subject"))

		if matchedRule := cfg_acc.MatchRules(c.config.Rules, sender, subject); matchedRule != nil {
			if c.config.Verbose {
				fmt.Printf("%s Outlook Rule match: moving %s to %q\n", app.PrefixInfo, id, matchedRule.Label)
			}
			errMove := c.MoveEmail([]string{id}, sourceLabelName, matchedRule.Label)
			if errMove != nil {
				remainingIDs = append(remainingIDs, id)
			}
		} else {
			remainingIDs = append(remainingIDs, id)
		}
	}

	return remainingIDs, nil
}

func (c *OutlookClient) ListFilters() error {
	if err := c.init(); err != nil {
		return err
	}

	resp, err := c.client.Get("https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messagerules")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list filters failed: status %d: %s", resp.StatusCode, string(b))
	}

	var res struct {
		Value []graphRule `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	app.ColorBoldCyan.Println("\n======================================================================")
	app.ColorBoldCyan.Println("                        REMOTE OUTLOOK FILTERS                        ")
	app.ColorBoldCyan.Println("======================================================================")

	if len(res.Value) == 0 {
		fmt.Println("  No remote filters found on Outlook.")
	} else {
		for i, r := range res.Value {
			status := "disabled"
			if r.IsEnabled {
				status = "enabled"
			}
			var conds []string
			if len(r.Conditions.SenderContains) > 0 {
				conds = append(conds, "From: "+strings.Join(r.Conditions.SenderContains, ", "))
			}
			if len(r.Conditions.SubjectContains) > 0 {
				conds = append(conds, "Subject: "+strings.Join(r.Conditions.SubjectContains, ", "))
			}
			condStr := strings.Join(conds, " AND ")
			if condStr == "" {
				condStr = "No conditions"
			}
			fmt.Printf("  [%d] %s (%s)\n", i+1, r.DisplayName, status)
			fmt.Printf("      Criteria: %s\n", condStr)
			fmt.Printf("      Action:   Move to folder ID %s\n", r.Actions.MoveToFolder)
		}
	}
	app.ColorBoldCyan.Println("======================================================================")
	return nil
}

func (c *OutlookClient) ExportRules() error {
	if err := c.init(); err != nil {
		return err
	}

	resp, err := c.client.Get("https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messagerules")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res struct {
		Value []graphRule `json:"value"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	for _, r := range res.Value {
		if strings.HasPrefix(r.DisplayName, "mail_cli_rule_") {
			req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messagerules/%s", r.ID), nil)
			if dResp, dErr := c.client.Do(req); dErr == nil {
				dResp.Body.Close()
			}
		}
	}

	for i, rule := range c.config.Rules {
		if rule.Exported && !c.config.ForceLearn {
			continue
		}
		destFolderID, err := c.getFolderID(rule.Label)
		if err != nil {
			_ = c.EnsureLabelExists(rule.Label)
			destFolderID, _ = c.getFolderID(rule.Label)
		}
		if destFolderID == "" {
			continue
		}

		payload := map[string]interface{}{
			"displayName": fmt.Sprintf("mail_cli_rule_%d", i+1),
			"sequence":    i + 1,
			"isEnabled":   true,
			"actions": map[string]interface{}{
				"moveToFolder": destFolderID,
			},
		}

		conditions := make(map[string]interface{})
		if rule.Sender != "" {
			conditions["senderContains"] = []string{rule.Sender}
		} else if rule.Subject != "" {
			conditions["subjectContains"] = []string{rule.Subject}
		}
		payload["conditions"] = conditions

		bodyBytes, _ := json.Marshal(payload)
		cResp, cErr := c.client.Post("https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messagerules", "application/json", bytes.NewReader(bodyBytes))
		if cErr == nil {
			cResp.Body.Close()
		}
	}

	fmt.Printf("%s Successfully exported local rules to Outlook.\n", app.PrefixSuccess)
	return nil
}
