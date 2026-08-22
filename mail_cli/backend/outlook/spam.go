package outlook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	myme "mail_cli/email"
	"mail_cli/uicommon"
	"mime"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c *OutlookClient) ReportSpam(messageIDs []string, sourceLabelName string) error {
	return c.MoveEmail(messageIDs, sourceLabelName, "Junk Email")
}

func (c *OutlookClient) sendMail(to, subject string, body []byte) error {
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject": subject,
			"body": map[string]interface{}{
				"contentType": "Text",
				"content":     string(body),
			},
			"toRecipients": []map[string]interface{}{
				{
					"emailAddress": map[string]interface{}{
						"address": to,
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	resp, err := c.client.Post("https://graph.microsoft.com/v1.0/me/sendMail", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendMail returned status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *OutlookClient) ShowPoliticalSpam(autoBlacklist bool) error {
	if err := c.init(); err != nil {
		return err
	}
	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Junk Email"
	}
	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Printf("%s Outlook Junk folder is empty.\n", app.PrefixInfo)
		return nil
	}

	dec := new(mime.WordDecoder)
	var politicalIDs []string

	for _, id := range ids {
		emailBytes, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			continue
		}
		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail != nil {
			continue
		}
		subject := myme.DecodeHeader(dec, localEmail.Header.Get("Subject"))
		bodyStr, _ := gmail.ExtractPlainBodyText(localEmail)
		bodyStr = myme.StripHTML(bodyStr)
		if len(bodyStr) > 8192 {
			bodyStr = bodyStr[:8192]
		}
		isPolitical, _, _ := myme.DetectPolitical(subject, bodyStr)
		if isPolitical {
			politicalIDs = append(politicalIDs, id)
		}
	}

	if len(politicalIDs) == 0 {
		return nil
	}

	fmt.Println()
	app.ColorBoldPurple.Println("======================================================================")
	app.ColorBoldPurple.Println("         POLITICAL DONATION EMAILS DETECTED IN OUTLOOK JUNK FOLDER    ")
	app.ColorBoldPurple.Println("======================================================================")

	politicalCount := 0
	var idsToDelete []string

	for _, id := range politicalIDs {
		emailBytes, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			continue
		}
		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail != nil {
			continue
		}
		subject := myme.DecodeHeader(dec, localEmail.Header.Get("Subject"))
		fromHeader := myme.DecodeHeader(dec, localEmail.Header.Get("From"))
		bodyStr, _ := gmail.ExtractPlainBodyText(localEmail)
		bodyStr = myme.StripHTML(bodyStr)
		if len(bodyStr) > 8192 {
			bodyStr = bodyStr[:8192]
		}
		listUnsubscribe := localEmail.Header.Get("List-Unsubscribe")
		_, score, triggered := myme.DetectPolitical(subject, bodyStr)
		sender := myme.ParseEmailAddress(fromHeader)

		isBlacklisted := false
		if autoBlacklist && myme.IsSafeToAutoBlacklist(sender, listUnsubscribe, score) {
			if errBlacklist := cfg_g.AutoBlacklistInternal(c.config, sender); errBlacklist == nil {
				isBlacklisted = true
			}
		}

		politicalCount++
		unsubSuccess := false
		if listUnsubscribe != "" && c.config.AutoUnsubscribe {
			mailto, httpLink := gmail.ParseListUnsubscribe(listUnsubscribe)
			if mailto != "" {
				errUnsub := c.sendMail(mailto, "Unsubscribe", []byte("Please unsubscribe me."))
				gmail.LogUnsubscription(c.config, fromHeader, "mailto", mailto, errUnsub)
				if errUnsub == nil {
					unsubSuccess = true
				}
			}
			if httpLink != "" && !unsubSuccess {
				errUnsub := gmail.ExecuteHTTPUnsubscribe(httpLink)
				gmail.LogUnsubscription(c.config, fromHeader, "http", httpLink, errUnsub)
				if errUnsub == nil {
					unsubSuccess = true
				}
			}
			if unsubSuccess {
				idsToDelete = append(idsToDelete, id)
			}
		}

		if c.config.Verbose {
			fmt.Printf("[%d] ID: ", politicalCount)
			app.ColorBold.Println(id)
			fmt.Printf("    From:    ")
			app.ColorCyan.Println(fromHeader)
			fmt.Printf("    Subject: ")
			app.ColorBold.Println(subject)
			scorePrinter := app.ColorYellow
			if score >= 20.0 {
				scorePrinter = app.ColorRed
			}
			fmt.Printf("    Score:   ")
			scorePrinter.Printf("%.1f", score)
			fmt.Println("/10.0")
			fmt.Printf("    Keys:    ")
			app.ColorYellow.Println(strings.Join(triggered, ", "))
			fmt.Println("----------------------------------------------------------------------")
		} else {
			var actions []string
			if unsubSuccess {
				actions = append(actions, "Unsubscribed")
			}
			if isBlacklisted {
				actions = append(actions, "Blacklisted")
			}
			actionStr := "None"
			if len(actions) > 0 {
				actionStr = strings.Join(actions, ", ")
			}
			fmt.Printf("  Political Spam: %s -> %s\n", sender, actionStr)
		}
	}

	if len(idsToDelete) > 0 {
		_ = c.DeleteAllSpam()
	}
	return nil
}

func (c *OutlookClient) LearnSpam() error {
	if err := c.init(); err != nil {
		return err
	}
	spamFolder := c.account.SpamFolder
	if spamFolder == "" {
		spamFolder = "Junk Email"
	}
	ids, err := c.FetchAndDownloadEmails(spamFolder, "spam")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Printf("%s Outlook Junk folder is empty.\n", app.PrefixInfo)
		return nil
	}

	total := len(ids)

	if err := app.RunPreFlightCheck(); err != nil {
		return err
	}

	trainedUidsPath := filepath.Join(c.config.DownloadDir, "trained_uids.json")
	trainedUids := make(map[string]bool)
	if !c.config.ForceLearn {
		if data, errRead := os.ReadFile(trainedUidsPath); errRead == nil {
			_ = json.Unmarshal(data, &trainedUids)
		}
	}

	successCount := 0
	alreadyLearnedCount := 0
	ignoredCount := 0

	for i, id := range ids {
		if trainedUids[id] {
			alreadyLearnedCount++
			continue
		}
		data, rErr := msg.Read(c.config.DownloadDir, id)
		if rErr != nil {
			ignoredCount++
			continue
		}

		cmd := exec.Command("bogofilter", "-s")
		cmd.Stdin = bytes.NewReader(data)
		if errRun := cmd.Run(); errRun == nil {
			successCount++
			trainedUids[id] = true
			_ = msg.ClearClassification(c.config.DownloadDir, id)
		} else {
			if c.config.Verbose {
				fmt.Printf("    %s Bogofilter training failed for ID %s: %v\n", app.PrefixWarn, id, errRun)
			}
		}
		uicommon.DrawProgressBar(i+1, total, app.PrefixInfo+" Training classifier...")
	}

	if tBytes, errMarshal := json.Marshal(trainedUids); errMarshal == nil {
		_ = os.WriteFile(trainedUidsPath, tBytes, 0600)
	}

	fmt.Printf("%s Successfully trained Bogofilter on ", app.PrefixSuccess)
	app.ColorBoldGreen.Printf("%d", successCount)
	fmt.Printf(" new spam message(s). ")
	app.ColorBoldYellow.Printf("%d", alreadyLearnedCount)
	fmt.Printf(" already trained. ")
	app.ColorBoldRed.Printf("%d", ignoredCount)
	fmt.Println(" ignored.")
	return nil
}
