package email

import (
	"io"
	"mime"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"
)

func ParseReader(r io.Reader, id string, path string) *Email {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil
	}
	dec := new(mime.WordDecoder)
	subject := DecodeHeader(dec, msg.Header.Get("Subject"))
	if subject == "" {
		subject = "(No Subject)"
	}
	from := ParseEmailAddress(msg.Header.Get("From"))
	if from == "" {
		from = "(Unknown Sender)"
	}
	fromRaw := strings.TrimSpace(msg.Header.Get("From"))
	if fromRaw == "" {
		fromRaw = "(Unknown Sender)"
	}
	dateStr := msg.Header.Get("Date")
	var t time.Time
	if dateStr != "" {
		if parsed, err := mail.ParseDate(dateStr); err == nil {
			t = parsed
		}
	}
	if t.IsZero() {
		if info, err := os.Stat(path); err == nil {
			t = info.ModTime()
		}
	}
	hasAttachment := HasAttachmentInMsg(msg.Header, msg.Body)
	hasICS := HasICSAttachmentInMsg(msg.Header, msg.Body)
	return &Email{
		ID:            id,
		Subject:       subject,
		FromEmail:     from,
		FromRaw:       fromRaw,
		EmailDate:     t,
		FormattedDate: FormatEmailDate(t),
		HasAttachment: hasAttachment,
		HasICS:        hasICS,
		MessageID:     strings.TrimSpace(msg.Header.Get("Message-ID")),
		InReplyTo:     strings.TrimSpace(msg.Header.Get("In-Reply-To")),
		References:    strings.TrimSpace(msg.Header.Get("References")),
	}
}

func MatchPattern(subject, pattern string) bool {
	subject = strings.ToLower(subject)
	pattern = strings.ToLower(pattern)
	if !strings.ContainsAny(pattern, "*?[") {
		return strings.Contains(subject, pattern)
	}
	var buf strings.Builder
	buf.WriteString("(?i)^")
	for _, r := range pattern {
		switch r {
		case '*':
			buf.WriteString(".*")
		case '?':
			buf.WriteString(".")
		case '(', ')', '$', '^', '+', '.', '{', '}', '|', '\\':
			buf.WriteString("\\" + string(r))
		default:
			buf.WriteString(string(r))
		}
	}
	buf.WriteString("$")
	reg, err := regexp.Compile(buf.String())
	if err != nil {
		return strings.Contains(subject, pattern)
	}
	return reg.MatchString(subject)
}
