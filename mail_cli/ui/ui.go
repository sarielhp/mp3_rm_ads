package ui

import (
	"bytes"
	"fmt"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/uicommon"
	"mime"
	"net/mail"
	"sort"
	"strings"
	"time"
)

func PrintFolderSummary(folderTitle string, numPrefix string, ids []string, spamIDs []string, scanResults map[string]*uicommon.SpamResponse, config *cfg_g.Config, cacheSubdir string, movedCount int, totalCountBeforePattern int) {
	width := app.GetTerminalWidth()
	separator := strings.Repeat("=", width)

	app.ColorBoldCyan.Println(folderTitle)
	app.ColorBoldCyan.Println(separator)

	dec := new(mime.WordDecoder)

	isSpamMap := make(map[string]bool)
	for _, id := range spamIDs {
		isSpamMap[id] = true
	}

	var emails []uicommon.FolderEmail

	for _, id := range ids {
		data, lErr := msg.Read(config.DownloadDir, id)
		var subject string
		var fromEmail string
		var emailDate time.Time
		var messageID string
		var inReplyTo string
		var references string
		var fromRaw string
		if lErr != nil {
			subject = fmt.Sprintf("[Error reading cached message ID: %s]", id)
			fromEmail = "[Error]"
			fromRaw = "[Error]"
		} else {
			msg, errParse := mail.ReadMessage(bytes.NewReader(data))
			if errParse != nil {
				subject = fmt.Sprintf("[Error parsing cached message ID: %s]", id)
				fromEmail = "[Error]"
				fromRaw = "[Error]"
			} else {
				subject = email.DecodeHeader(dec, msg.Header.Get("Subject"))
				if subject == "" {
					subject = "(No Subject)"
				}
				fromEmail = email.ParseEmailAddress(msg.Header.Get("From"))
				if fromEmail == "" {
					fromEmail = "(Unknown Sender)"
				}
				fromRaw = strings.TrimSpace(msg.Header.Get("From"))
				if fromRaw == "" {
					fromRaw = "(Unknown Sender)"
				}
				dateStrHeader := msg.Header.Get("Date")
				if dateStrHeader != "" {
					if parsedDate, errD := mail.ParseDate(dateStrHeader); errD == nil {
						emailDate = parsedDate
					}
				}
				messageID = strings.TrimSpace(msg.Header.Get("Message-ID"))
				inReplyTo = strings.TrimSpace(msg.Header.Get("In-Reply-To"))
				references = strings.TrimSpace(msg.Header.Get("References"))
			}
		}

		if emailDate.IsZero() {
			emailDate = time.Now()
		}
		formattedDate := email.FormatEmailDate(emailDate)
		subject = uicommon.ReverseHebrewRuns(subject)

		isPolitical := false
		isBlacklisted := false
		if res, ok := scanResults[id]; ok && res != nil {
			if _, hasPoli := res.Symbols["CUSTOM_POLITICAL_BLOCK"]; hasPoli {
				isPolitical = true
			}
			if strings.Contains(res.Action, "Blacklisted Sender") {
				isBlacklisted = true
			}
		}

		hasICS := hasICSAttachment(config.DownloadDir, id)
		hasAttachment := hasGeneralAttachment(config.DownloadDir, id)

		emails = append(emails, uicommon.FolderEmail{
			ID:            id,
			Subject:       subject,
			FromEmail:     fromEmail,
			FromRaw:       fromRaw,
			EmailDate:     emailDate,
			FormattedDate: formattedDate,
			IsSpam:        isSpamMap[id],
			IsPolitical:   isPolitical,
			IsBlacklisted: isBlacklisted,
			HasICS:        hasICS,
			HasAttachment: hasAttachment,
			MessageID:     messageID,
			InReplyTo:     inReplyTo,
			References:    references,
		})
	}

	isBadEmail := func(e uicommon.FolderEmail) bool {
		return e.IsSpam || e.IsPolitical || e.IsBlacklisted
	}
	sort.Slice(emails, func(i, j int) bool {
		badI := isBadEmail(emails[i])
		badJ := isBadEmail(emails[j])
		if badI != badJ {
			return !badI
		}
		return emails[i].EmailDate.Before(emails[j].EmailDate)
	})

	senderWidth := 20
	if width < 50 {
		senderWidth = 10
	}
	idWidth := 8
	subjectWidth := width - senderWidth - idWidth - 2
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for idx, eml := range emails {
		isEven := (idx%2 == 0)

		senderPlain := uicommon.FormatSender(eml.FromRaw)
		if config.Verbose {
			senderPlain = uicommon.FormatSenderVerbose(eml.FromRaw)
		}
		senderPadded := padOrTruncate(senderPlain, senderWidth)
		senderState := "normal"
		if eml.IsBlacklisted {
			senderState = "blacklist"
		} else if eml.IsPolitical {
			senderState = "political"
		} else if eml.IsSpam {
			senderState = "spam"
		}
		senderDisp := getCellStyle(senderState, isEven).Sprint(senderPadded)

		subjectPlain := eml.Subject
		subjectState := "normal"
		if eml.IsBlacklisted {
			subjectPlain = "[BLACKLIST] " + subjectPlain
			subjectState = "blacklist"
		} else if eml.IsPolitical {
			subjectPlain = "[POLI] " + subjectPlain
			subjectState = "political"
		} else if eml.IsSpam {
			subjectPlain = "[SPAM] " + subjectPlain
			subjectState = "spam"
		}
		subjectPadded := padOrTruncate(subjectPlain, subjectWidth)
		subjectDisp := getCellStyle(subjectState, isEven).Sprint(subjectPadded)

		idPlain := email.ComputeShortID(eml.ID)
		idPadded := padOrTruncate(idPlain, idWidth)
		idDisp := getCellStyle("id", isEven).Sprint(idPadded)

		spaceDisp := " "

		fmt.Println(senderDisp + spaceDisp + subjectDisp + spaceDisp + idDisp)

		if config.Verbose {
			var details []string
			senderName, senderEmailAddr := uicommon.ParseSenderInfo(eml.FromRaw)
			if senderEmailAddr == "" {
				senderEmailAddr = eml.FromEmail
			}
			if senderName != "" {
				details = append(details, fmt.Sprintf("  Sender Name: %s", senderName))
			} else {
				details = append(details, "  Sender Name: (none)")
			}
			details = append(details, fmt.Sprintf("  Sender Email: %s", senderEmailAddr))

			if res, ok := scanResults[eml.ID]; ok && res != nil {
				details = append(details, fmt.Sprintf("  [Bogofilter Result: Score=%.2f, Action=%s]", res.Score, res.Action))
				if len(res.Symbols) > 0 {
					details = append(details, "  Symbols:")
					var symNames []string
					for name := range res.Symbols {
						symNames = append(symNames, name)
					}
					sort.Strings(symNames)
					for _, name := range symNames {
						sym := res.Symbols[name]
						desc := sym.Description
						if desc == "" {
							desc = "(no description)"
						}
						details = append(details, fmt.Sprintf("    - %s (score=%.2f): %s", sym.Name, sym.Score, desc))
					}
				} else {
					details = append(details, "  Symbols: none")
				}
			} else {
				details = append(details, "  [Bogofilter Result: no Bogofilter data cached or fetched]")
			}

			for _, detailLine := range details {
				detailDisp := getCellStyle("id", isEven).Sprint(detailLine)
				fmt.Println(detailDisp)
			}
		}
	}
	totalMessages := len(ids)
	var countStr string
	if totalCountBeforePattern > 0 {
		countStr = fmt.Sprintf("==    matched %d/%d ", totalMessages, totalCountBeforePattern)
	} else {
		countStr = fmt.Sprintf("==    %d messages ", totalMessages)
		if totalMessages == 1 {
			countStr = fmt.Sprintf("==    %d message ", totalMessages)
		}
	}
	if len(countStr) < width {
		countStr += strings.Repeat("=", width-len(countStr))
	}
	app.ColorBoldCyan.Println(countStr)
	if movedCount > 0 {
		fmt.Printf("Messages moved by rules: %d\n", movedCount)
	}
	fmt.Println()
}

func hasICSAttachment(downloadDir, msgID string) bool {
	data, err := msg.Read(downloadDir, msgID)
	if err != nil {
		return false
	}
	m, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return false
	}
	return email.HasICSAttachmentInMsg(m.Header, m.Body)
}

func hasGeneralAttachment(downloadDir, msgID string) bool {
	data, err := msg.Read(downloadDir, msgID)
	if err != nil {
		return false
	}
	m, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return false
	}
	return email.HasAttachmentInMsg(m.Header, m.Body)
}
