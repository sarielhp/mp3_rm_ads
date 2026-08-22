package tui

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/mail"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cache/msg"
	"mail_cli/cfg_acc"
	"mail_cli/uicommon"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type replyEditorFinishedMsg struct {
	err      error
	path     string
	targetID string
}

type emailSentMsg struct {
	err      error
	targetID string
}

func (m *tuiModel) replyCmd(targetID string, groupReply bool) tea.Cmd {
	cfg := m.client.Config()
	cd := cfg.DownloadDir

	data, err := msg.Read(cd, targetID)
	if err != nil {
		return func() tea.Msg {
			return replyEditorFinishedMsg{err: fmt.Errorf("email not cached locally: %w", err), targetID: targetID}
		}
	}

	origMsg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return func() tea.Msg {
			return replyEditorFinishedMsg{err: fmt.Errorf("failed to parse email: %w", err), targetID: targetID}
		}
	}

	tempPath, err := m.createDraftTemplate(origMsg, groupReply)
	if err != nil {
		return func() tea.Msg {
			return replyEditorFinishedMsg{err: fmt.Errorf("failed to create draft template: %w", err), targetID: targetID}
		}
	}

	editor := cfg.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "emacs"
	}

	var args []string
	if len(cfg.EditorArgs) > 0 {
		args = append(args, cfg.EditorArgs...)
	} else if editor == "emacs" {
		args = append(args, "-nw")
	}
	args = append(args, tempPath)

	c := exec.Command(editor, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return replyEditorFinishedMsg{
			err:      err,
			path:     tempPath,
			targetID: targetID,
		}
	})
}

func (m *tuiModel) createDraftTemplate(orig *mail.Message, groupReply bool) (string, error) {
	tempDir, err := app.GetTempDir()
	if err != nil {
		return "", err
	}
	tempPath := filepath.Join(tempDir, "reply.eml")
	tmp, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	var activeAcc cfg_acc.AccountConfig
	cfg := m.client.Config()
	if cfg != nil {
		for _, acc := range cfg.Accounts {
			if strings.EqualFold(acc.Name, cfg.SelectedAccount) {
				activeAcc = acc
				break
			}
		}
		if activeAcc.Name == "" && len(cfg.Accounts) > 0 {
			activeAcc = cfg.Accounts[0]
		}
	}

	fullName := ""
	if activeAcc.DisplayName != "" {
		fullName = activeAcc.DisplayName
	} else {
		currUser, err := user.Current()
		if err == nil && currUser.Name != "" {
			fullName = currUser.Name
		}
	}

	fromHeader := ""
	emailAddr := ""
	if activeAcc.Username != "" {
		emailAddr = activeAcc.Username
	} else if cfg != nil && cfg.Username != "" {
		emailAddr = cfg.Username
	}

	if emailAddr != "" {
		if fullName != "" {
			fromHeader = fmt.Sprintf("%s <%s>", fullName, emailAddr)
		} else {
			fromHeader = emailAddr
		}
	}

	dec := new(mime.WordDecoder)
	toAddress := orig.Header.Get("Reply-To")
	if toAddress == "" {
		toAddress = orig.Header.Get("From")
	}

	subject := orig.Header.Get("Subject")
	subjDec := uicommon.DecDecode(dec, subject)
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(subjDec)), "re:") {
		subjDec = "Re: " + subjDec
	}

	msgID := orig.Header.Get("Message-ID")
	origRefs := orig.Header.Get("References")
	inReplyTo := strings.TrimSpace(msgID)
	references := ""
	if origRefs != "" {
		references = strings.TrimSpace(origRefs) + " " + inReplyTo
	} else {
		references = inReplyTo
	}

	var ccAddresses []string
	seen := make(map[string]bool)

	ourEmail := strings.ToLower(strings.TrimSpace(m.client.Config().Username))
	seen[ourEmail] = true

	primaryTo, errTo := mail.ParseAddress(toAddress)
	if errTo == nil && primaryTo != nil {
		seen[strings.ToLower(primaryTo.Address)] = true
	} else {
		seen[strings.ToLower(toAddress)] = true
	}

	origTo := orig.Header.Get("To")
	origCc := orig.Header.Get("Cc")

	if groupReply {
		for _, hdrVal := range []string{origTo, origCc} {
			if hdrVal == "" {
				continue
			}
			addrs, err := mail.ParseAddressList(hdrVal)
			if err == nil {
				for _, addr := range addrs {
					emailLower := strings.ToLower(addr.Address)
					if !seen[emailLower] {
						seen[emailLower] = true
						ccAddresses = append(ccAddresses, addr.String())
					}
				}
			}
		}
	}

	if fromHeader != "" {
		fmt.Fprintf(tmp, "From: %s\n", fromHeader)
	}
	fmt.Fprintf(tmp, "To: %s\n", toAddress)
	if len(ccAddresses) > 0 {
		fmt.Fprintf(tmp, "Cc: %s\n", strings.Join(ccAddresses, ", "))
	}
	fmt.Fprintf(tmp, "Subject: %s\n", subjDec)
	if inReplyTo != "" {
		fmt.Fprintf(tmp, "In-Reply-To: %s\n", inReplyTo)
	}
	if references != "" {
		fmt.Fprintf(tmp, "References: %s\n", references)
	}
	fmt.Fprint(tmp, "\n")

	bodyText, _ := gmail.ExtractPlainBodyText(orig)
	bodyText = strings.ReplaceAll(bodyText, "\r\n", "\n")
	bodyText = strings.ReplaceAll(bodyText, "\r", "\n")

	origFrom := uicommon.DecDecode(dec, orig.Header.Get("From"))
	origDate := orig.Header.Get("Date")

	if bodyText != "" {
		fmt.Fprintf(tmp, "On %s, %s wrote:\n", origDate, origFrom)
		lines := strings.Split(bodyText, "\n")
		for _, line := range lines {
			fmt.Fprintf(tmp, "> %s\n", line)
		}
	}

	sig := readSignature()
	if sig != "" {
		fmt.Fprint(tmp, sig)
	}

	return tempPath, nil
}

