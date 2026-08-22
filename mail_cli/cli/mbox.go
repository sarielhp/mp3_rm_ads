package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"

	"github.com/sarielhp/clihelp"
)

// DownloadCmd returns the clihelp.Command for the download command.
func DownloadCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "download",
		Description: "Download all messages in the specified label (which must match a unique label) to a local mbox file.",
		UsageLine:   "mail_cli download <label> <file_name>",
		Parameters: []clihelp.Param{
			{Name: "<label>", Description: "The unique label/folder to download messages from."},
			{Name: "<file_name>", Description: "The local mbox destination file path."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli download receipts ~/archive/receipts.mbox"},
		},
		Args: clihelp.ExactArgs(2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			labelName := args[0]
			mboxPath := args[1]

			if session.GetClient == nil {
				return fmt.Errorf("client not configured")
			}
			client, err := session.GetClient(session.Config)
			if err != nil {
				return err
			}
			if err := client.Validate(); err != nil {
				return err
			}

			uniqueLabel, err := resolveUniqueLabel(client, labelName)
			if err != nil {
				return fmt.Errorf("failed to resolve source label: %w", err)
			}

			cacheDirName := cfg_g.SanitizeLabelForCache(uniqueLabel)
			if cacheDirName == "" {
				cacheDirName = uniqueLabel
			}

			messageIDs, err := client.FetchAndDownloadEmails(uniqueLabel, cacheDirName)
			if err != nil {
				return err
			}
			if len(messageIDs) == 0 {
				fmt.Printf("%s Folder %q has no messages to download.\n", app.PrefixInfo, uniqueLabel)
				return nil
			}

			outFile, err := os.OpenFile(mboxPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return fmt.Errorf("failed to open mbox file for writing: %w", err)
			}
			defer outFile.Close()

			downloadDir := client.Config().DownloadDir
			fmt.Printf("%s Downloading %d messages from %q to %q...\n", app.PrefixInfo, len(messageIDs), uniqueLabel, mboxPath)

			for _, msgID := range messageIDs {
				rawBytes, err := msg.Read(downloadDir, msgID)
				if err != nil {
					fmt.Printf("%s Failed to read message %s from cache, skipping: %v\n", app.PrefixWarn, msgID, err)
					continue
				}

				m, err := mail.ReadMessage(bytes.NewReader(rawBytes))
				if err != nil {
					fmt.Printf("%s Failed to parse message %s headers, skipping: %v\n", app.PrefixWarn, msgID, err)
					continue
				}

				fromAddr := strings.TrimSpace(m.Header.Get("From"))
				cleanFrom := "placeholder@example.com"
				if addr, pErr := mail.ParseAddress(fromAddr); pErr == nil && addr != nil {
					cleanFrom = addr.Address
				}

				var dateVal time.Time
				if dateStr := m.Header.Get("Date"); dateStr != "" {
					if parsed, pErr := mail.ParseDate(dateStr); pErr == nil {
						dateVal = parsed
					}
				}
				if dateVal.IsZero() {
					dateVal = time.Now()
				}

				if err := writeToMbox(outFile, rawBytes, cleanFrom, dateVal); err != nil {
					return fmt.Errorf("failed to write message %s to mbox: %w", msgID, err)
				}
			}

			fmt.Printf("%s Successfully downloaded %d messages to %q.\n", app.PrefixSuccess, len(messageIDs), mboxPath)
			return nil
		},
	}
}

// UploadCmd returns the clihelp.Command for the upload command.
func UploadCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "upload",
		Description: "Upload all email messages from a local mbox file to the specified target label/folder on the server.",
		UsageLine:   "mail_cli upload <label> <file_name>",
		Parameters: []clihelp.Param{
			{Name: "<label>", Description: "The target label/folder to upload messages into."},
			{Name: "<file_name>", Description: "The local mbox file path to read messages from."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli upload receipts ~/archive/receipts.mbox"},
		},
		Args: clihelp.ExactArgs(2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			labelName := args[0]
			mboxPath := args[1]

			if session.GetClient == nil {
				return fmt.Errorf("client not configured")
			}
			client, err := session.GetClient(session.Config)
			if err != nil {
				return err
			}
			if err := client.Validate(); err != nil {
				return err
			}

			uniqueLabel, err := resolveUniqueLabel(client, labelName)
			if err != nil {
				return fmt.Errorf("failed to resolve target label: %w", err)
			}

			inFile, err := os.Open(mboxPath)
			if err != nil {
				return fmt.Errorf("failed to open mbox file for reading: %w", err)
			}
			defer inFile.Close()

			messages, err := readFromMbox(inFile)
			if err != nil {
				return fmt.Errorf("failed to parse mbox file: %w", err)
			}
			if len(messages) == 0 {
				fmt.Printf("%s No messages found in mbox file %q.\n", app.PrefixInfo, mboxPath)
				return nil
			}

			fmt.Printf("%s Uploading %d messages to label %q...\n", app.PrefixInfo, len(messages), uniqueLabel)

			successCount := 0
			for idx, msg := range messages {
				if err := client.UploadRawEmail(msg.Body, uniqueLabel); err != nil {
					fmt.Printf("%s [%d] Failed to upload message, skipping: %v\n", app.PrefixError, idx+1, err)
					continue
				}
				successCount++
			}

			fmt.Printf("%s Successfully uploaded %d/%d messages to %q.\n", app.PrefixSuccess, successCount, len(messages), uniqueLabel)
			return nil
		},
	}
}

func writeToMbox(w io.Writer, rawEmail []byte, fromAddr string, date time.Time) error {
	if fromAddr == "" {
		fromAddr = "unknown@example.com"
	}

	if _, err := mail.ParseAddress(fromAddr); err != nil {
		fromAddr = "invalid@example.com"
	}

	dateStr := date.Format("Mon Jan _2 15:04:05 2006")
	_, err := fmt.Fprintf(w, "From %s %s\n", fromAddr, dateStr)
	if err != nil {
		return err
	}

	lines := strings.Split(string(rawEmail), "\n")
	for _, line := range lines {
		lineNormalized := strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(lineNormalized, "From ") {
			_, err = fmt.Fprintf(w, ">%s\n", lineNormalized)
		} else {
			_, err = fmt.Fprintf(w, "%s\n", lineNormalized)
		}
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w)
	return err
}

type MboxMessage struct {
	From string
	Date string
	Body []byte
}

func readFromMbox(r io.Reader) ([]MboxMessage, error) {
	var msgs []MboxMessage
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var currentMsg *strings.Builder
	var currentFrom string
	var currentDate string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "From ") {
			if currentMsg != nil {
				if currentFrom == "" {
					currentFrom = "unknown@example.com"
				}
				msgs = append(msgs, MboxMessage{
					From: currentFrom,
					Date: currentDate,
					Body: []byte(currentMsg.String()),
				})
			}
			currentMsg = &strings.Builder{}
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 {
				currentFrom = parts[1]
			}
			if len(parts) >= 3 {
				currentDate = parts[2]
			}
		} else {
			if currentMsg != nil {
				if strings.HasPrefix(line, ">From ") {
					line = line[1:]
				}
				currentMsg.WriteString(line + "\n")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if currentMsg != nil {
		if currentFrom == "" {
			currentFrom = "unknown@example.com"
		}
		msgs = append(msgs, MboxMessage{
			From: currentFrom,
			Date: currentDate,
			Body: []byte(currentMsg.String()),
		})
	}
	return msgs, nil
}
