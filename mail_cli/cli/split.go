package cli

import (
	"bytes"
	"fmt"
	"mime"
	"net/mail"
	"regexp"
	"strings"

	"mail_cli/app"
	"mail_cli/cache/label"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"

	"github.com/sarielhp/clihelp"
)

var flagSplitDo bool

// SplitCmd returns the clihelp.Command for the split command.
func SplitCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "split",
		Description: "Scan messages in the source label. If their subject matches the pattern (which may contain wildcards * and ?), move them to the target label. Runs in dry-run mode by default; use --do to perform actual operations.",
		UsageLine:   "mail_cli split <source_label> <pattern> <target_label> [--do]",
		Parameters: []clihelp.Param{
			{Name: "<source_label>", Description: "Source folder/label containing messages to split."},
			{Name: "<pattern>", Description: "Subject pattern with wildcards (*, ?) to match."},
			{Name: "<target_label>", Description: "Destination folder/label to move matched messages to."},
		},
		Options: []clihelp.Option{
			clihelp.Bool(&flagSplitDo, "--do", false, "Perform the actual move operations"),
		},
		Examples: []clihelp.Example{
			{Line: `mail_cli split Inbox "*invoice*" Receipts`},
			{Line: `mail_cli split Inbox "*invoice*" Receipts --do`},
		},
		Args: clihelp.ExactArgs(3),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			sourceLabel := args[0]
			pattern := args[1]
			targetLabel := args[2]

			srcClient, srcFolder, err := session.ResolveClientAndLabel(sourceLabel)
			if err != nil {
				return err
			}
			if err := srcClient.Validate(); err != nil {
				return err
			}

			targetClient, targetFolder, err := session.ResolveClientAndLabel(targetLabel)
			if err != nil {
				return err
			}
			if err := targetClient.Validate(); err != nil {
				return err
			}

			sourceUnique, err := resolveUniqueLabel(srcClient, srcFolder)
			if err != nil {
				return fmt.Errorf("failed to resolve source label: %w", err)
			}
			targetUnique, err := resolveUniqueLabel(targetClient, targetFolder)
			if err != nil {
				return fmt.Errorf("failed to resolve target label: %w", err)
			}

			targetCacheDirName := cfg_g.SanitizeLabelForCache(targetUnique)
			if targetCacheDirName == "" {
				targetCacheDirName = targetUnique
			}
			targetMessageIDs, err := targetClient.FetchAndDownloadEmails(targetUnique, targetCacheDirName)
			if err != nil {
				return fmt.Errorf("failed to fetch target folder emails: %w", err)
			}
			existingTargetIDs := make(map[string]bool)
			for _, id := range targetMessageIDs {
				existingTargetIDs[id] = true
			}

			cacheDirName := cfg_g.SanitizeLabelForCache(sourceUnique)
			if cacheDirName == "" {
				cacheDirName = sourceUnique
			}

			messageIDs, err := srcClient.FetchAndDownloadEmails(sourceUnique, cacheDirName)
			if err != nil {
				return err
			}
			if len(messageIDs) == 0 {
				fmt.Printf("%s Source label %q has no messages to scan.\n", app.PrefixInfo, sourceUnique)
				return nil
			}

			downloadDir := srcClient.Config().DownloadDir
			var matchedMsgs []string
			dec := new(mime.WordDecoder)

			for _, msgID := range messageIDs {
				rawBytes, rErr := msg.Read(downloadDir, msgID)
				if rErr != nil {
					continue
				}
				m, rErr := mail.ReadMessage(bytes.NewReader(rawBytes))
				if rErr != nil {
					continue
				}
				subject := email.DecodeHeader(dec, m.Header.Get("Subject"))
				if subject == "" {
					subject = "(No Subject)"
				}

				matched, mErr := matchPattern(pattern, subject)
				if mErr != nil {
					return mErr
				}
				if matched {
					matchedMsgs = append(matchedMsgs, msgID)
				}
			}

			if len(matchedMsgs) == 0 {
				fmt.Printf("%s No messages in %q matched pattern %q.\n", app.PrefixInfo, sourceUnique, pattern)
				return nil
			}

			srcDisplay := app.FormatAccountLabel(session, srcClient, sourceUnique)
			targetDisplay := app.FormatAccountLabel(session, targetClient, targetUnique)

			fmt.Printf("%s Found %d messages matching pattern %q in %q:\n", app.PrefixInfo, len(matchedMsgs), pattern, sourceUnique)

			if !flagSplitDo {
				// Dry-run
				fmt.Printf("%s [Dry-Run] Would move/remove %d message(s) from %s to %s (pattern: %q):\n", app.PrefixInfo, len(matchedMsgs), srcDisplay, targetDisplay, pattern)
				for _, msgID := range matchedMsgs {
					rawBytes, _ := msg.Read(downloadDir, msgID)
					var subject string
					if m, err := mail.ReadMessage(bytes.NewReader(rawBytes)); err == nil {
						subject = email.DecodeHeader(dec, m.Header.Get("Subject"))
					}
					status := ""
					if existingTargetIDs[msgID] {
						status = " (already in target, will only remove from source)"
					}
					fmt.Printf("  - %s: %s%s\n", msgID[:8], subject, status)
				}
				fmt.Printf("\nUse --do flag to perform the actual move.\n")
			} else {
				// Real run
				fmt.Printf("%s Moving/removing %d message(s) from %s to %s on server...\n", app.PrefixInfo, len(matchedMsgs), srcDisplay, targetDisplay)
				if srcClient == targetClient {
					if err := srcClient.MoveEmail(matchedMsgs, sourceUnique, targetUnique); err != nil {
						return fmt.Errorf("failed to move emails: %w", err)
					}
					// Update local cache
					for _, msgID := range matchedMsgs {
						_ = label.Move(downloadDir, msgID, sourceUnique, targetUnique)
						_ = msg.ClearClassification(downloadDir, msgID)
					}
				} else {
					// Inter-account transfer
					trashTarget := "Trash"
					if session.ResolveTrashTarget != nil {
						if t, err := session.ResolveTrashTarget(srcClient); err == nil && t != "" {
							trashTarget = t
						}
					}
					for _, msgID := range matchedMsgs {
						rawBytes, rErr := msg.Read(downloadDir, msgID)
						if rErr != nil {
							continue
						}
						if err := targetClient.UploadRawEmail(rawBytes, targetUnique); err != nil {
							return fmt.Errorf("failed to upload message %s to target account: %w", msgID, err)
						}
						_ = srcClient.MoveEmail([]string{msgID}, sourceUnique, trashTarget)
					}
				}
				fmt.Printf("%s Successfully moved/removed %d message(s) from %s to %s.\n", app.PrefixSuccess, len(matchedMsgs), srcDisplay, targetDisplay)
			}

			return nil
		},
	}
}

func resolveUniqueLabel(client mailclient.MailClient, name string) (string, error) {
	return resolveSingleLabel(client, name)
}

func matchPattern(pattern, subject string) (bool, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '\\', '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|':
			sb.WriteString("\\")
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile("(?i)" + sb.String())
	if err != nil {
		return false, err
	}
	return re.MatchString(subject), nil
}