func (m *tuiModel) saveFailedDraft(content []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	now := time.Now()
	dirPattern := filepath.Join(home, ".cache", "mail_cli", "drafts",
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
	)

	counter := 1
	if files, err := os.ReadDir(dirPattern); err == nil {
		maxNum := 0
		for _, f := range files {
			if f.IsDir() {
				var num int
				if _, err := fmt.Sscanf(f.Name(), "%d", &num); err == nil {
					if num > maxNum {
						maxNum = num
					}
				}
			}
		}
		counter = maxNum + 1
	}

	draftDir := filepath.Join(dirPattern, fmt.Sprintf("%d", counter))
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		return "", err
	}

	filePath := filepath.Join(draftDir, "draft.eml")
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return "", err
	}

	return filePath, nil
}

func (m *tuiModel) handleReplyEditorFinished(msg replyEditorFinishedMsg) (tea.Model, tea.Cmd) {
	defer os.RemoveAll(filepath.Dir(msg.path))

	if msg.err != nil {
		m.err = fmt.Errorf("editor session failed: %w", msg.err)
		m.showError = true
		return m, nil
	}

	editedBytes, err := os.ReadFile(msg.path)
	if err != nil {
		m.err = fmt.Errorf("failed to read draft file: %w", err)
		m.showError = true
		return m, nil
	}

	parsedMsg, err := mail.ReadMessage(bytes.NewReader(editedBytes))
	if err != nil {
		m.err = fmt.Errorf("edited draft lacks valid email format: %w", err)
		m.showError = true
		return m, nil
	}

	var finalBuf bytes.Buffer
	for k, values := range parsedMsg.Header {
		for _, v := range values {
			finalBuf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}

	if parsedMsg.Header.Get("MIME-Version") == "" {
		finalBuf.WriteString("MIME-Version: 1.0\r\n")
	}
	if parsedMsg.Header.Get("Content-Type") == "" {
		finalBuf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	}
	if parsedMsg.Header.Get("Content-Transfer-Encoding") == "" {
		finalBuf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	}
	finalBuf.WriteString("\r\n")

	bodyBytes, err := io.ReadAll(parsedMsg.Body)
	if err != nil {
		m.err = fmt.Errorf("failed to read body of edited email: %w", err)
		m.showError = true
		return m, nil
	}

	// Normalize body line endings to CRLF to prevent JMAP "bare newlines" rejection
	bodyStr := string(bodyBytes)
	bodyStr = strings.ReplaceAll(bodyStr, "\r\n", "\n")
	bodyStr = strings.ReplaceAll(bodyStr, "\n", "\r\n")
	finalBuf.WriteString(bodyStr)
	rawBytes := finalBuf.Bytes()

	m.confirmSend = true
	m.confirmSendBytes = rawBytes
	m.replyTargetID = msg.targetID
	return m, nil
}

func (m *tuiModel) kConfirmSend(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch s {
	case "y", "Y", "enter":
		m.confirmSend = false
		rawBytes := m.confirmSendBytes
		m.confirmSendBytes = nil
		targetID := m.replyTargetID
		m.replyTargetID = ""

		if m.isReadOnly() {
			m.err = fmt.Errorf("Sending is disabled in Read-Only mode")
			m.showError = true
			return m, nil
		}

		m.showLoad = "Sending email..."
		slog.Info("kConfirmSend: user confirmed send, dispatching SendEmail")
		return m, func() tea.Msg {
			err := m.client.SendEmail(rawBytes)
			if err != nil {
				if path, saveErr := m.saveFailedDraft(rawBytes); saveErr == nil {
					return emailSentMsg{err: fmt.Errorf("%w (saved copy to %s)", err, path), targetID: targetID}
				}
				return emailSentMsg{err: err, targetID: targetID}
			}
			return emailSentMsg{err: nil, targetID: targetID}
		}

	case "e", "E":
		rawBytes := m.confirmSendBytes
		m.confirmSend = false
		m.confirmSendBytes = nil
		return m.reEditDraft(rawBytes)

	case "n", "N", "esc", "q":
		rawBytes := m.confirmSendBytes
		m.confirmSend = false
		m.confirmSendBytes = nil

		if path, saveErr := m.saveFailedDraft(rawBytes); saveErr == nil {
			m.err = fmt.Errorf("sending cancelled; draft saved to %s", path)
			m.showError = true
		} else {
			m.err = fmt.Errorf("sending cancelled")
			m.showError = true
		}
		return m, nil
	}

	return m, nil
}

func renderConfirmSendDialog(m *tuiModel) string {
	msg, err := mail.ReadMessage(bytes.NewReader(m.confirmSendBytes))
	toStr := "unknown"
	subjStr := "(No Subject)"
	if err == nil {
		dec := new(mime.WordDecoder)
		toStr = uicommon.DecDecode(dec, msg.Header.Get("To"))
		subjStr = uicommon.DecDecode(dec, msg.Header.Get("Subject"))
	}

	dialogWidth := 50
	dialogHeight := 10

	var content strings.Builder
	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("         SEND EMAIL?         ") + "\n\n")
	content.WriteString(fmt.Sprintf("  To:      %s\n", truncateStr(toStr, dialogWidth-12)))
	content.WriteString(fmt.Sprintf("  Subject: %s\n\n", truncateStr(subjStr, dialogWidth-12)))
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("  [y] Send") + "    " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("  [e] Edit") + "    " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("  [n] Save draft & exit"))

	dialogBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight).
		Render(content.String())

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(dialogBox)
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		if maxLen > 3 {
			return s[:maxLen-3] + "..."
		}
		return s[:maxLen]
	}
	return s
}

func (m *tuiModel) reEditDraft(content []byte) (tea.Model, tea.Cmd) {
	tempDir, err := app.GetTempDir()
	if err != nil {
		m.err = err
		m.showError = true
		return m, nil
	}
	tempPath := filepath.Join(tempDir, "reply.eml")
	if err := os.WriteFile(tempPath, content, 0600); err != nil {
		m.err = err
		m.showError = true
		return m, nil
	}

	cfg := m.client.Config()
	editor := cfg.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "emacs"
	}

	var args []string
	if len(cfg.EditorArgs) > 0 {
		args = append(args, cfg.EditorArgs...)
	} else if editor == "emacs" {
		args = append(args, "-nw")
	}
	args = append(args, tempPath)

	c := exec.Command(editor, args...)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return replyEditorFinishedMsg{
			err:  err,
			path: tempPath,
		}
	})
}

func readSignature() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	sigPath := filepath.Join(home, ".signature")
	data, err := os.ReadFile(sigPath)
	if err != nil {
		return ""
	}
	sigText := string(data)
	if sigText != "" {
		return "\n-- \n" + strings.TrimSuffix(sigText, "\n") + "\n"
	}
	return ""
}
