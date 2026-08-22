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
	"strconv"
	"strings"
)

func parseSpamicity(output string) float32 {
	if idx := strings.Index(output, "spamicity="); idx != -1 {
		sub := output[idx+10:]
		if endIdx := strings.IndexAny(sub, ", \r\n"); endIdx != -1 {
			sub = sub[:endIdx]
		}
		if val, errParse := strconv.ParseFloat(strings.TrimSpace(sub), 64); errParse == nil {
			return float32(val)
		}
	}
	return 0.0
}

func TrainHamUntilMarked(emailBytes []byte, msgID string, config *cfg_g.Config) (float32, error) {
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

		if strings.Contains(output, "Ham") {
			if !config.Quiet {
				fmt.Printf("    %s Message %s is now classified as Ham locally (score: %f).\n", app.PrefixSuccess, msgID, score)
			}
			return score, nil
		}

		if !config.Quiet {
			fmt.Printf("    %s Training message %s as Ham (attempt %d/15, current score: %f)...\n", app.PrefixInfo, msgID, i+1, score)
		}
		cmdTrain := exec.Command("bogofilter", "-n")
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
	if strings.Contains(output, "Ham") {
		return score, nil
	}

	return score, fmt.Errorf("message %s is still not classified as Ham locally after training (score: %f)", msgID, score)
}

func clearTrainedDBs(downloadDir, configDir, msgID string, quiet bool) {
	dbPath := filepath.Join(configDir, "trained_message_ids.json")
	if data, errRead := os.ReadFile(dbPath); errRead == nil {
		var loadedList []string
		if errUnmarshal := json.Unmarshal(data, &loadedList); errUnmarshal == nil {
			var updatedList []string
			changed := false
			for _, id := range loadedList {
				if id == msgID {
					changed = true
				} else {
					updatedList = append(updatedList, id)
				}
			}
			if changed {
				if dbBytes, errMarshal := json.MarshalIndent(updatedList, "", "  "); errMarshal == nil {
					_ = os.WriteFile(dbPath, dbBytes, 0600)
					if !quiet {
						fmt.Println(dbPath)
					}
				}
			}
		}
	}

	jmapDbPath := filepath.Join(downloadDir, "trained_uids.json")
	if data, errRead := os.ReadFile(jmapDbPath); errRead == nil {
		var jmapTrained map[string]bool
		if errUnmarshal := json.Unmarshal(data, &jmapTrained); errUnmarshal == nil {
			if jmapTrained[msgID] {
				delete(jmapTrained, msgID)
				if dbBytes, errMarshal := json.Marshal(jmapTrained); errMarshal == nil {
					_ = os.WriteFile(jmapDbPath, dbBytes, 0600)
					if !quiet {
						fmt.Println(jmapDbPath)
					}
				}
			}
		}
	}

	hamDbPath := filepath.Join(downloadDir, "trained_uids_ham.json")
	if data, errRead := os.ReadFile(hamDbPath); errRead == nil {
		var loadedList []string
		if errUnmarshal := json.Unmarshal(data, &loadedList); errUnmarshal == nil {
			var updatedList []string
			changed := false
			for _, id := range loadedList {
				if id == msgID {
					changed = true
				} else {
					updatedList = append(updatedList, id)
				}
			}
			if changed {
				if dbBytes, errMarshal := json.MarshalIndent(updatedList, "", "  "); errMarshal == nil {
					_ = os.WriteFile(hamDbPath, dbBytes, 0600)
					if !quiet {
						fmt.Println(hamDbPath)
					}
				}
			}
		}
	}
}

