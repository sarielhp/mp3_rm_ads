package gmail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// showPoliticalSpamInGmailREST searches recent spam for political keywords, unsubscribes and optionally deletes.
func learnSpamFromGmailREST(config *Config) error {
	srv, err := GetGmailService(config)
	if err != nil {
		return err
	}

	fmt.Printf("%s Searching for recent spam emails to train...\n", app.PrefixInfo)
	res, err := srv.Users.Messages.List("me").Q("label:SPAM").IncludeSpamTrash(true).MaxResults(int64(config.Limit)).Do()
	if err != nil {
		return fmt.Errorf("failed to list spam: %w", err)
	}

	if len(res.Messages) == 0 {
		fmt.Printf("%s No emails found in the spam folder. Nothing to learn.\n", app.PrefixInfo)
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

	if err := downloadMissingSpamEmails(srv, config, missingIDs, "training"); err != nil {
		return err
	}

	configDir := config.ConfigDir
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config", "mail_cli")
	}
	dbPath := filepath.Join(configDir, "trained_message_ids.json")

	trainedMap := make(map[string]bool)
	if !config.ForceLearn {
		if data, errRead := os.ReadFile(dbPath); errRead == nil {
			var loadedList []string
			if errUnmarshal := json.Unmarshal(data, &loadedList); errUnmarshal == nil {
				for _, id := range loadedList {
					trainedMap[id] = true
				}
				if config.Verbose {
					fmt.Printf("%s Loaded %d Message IDs from local trained database\n", app.PrefixInfo, len(trainedMap))
				}
			}
		}
	}

	fmt.Printf("%s Training Bogofilter on %d spam emails...\n", app.PrefixInfo, len(allIDs))

	successCount := 0
	alreadyLearnedCount := 0
	ignoredCount := 0
	dbUpdated := false

	for idx, id := range allIDs {
		if trainedMap[id] {
			alreadyLearnedCount++
			continue
		}

		if idx > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		exists, err := msg.Exists(config.DownloadDir, id)
		if err != nil || !exists {
			ignoredCount++
			trainedMap[id] = true
			dbUpdated = true
			continue
		}

		emailBytes, err := msg.Read(config.DownloadDir, id)
		if err != nil {
			log.Printf("%s Error reading cached spam email ID %s: %v", app.PrefixWarn, id, err)
			continue
		}

		localEmail, errMail := mail.ReadMessage(bytes.NewReader(emailBytes))
		if errMail == nil {
			fromHeader := localEmail.Header.Get("From")
			sender := email.ParseEmailAddress(fromHeader)
			if cfg_g.IsBlacklisted(sender, config.Blacklist) {
				if config.Verbose {
					fmt.Printf("    %s Skipping Bogofilter training for Message ID %s: sender '%s' is blacklisted.\n", app.PrefixInfo, id, sender)
				}
				ignoredCount++
				trainedMap[id] = true
				dbUpdated = true
				continue
			}
		}

		cmd := exec.Command("bogofilter", "-s")
		cmd.Stdin = bytes.NewReader(emailBytes)
		errRun := cmd.Run()
		if errRun == nil {
			successCount++
			trainedMap[id] = true
			dbUpdated = true
			if info, _ := msg.GetInfo(config.DownloadDir, id); info != nil {
				_ = msg.ClearClassification(config.DownloadDir, id)
			}
		} else {
			log.Printf("%s Bogofilter training failed for ID %s: %v", app.PrefixWarn, id, errRun)
		}
	}

	if dbUpdated {
		var listToSave []string
		for id := range trainedMap {
			listToSave = append(listToSave, id)
		}
		if dbBytes, errMarshal := json.MarshalIndent(listToSave, "", "  "); errMarshal == nil {
			_ = os.WriteFile(dbPath, dbBytes, 0600)
		}
	}

	fmt.Printf("%s Successfully trained Bogofilter on ", app.PrefixSuccess)
	app.ColorBoldGreen.Printf("%d", successCount)
	fmt.Printf(" new emails. ")
	app.ColorBoldYellow.Printf("%d", alreadyLearnedCount)
	fmt.Printf(" already trained. ")
	app.ColorBoldRed.Printf("%d", ignoredCount)
	fmt.Println(" ignored.")

	if successCount > 0 {
	}

	return nil
}

// showPoliticalSpamInGmailREST searches recent spam for political keywords, unsubscribes and optionally deletes.
