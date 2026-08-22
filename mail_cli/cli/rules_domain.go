package cli

import (
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/app"
	"mail_cli/cache"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/email"

	"github.com/sarielhp/clihelp"
)

func ruleAddDomainCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "add-domain",
		Aliases:     []string{"add_domain"},
		Title:       "rule add_domain <message_id> [lbl]",
		Description: "Add an auto-labeling rule for all emails from the sender's domain. Extracts the domain of the sender of the specified cached email and creates a rule to auto-label all emails from that domain.",
		UsageLine:   "mail_cli rule add_domain <message_id> [lbl]",
		Parameters: []clihelp.Param{
			{Name: "<message_id>", Description: "The message ID or short ID of the email."},
			{Name: "[lbl]", Description: "The target label hierarchy (optional; defaults to message folder or SpamLearn folder)."},
		},
		Examples: []clihelp.Example{
			{Line: `mail_cli rule add_domain 12345 "Sort/Newsletters"`},
			{Line: "mail_cli rule add_domain 12345"},
		},
		Args: clihelp.RangeArgs(1, 2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			messageID := args[0]
			label := ""
			if len(args) > 1 {
				label = args[1]
			}
			return addDomainRuleToConfig(session, messageID, label)
		},
	}
}

func addDomainRuleToConfig(session *app.Session, messageID, label string) error {
	_, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(session.Config)
	if err != nil {
		return err
	}

	foundID, folderName, downloadDir, err := resolveCachedEmail(session, targetAcc, messageID)
	if err != nil {
		return fmt.Errorf("failed to find message %s: %w", messageID, err)
	}

	rawMsg, err := msg.Read(downloadDir, foundID)
	if err != nil {
		return fmt.Errorf("failed to read cached message %s: %w", messageID, err)
	}

	parsedMsg, err := mail.ReadMessage(bytes.NewReader(rawMsg))
	if err != nil {
		return fmt.Errorf("failed to parse cached message %s: %w", messageID, err)
	}

	fromRaw := strings.TrimSpace(parsedMsg.Header.Get("From"))
	fromEmail := email.ParseEmailAddress(fromRaw)
	if fromEmail == "" {
		return fmt.Errorf("could not extract sender email address from message %s", messageID)
	}

	parts := strings.Split(fromEmail, "@")
	if len(parts) < 2 || parts[len(parts)-1] == "" {
		return fmt.Errorf("invalid sender email address %q in message %s", fromEmail, messageID)
	}

	domain := strings.ToLower(parts[len(parts)-1])
	domainPattern := "@" + domain

	if label == "" {
		if folderName != "" && !strings.EqualFold(folderName, "inbox") {
			label = folderName
		} else if targetAcc.SpamLearn != "" {
			label = targetAcc.SpamLearn
		} else {
			return fmt.Errorf("no label specified and message is in inbox; please provide a target label")
		}
	}

	return addRuleToConfig(session, domainPattern, label, "")
}

func resolveCachedEmail(session *app.Session, targetAcc *cfg_acc.AccountConfig, messageID string) (foundID string, folderName string, downloadDir string, err error) {
	var candidateDirs []string

	if session.GetClient != nil {
		if client, cErr := session.GetClient(session.Config); cErr == nil && client != nil {
			if cfg := client.Config(); cfg != nil && cfg.DownloadDir != "" {
				candidateDirs = append(candidateDirs, cfg.DownloadDir)
			}
		}
	}

	if session.DownloadDir != "" {
		candidateDirs = append(candidateDirs, session.DownloadDir)
	}

	baseDir := session.Config.DownloadDir
	if baseDir != "" && targetAcc != nil {
		accDir := cfg_g.SanitizeLabelForCache(targetAcc.Name)
		if accDir != "" {
			candidateDirs = append(candidateDirs, filepath.Join(baseDir, accDir))
		}
	}

	if baseDir != "" {
		candidateDirs = append(candidateDirs, baseDir)
		if entries, rErr := os.ReadDir(baseDir); rErr == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					candidateDirs = append(candidateDirs, filepath.Join(baseDir, entry.Name()))
				}
			}
		}
	}

	seen := make(map[string]bool)
	for _, dir := range candidateDirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true

		if fid, fname, fErr := cache.FindCachedEmailByID(dir, messageID); fErr == nil {
			return fid, fname, dir, nil
		}
	}

	return "", "", "", fmt.Errorf("no cached email found with ID %q", messageID)
}