func UnspamByID(client interface {
	Validate() error
	mailclient.ConfigProvider
	mailclient.EmailWriter
	mailclient.LabelReader
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

	if !config.Quiet {
		fmt.Printf("%s Training message %s (%q) as ham locally...\n", app.PrefixInfo, targetID, subject)
	}

	score, err := TrainHamUntilMarked(emailBytes, msgID, config)
	if err != nil {
		return err
	}

	clearTrainedDBs(client.Config().DownloadDir, config.ConfigDir, msgID, config.Quiet)

	_ = msg.SetClassification(client.Config().DownloadDir, msgID, false, false, false, score)

	if config.ReadOnly {
		fmt.Printf("[DRY-RUN] Would move message %s to %s\n", targetID, client.InboxFolder())
		return nil
	}

	if !config.Quiet {
		fmt.Printf("%s Successfully learned message %s as ham locally.\n", app.PrefixSuccess, targetID)
		fmt.Printf("%s Moving message back to inbox on server...\n", app.PrefixInfo)
	}
	if err := client.MoveToInbox([]string{msgID}, folderName); err != nil {
		return fmt.Errorf("failed to move message to inbox: %w", err)
	}
	_ = label.Move(client.Config().DownloadDir, msgID, folderName, client.InboxFolder())
	if !config.Quiet {
		fmt.Printf("%s Successfully moved message %s to inbox.\n", app.PrefixSuccess, targetID)
	}

	return nil
}

func UnspamFolder(client interface {
	Validate() error
	mailclient.ConfigProvider
	mailclient.EmailFetcher
	mailclient.EmailWriter
	mailclient.LabelReader
}, config *cfg_g.Config, folderName string) error {
	if err := client.Validate(); err != nil {
		return err
	}

	fmt.Printf("%s Running in Folder Unspam Mode, targeting folder '%s'...\n", app.PrefixInfo, folderName)

	cacheSubdir := cfg_g.SanitizeLabelForCache(folderName)
	ids, err := client.FetchAndDownloadEmails(folderName, cacheSubdir)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		fmt.Printf("%s Folder %q is empty or no emails found. Nothing to unspam.\n", app.PrefixInfo, folderName)
		return nil
	}

	fmt.Printf("%s Training Bogofilter on %d ham emails from folder %q...\n", app.PrefixInfo, len(ids), folderName)

	successIDs := []string{}
	for _, id := range ids {
		exists, _ := msg.Exists(client.Config().DownloadDir, id)
		if !exists {
			continue
		}

		emailBytes, err := msg.Read(client.Config().DownloadDir, id)
		if err != nil {
			fmt.Printf("%s Error reading cached email ID %s: %v\n", app.PrefixWarn, id, err)
			continue
		}

		score, err := TrainHamUntilMarked(emailBytes, id, config)
		if err != nil {
			fmt.Printf("%s Bogofilter training failed for ID %s: %v\n", app.PrefixWarn, id, err)
			continue
		}

		clearTrainedDBs(client.Config().DownloadDir, config.ConfigDir, id, config.Quiet)
		_ = msg.SetClassification(client.Config().DownloadDir, id, false, false, false, score)
		successIDs = append(successIDs, id)
	}

	if len(successIDs) > 0 {
		fmt.Printf("%s Successfully trained Bogofilter on %d email(s) as ham.\n", app.PrefixSuccess, len(successIDs))
		if config.ReadOnly {
			fmt.Printf("[DRY-RUN] Would move %d message(s) to %s\n", len(successIDs), client.InboxFolder())
			return nil
		}
		fmt.Printf("%s Moving %d email(s) back to inbox on server...\n", app.PrefixInfo, len(successIDs))
		if err := client.MoveToInbox(successIDs, folderName); err != nil {
			return fmt.Errorf("failed to move messages to inbox: %w", err)
		}
		for _, id := range successIDs {
			_ = label.Move(client.Config().DownloadDir, id, folderName, client.InboxFolder())
		}
		fmt.Printf("%s Successfully moved %d email(s) to inbox.\n", app.PrefixSuccess, len(successIDs))
	} else {
		fmt.Printf("%s No emails successfully trained as ham.\n", app.PrefixWarn)
	}

	return nil
}

func ResetLearning(config *cfg_g.Config) error {
	downloadDir := config.DownloadDir
	configDir := config.ConfigDir

	if !config.Quiet {
		fmt.Printf("%s Resetting all spam scores and classifications in cache for account...\n", app.PrefixInfo)
	}

	if err := msg.ResetAllClassifications(downloadDir); err != nil {
		return fmt.Errorf("failed to reset classifications: %w", err)
	}

	dbPath := filepath.Join(configDir, "trained_message_ids.json")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		if !config.Quiet {
			fmt.Printf("    %s Warning: failed to remove %s: %v\n", app.PrefixWarn, dbPath, err)
		}
	}

	jmapDbPath := filepath.Join(downloadDir, "trained_uids.json")
	if err := os.Remove(jmapDbPath); err != nil && !os.IsNotExist(err) {
		if !config.Quiet {
			fmt.Printf("    %s Warning: failed to remove %s: %v\n", app.PrefixWarn, jmapDbPath, err)
		}
	}

	hamDbPath := filepath.Join(downloadDir, "trained_uids_ham.json")
	if err := os.Remove(hamDbPath); err != nil && !os.IsNotExist(err) {
		if !config.Quiet {
			fmt.Printf("    %s Warning: failed to remove %s: %v\n", app.PrefixWarn, hamDbPath, err)
		}
	}

	if !config.Quiet {
		fmt.Printf("%s Successfully reset all cache scores and local training data.\n", app.PrefixSuccess)
	}

	return nil
}
