package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mail_cli/app"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"mail_cli/cache/msg"
	"mail_cli/uicommon"
	"sync"
)

// renderHTMLToText converts raw HTML to plain text using w3m -dump.
func renderHTMLToText(htmlStr string) (string, error) {
	if htmlStr == "" {
		return "", nil
	}

	w3mPath, err := exec.LookPath("w3m")
	if err != nil {
		return "", fmt.Errorf("w3m not found (required for HTML email rendering); please install it (e.g. apt install w3m), or provide a text/plain part in your emails. (looked in $PATH for %q)", "w3m")
	}
	_ = w3mPath

	cols := app.GetTerminalWidth()
	if cols > 6 {
		cols -= 4
	}
	if cols > 100 {
		cols = 100
	}
	if cols < 20 {
		cols = 70
	}
	cmd := exec.Command("w3m", "-dump", "-cols", fmt.Sprintf("%d", cols), "-T", "text/html", "-no-cookie")
	cmd.Stdin = strings.NewReader(htmlStr)
	var out bytes.Buffer
	cmd.Stdout = &out

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("w3m -dump failed: %w", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		return "", fmt.Errorf("w3m -dump timed out after 5s")
	}

	result := strings.TrimSpace(out.String())
	if result == "" {
		return "", fmt.Errorf("w3m -dump produced empty output for %d bytes of HTML content", len(htmlStr))
	}
	return result, nil
}

// decodePartBody decodes base64 or quoted-printable body content.
func DecodePartBody(r io.Reader, encoding string) ([]byte, error) {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	if encoding == "base64" {
		dec := base64.NewDecoder(base64.StdEncoding, r)
		return io.ReadAll(dec)
	} else if encoding == "quoted-printable" {
		dec := quotedprintable.NewReader(r)
		return io.ReadAll(dec)
	}
	return io.ReadAll(r)
}

// extractPlainBodyText extracts plain text from a parsed email message/part.
func ExtractPlainBodyText(msg *mail.Message) (string, error) {
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// Fallback to reading raw body
		bodyBytes, _ := io.ReadAll(msg.Body)
		return string(bodyBytes), nil
	}

	encoding := msg.Header.Get("Content-Transfer-Encoding")

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		var plainTextParts []string
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

			if strings.HasPrefix(partMediaType, "multipart/") {
				// Recurse into nested multipart parts WITHOUT decoding the part body beforehand
				_, nestedParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
				nestedMR := multipart.NewReader(part, nestedParams["boundary"])
				nestedPlain, nestedHTML, err := ExtractNestedMultipartParts(nestedMR)
				if err == nil {
					plainTextParts = append(plainTextParts, nestedPlain...)
					htmlParts = append(htmlParts, nestedHTML...)
				}
			} else {
				partEncoding := part.Header.Get("Content-Transfer-Encoding")
				partBytes, err := DecodePartBody(part, partEncoding)
				if err != nil {
					continue
				}

				if partMediaType == "text/plain" {
					plainTextParts = append(plainTextParts, string(partBytes))
				} else if partMediaType == "text/html" {
					htmlParts = append(htmlParts, string(partBytes))
				}
			}
		}

		if len(plainTextParts) > 0 {
			return strings.Join(plainTextParts, "\n"), nil
		}
		if len(htmlParts) > 0 {
			// Render HTML to plain text via w3m
			htmlContent := strings.Join(htmlParts, "\n")
			result, err := renderHTMLToText(htmlContent)
			if err != nil {
				// Fallback to email.StripHTML
				if fallback := email.StripHTML(htmlContent); fallback != "" {
					return fallback, nil
				}
				return htmlContent, nil // Absolute fallback: raw HTML
			}
			return result, nil
		}
		return "", nil
	}

	bodyBytes, err := DecodePartBody(msg.Body, encoding)
	if err != nil {
		return "", err
	}

	if mediaType == "text/html" {
		res, err := renderHTMLToText(string(bodyBytes))
		if err != nil {
			if fallback := email.StripHTML(string(bodyBytes)); fallback != "" {
				return fallback, nil
			}
			return string(bodyBytes), nil
		}
		return res, nil
	}
	return string(bodyBytes), nil
}

// extractNestedMultipartParts recursively extracts plain text and HTML from nested multipart parts.
func ExtractNestedMultipartParts(mr *multipart.Reader) ([]string, []string, error) {
	var plainTextParts []string
	var htmlParts []string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return plainTextParts, htmlParts, err
		}

		partMediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			continue
		}

		if strings.HasPrefix(partMediaType, "multipart/") {
			_, nestedParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			nestedMR := multipart.NewReader(part, nestedParams["boundary"])
			nestedPlain, nestedHTML, err := ExtractNestedMultipartParts(nestedMR)
			if err == nil {
				plainTextParts = append(plainTextParts, nestedPlain...)
				htmlParts = append(htmlParts, nestedHTML...)
			}
		} else {
			partEncoding := part.Header.Get("Content-Transfer-Encoding")
			partBytes, err := DecodePartBody(part, partEncoding)
			if err != nil {
				continue
			}

			if partMediaType == "text/plain" {
				plainTextParts = append(plainTextParts, string(partBytes))
			} else if partMediaType == "text/html" {
				htmlParts = append(htmlParts, string(partBytes))
			}
		}
	}
	return plainTextParts, htmlParts, nil
}

