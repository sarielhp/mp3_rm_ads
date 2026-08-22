package ui

import (
	"fmt"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/uicommon"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
)

// padOrTruncate pads or truncates string s to the given terminal width.
func padOrTruncate(s string, width int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	currentWidth := runewidth.StringWidth(s)
	if currentWidth == width {
		return s
	}
	if currentWidth > width {
		var sb strings.Builder
		w := 0
		for _, r := range s {
			rw := runewidth.RuneWidth(r)
			if w+rw > width {
				break
			}
			sb.WriteRune(r)
			w += rw
		}
		remainder := width - w
		if remainder > 0 {
			sb.WriteString(strings.Repeat(" ", remainder))
		}
		return sb.String()
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// getCellStyle returns the terminal color style based on the email state and row parity.
func getCellStyle(state string, isEven bool) *color.Color {
	var style *color.Color
	switch state {
	case "blacklist":
		style = color.New(color.FgHiYellow, color.Bold, color.CrossedOut)
	case "political":
		style = color.RGB(255, 127, 127).Add(color.Bold)
	case "spam":
		style = color.New(color.FgHiYellow, color.Bold)
	case "id":
		style = color.New(color.FgHiBlack)
	case "folder":
		style = color.New(color.FgHiBlue)
	default:
		if !isEven {
			style = color.RGB(255, 255, 170)
		} else {
			style = color.New(color.FgWhite)
		}
	}
	return style
}

// PrintEmailsWithFolders prints a summary of emails along with their source folders.
func PrintEmailsWithFolders(title string, emails []uicommon.FolderEmail, folderMap map[string]string, config *cfg_g.Config) {
	width := app.GetTerminalWidth()
	separator := strings.Repeat("=", width)

	app.ColorBoldCyan.Println(title)
	app.ColorBoldCyan.Println(separator)

	folderWidth := 15
	if width < 70 {
		folderWidth = 10
	}
	senderWidth := 20
	if width < 50 {
		senderWidth = 10
	}
	idWidth := 8
	subjectWidth := width - folderWidth - senderWidth - idWidth - 3
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for idx, eml := range emails {
		isEven := (idx%2 == 0)

		folderName := folderMap[eml.ID]
		if folderName == "" {
			folderName = "(Unknown)"
		}
		folderPadded := padOrTruncate(folderName, folderWidth)
		folderDisp := getCellStyle("folder", isEven).Sprint(folderPadded)

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

		fmt.Println(folderDisp + spaceDisp + senderDisp + spaceDisp + subjectDisp + spaceDisp + idDisp)
	}

	totalMessages := len(emails)
	countStr := fmt.Sprintf("==    %d messages ", totalMessages)
	if totalMessages == 1 {
		countStr = fmt.Sprintf("==    %d message ", totalMessages)
	}
	if len(countStr) < width {
		countStr += strings.Repeat("=", width-len(countStr))
	}
	app.ColorBoldCyan.Println(countStr)
	fmt.Println()
}
