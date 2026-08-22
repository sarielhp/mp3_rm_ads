package gmail

import (
	"bytes"
	"fmt"
	"log"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mime"
	"net/mail"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

func showPoliticalSpamInGmailREST(config *Config, autoBlacklist bool) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Fetching recent spam emails from Gmail Spam folder...\n", app.PrefixInfo)
	res, err := srv.Users.Messages.List("me").Q("label:SPAM").IncludeSpamTrash(true).MaxResults(int64(config.Limit)).Do()
	if err != nil {
		return fmt.Errorf("failed to list spam: %w", err)
	}

	if len(res.Messages) == 0 {
		fmt.Printf("%s No emails found in Gmail Spam folder.\n", app.PrefixInfo)
		return nil
	}

	var allIDs []string
	var missingIDs []string

	for _, m := range res.Messages {
		allIDs = append(allIDs, m.Id)
		exists, err := msg.Exists(config.DownloadDir, m.Id)
		if err != nil || !exists {
			missingIDs = append(missingIDs, m.Id)
		}
	}

	if err := downloadMissingSpamEmails(srv, config, missingIDs, "political inspection"); err != nil {
		return err
	}

	fmt.Printf("%s Inspecting %d spam emails for political content...\n", app.PrefixInfo, len(allIDs))

	dec := new(mime.WordDecoder)
	var politicalIDs []string

	for _, id := range allIDs {
		emailBytes, err := msg.Read(config.DownloadDir, id)
		if err != nil {
			continue
		}

		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail != nil {
			continue
		}

		subject := email.DecodeHeader(dec, localEmail.Header.Get("Subject"))
		fromHeader := email.DecodeHeader(dec, localEmail.Header.Get("From"))

		sender := email.ParseEmailAddress(fromHeader)
		if cfg_g.IsWhitelisted(sender, config.Whitelist) {
			continue
		}

		bodyStr, _ := ExtractPlainBodyText(localEmail)
		bodyStr = email.StripHTML(bodyStr)
		if len(bodyStr) > 8192 {
			bodyStr = bodyStr[:8192]
		}

		isPolitical, _, _ := email.DetectPolitical(subject, bodyStr)
		if isPolitical {
			politicalIDs = append(politicalIDs, id)
		}
	}

	if len(politicalIDs) == 0 {
		return nil
	}

	fmt.Println()
	app.ColorBoldPurple.Println("======================================================================")
	app.ColorBoldPurple.Println("           POLITICAL DONATION EMAILS DETECTED IN SPAM FOLDER          ")
	app.ColorBoldPurple.Println("======================================================================")

	politicalCount := 0
	var idsToDelete []string

	for _, id := range politicalIDs {
		emailBytes, err := msg.Read(config.DownloadDir, id)
		if err != nil {
			continue
		}

		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail != nil {
			continue
		}

		subject := email.DecodeHeader(dec, localEmail.Header.Get("Subject"))
		fromHeader := email.DecodeHeader(dec, localEmail.Header.Get("From"))
		bodyStr, _ := ExtractPlainBodyText(localEmail)
		bodyStr = email.StripHTML(bodyStr)
		if len(bodyStr) > 8192 {
			bodyStr = bodyStr[:8192]
		}

		listUnsubscribe := localEmail.Header.Get("List-Unsubscribe")
		_, score, triggered := email.DetectPolitical(subject, bodyStr)

		sender := email.ParseEmailAddress(fromHeader)
		isBlacklisted := false
		if autoBlacklist && email.IsSafeToAutoBlacklist(sender, listUnsubscribe, score) {
			if errBlacklist := cfg_g.AutoBlacklistInternal(config, sender); errBlacklist != nil {
				if config.Verbose {
					fmt.Printf("             %s Failed to auto-blacklist sender: %v\n", app.PrefixError, errBlacklist)
				}
			} else {
				isBlacklisted = true
			}
		}

		politicalCount++
		unsubSuccess := false

		if listUnsubscribe != "" && config.AutoUnsubscribe {
			mailto, httpLink := parseListUnsubscribe(listUnsubscribe)
			if mailto != "" {
				if config.Verbose {
					fmt.Printf("             %s Attempting mailto unsubscribe...\n", app.PrefixInfo)
				}
				errUnsub := executeMailtoUnsubscribeREST(srv, config, mailto)
				logUnsubscription(config, fromHeader, "mailto", mailto, errUnsub)
				if errUnsub != nil {
					if config.Verbose {
						fmt.Printf("             %s Mailto unsub failed: %v\n", app.PrefixError, errUnsub)
					}
				} else {
					if config.Verbose {
						fmt.Printf("             %s Mailto unsub sent successfully via Gmail API!\n", app.PrefixSuccess)
					}
					unsubSuccess = true
				}
			}
			if httpLink != "" {
				if config.Verbose {
					fmt.Printf("             %s Attempting HTTP unsubscribe...\n", app.PrefixInfo)
				}
				errUnsub := executeHTTPUnsubscribe(httpLink)
				logUnsubscription(config, fromHeader, "http", httpLink, errUnsub)
				if errUnsub != nil {
					if config.Verbose {
						fmt.Printf("             %s HTTP unsub failed: %v\n", app.PrefixError, errUnsub)
					}
				} else {
					if config.Verbose {
						fmt.Printf("             %s HTTP unsub requested successfully!\n", app.PrefixSuccess)
					}
					unsubSuccess = true
				}
			}

			if unsubSuccess {
				idsToDelete = append(idsToDelete, id)
			}
		}

		if config.Verbose {
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

			if listUnsubscribe != "" {
				fmt.Printf("    Unsub:   Found header: %s\n", listUnsubscribe)
				if config.AutoUnsubscribe && unsubSuccess {
					fmt.Printf("             %s Marked email for deletion from Gmail Spam folder.\n", app.PrefixSuccess)
				}
			}
			fmt.Println("----------------------------------------------------------------------")
		} else {
			var actions []string
			if unsubSuccess {
				actions = append(actions, "Unsubscribed")
			}
			if isBlacklisted {
				actions = append(actions, "Blacklisted")
			}
			actionStr := ""
			if len(actions) > 0 {
				actionStr = " [" + strings.Join(actions, " & ") + "]"
			}
			scorePrinter := app.ColorYellow
			if score >= 20.0 {
				scorePrinter = app.ColorRed
			}
			scoreStr := scorePrinter.Sprintf("%.1f", score)
			fmt.Printf("  - %d. [%s/10.0] From: %s | Subj: %s%s\n", politicalCount, scoreStr, fromHeader, subject, actionStr)
		}
	}

	if len(idsToDelete) > 0 {
		fmt.Printf("%s Deleting %d unsubscribed emails from Gmail Spam folder...\n", app.PrefixInfo, len(idsToDelete))
		err = srv.Users.Messages.BatchDelete("me", &gmailapi.BatchDeleteMessagesRequest{
			Ids: idsToDelete,
		}).Do()
		if err != nil {
			log.Printf("%s Failed to batch delete unsubscribed emails: %v", app.PrefixWarn, err)
		} else {
			fmt.Printf("%s Successfully deleted %d emails from Gmail Spam folder.\n", app.PrefixSuccess, len(idsToDelete))
		}
	}

	fmt.Printf("Found ")
	app.ColorBoldPurple.Printf("%d", politicalCount)
	fmt.Printf(" political email(s) out of %d spam emails analyzed.\n", len(allIDs))
	app.ColorBoldPurple.Println("======================================================================")

	return nil
}