// isInsufficientScopeError checks if the error indicates insufficient OAuth scope.
func IsInsufficientScopeError(err error) bool {
	if err == nil {
		return false
	}
	if gErr, ok := err.(*googleapi.Error); ok {
		for _, e := range gErr.Errors {
			if e.Reason == "insufficientPermissions" || e.Reason == "forbidden" {
				return true
			}
		}
	}
	return false
}

// handleScopeError deletes the cached token if it's an insufficient scope error and returns a formatted error.
func HandleScopeError(config *Config, err error) error {
	if IsInsufficientScopeError(err) {
		configDir := config.ConfigDir
		if configDir == "" {
			homeDir, errHome := os.UserHomeDir()
			if errHome == nil {
				configDir = filepath.Join(homeDir, ".config", app.AppName)
			}
		}
		if configDir != "" {
			tokenPath := GetTokenPath(config)
			_ = os.Remove(tokenPath)
			fmt.Printf("\n%s Error: Insufficient authentication scopes to manage Gmail filters.\n", app.PrefixError)
			fmt.Printf("%s Deleted outdated token from %s\n", app.PrefixInfo, tokenPath)
			fmt.Printf("%s Please run the command again to re-authenticate with the required settings scopes.\n", app.PrefixInfo)
		} else {
			fmt.Printf("\n%s Error: Insufficient authentication scopes to manage Gmail filters.\n", app.PrefixError)
		}
		return fmt.Errorf("insufficient authentication scopes")
	}
	return err
}

// getTokenPath computes the OAuth token file path, supporting account-specific tokens.
func GetTokenPath(config *Config) string {
	configDir := config.ConfigDir
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configDir = filepath.Join(homeDir, ".config", app.AppName)
		}
	}
	tokenName := "token.json"
	if config.SelectedAccount != "" {
		tokenName = fmt.Sprintf("token_%s.json", cfg_g.SanitizeLabelForCache(config.SelectedAccount))
	}
	return filepath.Join(configDir, tokenName)
}

// decodeGmailRaw decodes Gmail's raw message Base64 string.
func DecodeGmailRaw(rawStr string) ([]byte, error) {
	dec, err := base64.RawURLEncoding.DecodeString(rawStr)
	if err == nil {
		return dec, nil
	}

	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, rawStr)

	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.StdEncoding} {
		dec, err = enc.DecodeString(cleaned)
		if err == nil {
			return dec, nil
		}
	}

	return nil, fmt.Errorf("failed to decode base64 after %d attempts", len(rawStr))
}

// resolveLabelIDByName resolves a label name to its corresponding Gmail Label ID.
func resolveLabelIDByName(srv *gmailapi.Service, config *Config, labelName string) (string, error) {
	if strings.EqualFold(labelName, "inbox") {
		return "INBOX", nil
	}
	if strings.EqualFold(labelName, "spam") {
		return "SPAM", nil
	}
	if strings.EqualFold(labelName, "trash") {
		return "TRASH", nil
	}
	labelsRes, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return "", fmt.Errorf("failed to list Gmail labels: %w", err)
	}
	for _, l := range labelsRes.Labels {
		if strings.EqualFold(l.Name, labelName) {
			return l.Id, nil
		}
	}
	return "", fmt.Errorf("label %q not found on Gmail", labelName)
}

// downloadMissingSpamEmails fetches and caches missing emails concurrently from Gmail Spam folder.
func downloadMissingSpamEmails(srv *gmailapi.Service, config *Config, missingIDs []string, purpose string) error {
	return downloadMissingSpamEmailsWithContext(context.Background(), srv, config, missingIDs, purpose)
}

// downloadMissingSpamEmailsWithContext fetches and caches missing emails concurrently with context support.
func downloadMissingSpamEmailsWithContext(ctx context.Context, srv *gmailapi.Service, config *Config, missingIDs []string, purpose string) error {
	if len(missingIDs) == 0 {
		return nil
	}

	fmt.Printf("%s Downloading %d missing spam emails for %s...\n", app.PrefixInfo, len(missingIDs), purpose)

	sem := make(chan struct{}, 8)
	errChan := make(chan error, len(missingIDs))
	var wg sync.WaitGroup

	var mu sync.Mutex
	count := 0
	total := len(missingIDs)

	for i, id := range missingIDs {
		wg.Add(1)
		go func(msgID string, idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context before starting each download
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			resMsg, err := srv.Users.Messages.Get("me", msgID).Format("raw").Do()
			if err != nil {
				errChan <- fmt.Errorf("failed to download message %s: %w", msgID, err)
				return
			}

			rawBytes, err := DecodeGmailRaw(resMsg.Raw)
			if err != nil {
				errChan <- fmt.Errorf("failed to decode message %s: %w", msgID, err)
				return
			}

			if err := msg.Store(config.DownloadDir, msgID, rawBytes, time.Now()); err != nil {
				errChan <- fmt.Errorf("failed to store message %s to cache: %w", msgID, err)
				return
			}

			mu.Lock()
			count++
			current := count
			mu.Unlock()

			if config.Verbose {
				fmt.Printf("    %s Cached spam email ID %s locally\n", app.PrefixSuccess, msgID)
			} else {
				uicommon.DrawProgressBar(current, total, app.PrefixInfo+" Downloading spam emails")
			}
		}(id, i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}
	if !config.Verbose {
		fmt.Println()
	}
	return nil
}
