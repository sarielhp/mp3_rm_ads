package spam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
	"mime"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func LearnHam(client interface {
	mailclient.EmailFetcher
	mailclient.ConfigProvider
}, config *cfg_g.Config, folderName string) error {
	if _, err := exec.LookPath("bogofilter"); err != nil {
		return fmt.Errorf("bogofilter executable not found in PATH. Please install bogofilter on your system.\n" +
			"On Debian/Ubuntu/Kali Linux, execute:\n  sudo apt-get install bogofilter")
	}

	fmt.Printf("%s Running in Ham Learning Mode, targeting folder '%s'...\n", app.PrefixInfo, folderName)

	cacheSubdir := cfg_g.SanitizeLabelForCache(folderName)
	ids, err := client.FetchAndDownloadEmails(folderName, cacheSubdir)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		fmt.Printf("%s Folder %q is empty or no emails found. Nothing to learn.\n", app.PrefixInfo, folderName)
		return nil
	}

	dbPath := filepath.Join(config.DownloadDir, "trained_uids_ham.json")
	trainedMap := make(map[string]bool)
	if !config.ForceLearn {
		if data, errRead := os.ReadFile(dbPath); errRead == nil {
			var loadedList []string
			if errUnmarshal := json.Unmarshal(data, &loadedList); errUnmarshal == nil {
				for _, id := range loadedList {
					trainedMap[id] = true
				}
				if config.Verbose {
					fmt.Printf("%s Loaded %d Message IDs from local trained ham database\n", app.PrefixInfo, len(trainedMap))
				}
			}
		}
	}

	fmt.Printf("%s Training Bogofilter on %d ham emails...\n", app.PrefixInfo, len(ids))

	successCount := 0
	alreadyLearnedCount := 0
	ignoredCount := 0
	dbUpdated := false

	for idx, id := range ids {
		if trainedMap[id] {
			alreadyLearnedCount++
			continue
		}

		if idx > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		exists, exErr := msg.Exists(config.DownloadDir, id)
		if !exists {
			if config.Verbose {
				fmt.Printf("%s Warning: Cache file not found for ID %s: %v\n", app.PrefixWarn, id, exErr)
			}
			ignoredCount++
			trainedMap[id] = true
			dbUpdated = true
			continue
		}

		emailBytes, err := msg.Read(config.DownloadDir, id)
		if err != nil {
			fmt.Printf("%s Error reading cached ham email ID %s: %v\n", app.PrefixWarn, id, err)
			continue
		}

		cmd := exec.Command("bogofilter", "-n")
		cmd.Stdin = bytes.NewReader(emailBytes)
		errRun := cmd.Run()
		if errRun == nil {
			successCount++
			trainedMap[id] = true
			dbUpdated = true
		} else {
			fmt.Printf("%s Bogofilter training failed for ID %s: %v\n", app.PrefixWarn, id, errRun)
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
	fmt.Printf(" new ham emails. ")
	app.ColorBoldYellow.Printf("%d", alreadyLearnedCount)
	fmt.Printf(" already trained. ")
	app.ColorBoldRed.Printf("%d", ignoredCount)
	fmt.Println(" ignored.")

	return nil
}

func TrainSpamUntilMarked(emailBytes []byte, msgID string, config *cfg_g.Config) (float32, error) {
	var score float32
	for i := 0; i < 15; i++ {
		cmdTest := exec.Command("bogofilter", "-v")
		cmdTest.Stdin = bytes.NewReader(emailBytes)
		var outBuf bytes.Buffer
		cmdTest.Stdout = &outBuf
		_ = cmdTest.Run()
		output := outBuf.String()

		score = parseSpamicity(output)
		if !config.Quiet {
			fmt.Printf("    %s Bogofilter score: %f\n", app.PrefixInfo, score)
		}

		if strings.Contains(output, "Spam") {
			if !config.Quiet {
				fmt.Printf("    %s Message %s is now classified as Spam locally (score: %f).\n", app.PrefixSuccess, msgID, score)
			}
			return score, nil
		}

		if !config.Quiet {
			fmt.Printf("    %s Training message %s as Spam (attempt %d/15, current score: %f)...\n", app.PrefixInfo, msgID, i+1, score)
		}
		cmdTrain := exec.Command("bogofilter", "-s")
		cmdTrain.Stdin = bytes.NewReader(emailBytes)
		if err := cmdTrain.Run(); err != nil {
			return score, fmt.Errorf("bogofilter training failed: %w", err)
		}
	}

	cmdTest := exec.Command("bogofilter", "-v")
	cmdTest.Stdin = bytes.NewReader(emailBytes)
	var outBuf bytes.Buffer
	cmdTest.Stdout = &outBuf
	_ = cmdTest.Run()
	output := outBuf.String()
	score = parseSpamicity(output)
	if strings.Contains(output, "Spam") {
		return score, nil
	}

	return score, fmt.Errorf("message %s is still not classified as Spam locally after training (score: %f)", msgID, score)
}

func MarkByID(client interface {
	mailclient.ConfigProvider
	Validate() error
	ReportSpam(messageIDs []string, sourceLabelName string) error
}, config *cfg_g.Config, targetID string) error {
	if err := client.Validate(); err != nil {
		return err
	}

	msgID, folderName, err := cache.FindCachedEmailByID(client.Config().DownloadDir, targetID)
	if err != nil {
		return err
	}

	emailBytes, err := msg.Read(client.Config().DownloadDir, msgID)
	if err != nil {
		return fmt.Errorf("failed to read cached email %s: %w", msgID, err)
	}

	subject := ""
	if data, rErr := msg.Read(client.Config().DownloadDir, msgID); rErr == nil {
		if m, errParse := mail.ReadMessage(bytes.NewReader(data)); errParse == nil {
			dec := new(mime.WordDecoder)
			subject = email.DecodeHeader(dec, m.Header.Get("Subject"))
		}
	}
	if subject == "" {
		subject = "(No Subject)"
	}

	fmt.Printf("%s Training message %s (%q) as spam locally...\n", app.PrefixInfo, targetID, subject)

	score, err := TrainSpamUntilMarked(emailBytes, msgID, config)
	if err != nil {
		return err
	}

	dbPath := filepath.Join(config.ConfigDir, "trained_message_ids.json")
	trainedMap := make(map[string]bool)
	if data, errRead := os.ReadFile(dbPath); errRead == nil {
		var loadedList []string
		if errUnmarshal := json.Unmarshal(data, &loadedList); errUnmarshal == nil {
			for _, id := range loadedList {
				trainedMap[id] = true
			}
		}
	}
	if !trainedMap[msgID] {
		trainedMap[msgID] = true
		var listToSave []string
		for id := range trainedMap {
			listToSave = append(listToSave, id)
		}
		if dbBytes, errMarshal := json.MarshalIndent(listToSave, "", "  "); errMarshal == nil {
			_ = os.WriteFile(dbPath, dbBytes, 0600)
		}
	}

	_ = msg.SetClassification(client.Config().DownloadDir, msgID, true, false, false, score)

	isAlreadySpam := strings.EqualFold(folderName, cfg_g.SanitizeLabelForCache(config.SpamFolder)) ||
		strings.EqualFold(folderName, "spam")

	if !isAlreadySpam {
		if config.ReadOnly {
			fmt.Printf("[DRY-RUN] Would mark message %s as spam and move to %s\n", targetID, config.SpamFolder)
			return nil
		}
		fmt.Printf("%s Moving message to spam filter on server...\n", app.PrefixInfo)
		err = client.ReportSpam([]string{msgID}, folderName)
		if err != nil {
			return fmt.Errorf("failed to move message to spam: %w", err)
		}
		_ = msg.Delete(client.Config().DownloadDir, msgID)
		_ = label.Remove(client.Config().DownloadDir, folderName, msgID)
		fmt.Printf("%s Successfully learned and moved message %s to '%s' folder.\n", app.PrefixSuccess, targetID, config.SpamFolder)
	} else {
		fmt.Printf("%s Successfully learned message %s locally (already in '%s' folder).\n", app.PrefixSuccess, targetID, folderName)
	}
	return nil
}
