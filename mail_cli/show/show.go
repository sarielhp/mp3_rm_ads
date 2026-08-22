package show

import (
	"bytes"
	"fmt"
	"io"
	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
	"mail_cli/ui"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Perform(client interface {
	mailclient.LabelReader
	mailclient.EmailFetcher
	mailclient.ConfigProvider
}, config *cfg_g.Config, labelPrefix string, targetMsgID string) error {
	matchedLabels, err := client.GetMatchingLabels(labelPrefix)
	if err != nil {
		return err
	}
	if len(matchedLabels) == 0 {
		return fmt.Errorf("no labels found matching prefix %q", labelPrefix)
	}

	for _, matchedLabel := range matchedLabels {
		cacheDirName := cfg_g.SanitizeLabelForCache(matchedLabel)

		downloadedIDs, err := client.FetchAndDownloadEmails(matchedLabel, cacheDirName)
		if err != nil {
			return fmt.Errorf("error downloading emails for label %s: %w", matchedLabel, err)
		}

		if len(downloadedIDs) == 0 {
			fmt.Printf("%s No emails found in label %s.\n", app.PrefixInfo, matchedLabel)
			continue
		}

		if targetMsgID != "" {
			foundID := ""
			targetUpper := strings.ToUpper(strings.TrimSpace(targetMsgID))
			for _, id := range downloadedIDs {
				if strings.ToUpper(id) == targetUpper || email.ComputeShortID(id) == targetUpper {
					foundID = id
					break
				}
			}
			if foundID == "" {
				continue
			}
			emailBytes, rErr := msg.Read(config.DownloadDir, foundID)
			if rErr != nil {
				return fmt.Errorf("failed to find cached email %s: %w", foundID, rErr)
			}
			msg, err := mail.ReadMessage(bytes.NewReader(emailBytes))
			if err != nil {
				return fmt.Errorf("failed to parse email: %w", err)
			}

			dec := new(mime.WordDecoder)
			subject := email.DecodeHeader(dec, msg.Header.Get("Subject"))
			from := email.DecodeHeader(dec, msg.Header.Get("From"))
			date := email.DecodeHeader(dec, msg.Header.Get("Date"))

			fmt.Println()
			app.ColorBoldCyan.Println("======================================================================")
			fmt.Printf("Message ID: %s in %s\n", foundID, matchedLabel)
			app.ColorBoldCyan.Println("======================================================================")
			app.ColorBoldYellow.Printf("From:    ")
			fmt.Println(from)
			app.ColorBoldYellow.Printf("Subject: ")
			fmt.Println(subject)
			app.ColorBoldYellow.Printf("Date:    ")
			fmt.Println(date)
			app.ColorBoldCyan.Println("----------------------------------------------------------------------")
			if app.FlagShowWeb {
				htmlStr, err := extractHTMLBodyText(msg)
				if err == nil && htmlStr != "" {
					if errOpen := openHTMLInBrowser(htmlStr, config); errOpen != nil {
						return errOpen
					}
					return nil
				}
				fmt.Printf("%s No HTML body found in email, displaying plain text instead.\n", app.PrefixWarn)
			}
			bodyStr, _ := gmail.ExtractPlainBodyText(msg)
			fmt.Println(bodyStr)
			app.ColorBoldCyan.Println("======================================================================")
			fmt.Println()
			return nil
		} else {
			ui.PrintFolderSummary(strings.ToUpper(matchedLabel)+" EMAILS", "", downloadedIDs, nil, nil, config, cacheDirName, 0, 0)
		}
	}
	if targetMsgID != "" {
		return fmt.Errorf("no email found with ID %q in any label matching %q", targetMsgID, labelPrefix)
	}
	return nil
}

func ByID(client mailclient.ConfigProvider, config *cfg_g.Config, msgID string, folderName string) error {
	emailBytes, rErr := msg.Read(client.Config().DownloadDir, msgID)
	if rErr != nil {
		return fmt.Errorf("failed to find cached email: %w", rErr)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(emailBytes))
	if err != nil {
		return fmt.Errorf("failed to parse email: %w", err)
	}

	dec := new(mime.WordDecoder)
	subject := email.DecodeHeader(dec, msg.Header.Get("Subject"))
	from := email.DecodeHeader(dec, msg.Header.Get("From"))
	date := email.DecodeHeader(dec, msg.Header.Get("Date"))
	to := email.DecodeHeader(dec, msg.Header.Get("To"))

	fmt.Println()
	app.ColorBoldCyan.Println("======================================================================")
	app.ColorBoldGreen.Printf("Message ID: ")
	app.ColorWhite.Println(msgID)
	app.ColorBoldGreen.Printf("Folder:     ")
	app.ColorWhite.Println(folderName)
	app.ColorBoldCyan.Println("======================================================================")
	app.ColorBoldYellow.Printf("From:       ")
	app.ColorWhite.Println(from)
	app.ColorBoldYellow.Printf("To:         ")
	app.ColorWhite.Println(to)
	app.ColorBoldYellow.Printf("Subject:    ")
	app.ColorBold.Println(subject)
	app.ColorBoldYellow.Printf("Date:       ")
	app.ColorWhite.Println(date)
	app.ColorBoldCyan.Println("----------------------------------------------------------------------")
	if app.FlagShowWeb {
		htmlStr, err := extractHTMLBodyText(msg)
		if err == nil && htmlStr != "" {
			if errOpen := openHTMLInBrowser(htmlStr, config); errOpen != nil {
				return errOpen
			}
			return nil
		}
		fmt.Printf("%s No HTML body found in email, displaying plain text instead.\n", app.PrefixWarn)
	}
	bodyStr, _ := gmail.ExtractPlainBodyText(msg)
	fmt.Println(bodyStr)
	app.ColorBoldCyan.Println("======================================================================")
	fmt.Println()
	return nil
}

func extractHTMLBodyText(msg *mail.Message) (string, error) {
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		bodyBytes, _ := io.ReadAll(msg.Body)
		return string(bodyBytes), nil
	}

	encoding := msg.Header.Get("Content-Transfer-Encoding")

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		var htmlParts []string

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			partMediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if err != nil {
				continue
			}

			partEncoding := part.Header.Get("Content-Transfer-Encoding")
			partBytes, err := gmail.DecodePartBody(part, partEncoding)
			if err != nil {
				continue
			}

			if partMediaType == "text/html" {
				htmlParts = append(htmlParts, string(partBytes))
			} else if strings.HasPrefix(partMediaType, "multipart/") {
				_, nestedParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
				nestedMR := multipart.NewReader(part, nestedParams["boundary"])
				_, nestedHTML, err := gmail.ExtractNestedMultipartParts(nestedMR)
				if err == nil {
					htmlParts = append(htmlParts, nestedHTML...)
				}
			}
		}

		if len(htmlParts) > 0 {
			return strings.Join(htmlParts, "\n"), nil
		}
		return "", nil
	}

	bodyBytes, err := gmail.DecodePartBody(msg.Body, encoding)
	if err != nil {
		return "", err
	}

	if mediaType == "text/html" {
		return string(bodyBytes), nil
	}
	return fmt.Sprintf("<html><body><pre>%s</pre></body></html>", string(bodyBytes)), nil
}

func openHTMLInBrowser(html string, config *cfg_g.Config) error {
	tempDir, err := app.GetTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	tempPath := filepath.Join(tempDir, "show.html")
	if err := os.WriteFile(tempPath, []byte(html), 0600); err != nil {
		return fmt.Errorf("failed to write html to temp file: %w", err)
	}

	fmt.Printf("%s Opening email in browser...\n", app.PrefixInfo)

	errOpen := gmail.OpenBrowser(config.Browser, tempPath)
	if errOpen != nil {
		return fmt.Errorf("failed to launch browser: %w", errOpen)
	}

	go func() {
		time.Sleep(3 * time.Second)
		_ = os.RemoveAll(tempDir)
	}()

	return nil
}
