package email

import (
	"net/mail"
	"strings"
	"testing"
)

func TestHasICSAttachmentInMsg(t *testing.T) {
	tests := []struct {
		name    string
		headers string
		body    string
		want    bool
	}{
		{
			name:    "No attachments, plain text",
			headers: "Content-Type: text/plain; charset=UTF-8\r\n",
			body:    "Hello world",
			want:    false,
		},
		{
			name:    "Direct text/calendar content type",
			headers: "Content-Type: text/calendar; method=REQUEST; charset=UTF-8\r\n",
			body:    "BEGIN:VCALENDAR\r\nEND:VCALENDAR",
			want:    true,
		},
		{
			name:    "Direct application/ics content type",
			headers: "Content-Type: application/ics\r\n",
			body:    "BEGIN:VCALENDAR\r\nEND:VCALENDAR",
			want:    true,
		},
		{
			name:    "Content-Type name parameter has .ics extension",
			headers: "Content-Type: application/octet-stream; name=\"invite.ics\"\r\n",
			body:    "some ics binary/text",
			want:    true,
		},
		{
			name:    "Content-Disposition filename has .ics extension",
			headers: "Content-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"meeting.ICS\"\r\n",
			body:    "some ics binary/text",
			want:    true,
		},
		{
			name:    "Multipart containing text/calendar",
			headers: "Content-Type: multipart/alternative; boundary=\"boundary123\"\r\n",
			body: "--boundary123\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"Plain text\r\n" +
				"--boundary123\r\n" +
				"Content-Type: text/calendar; method=PUBLISH\r\n\r\n" +
				"BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n" +
				"--boundary123--\r\n",
			want: true,
		},
		{
			name:    "Multipart containing ics attachment file",
			headers: "Content-Type: multipart/mixed; boundary=\"boundary456\"\r\n",
			body: "--boundary456\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"Hello\r\n" +
				"--boundary456\r\n" +
				"Content-Type: application/octet-stream; name=\"meeting.ics\"\r\n" +
				"Content-Disposition: attachment; filename=\"meeting.ics\"\r\n\r\n" +
				"BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n" +
				"--boundary456--\r\n",
			want: true,
		},
		{
			name:    "Multipart without ics",
			headers: "Content-Type: multipart/mixed; boundary=\"boundary789\"\r\n",
			body: "--boundary789\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"Hello\r\n" +
				"--boundary789\r\n" +
				"Content-Type: image/png; name=\"logo.png\"\r\n" +
				"Content-Disposition: attachment; filename=\"logo.png\"\r\n\r\n" +
				"imagedata\r\n" +
				"--boundary789--\r\n",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMsg := tt.headers + "\r\n" + tt.body
			msg, err := mail.ReadMessage(strings.NewReader(rawMsg))
			if err != nil {
				t.Fatalf("failed to parse test message: %v", err)
			}
			got := HasICSAttachmentInMsg(msg.Header, msg.Body)
			if got != tt.want {
				t.Errorf("HasICSAttachmentInMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractAttachments(t *testing.T) {
	rawMsg := "Content-Type: multipart/mixed; boundary=\"boundary456\"\r\n\r\n" +
		"--boundary456\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Hello\r\n" +
		"--boundary456\r\n" +
		"Content-Type: application/octet-stream; name=\"meeting.ics\"\r\n" +
		"Content-Disposition: attachment; filename=\"meeting.ics\"\r\n\r\n" +
		"BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n" +
		"--boundary456\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Disposition: attachment; filename=\"logo.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"dGVzdGRhdGE=\r\n" +
		"--boundary456--\r\n"

	atts, err := ExtractAttachments([]byte(rawMsg))
	if err != nil {
		t.Fatalf("ExtractAttachments failed: %v", err)
	}

	if len(atts) != 2 {
		t.Fatalf("Expected 2 attachments, got %d", len(atts))
	}

	if atts[0].Filename != "meeting.ics" {
		t.Errorf("Expected first filename 'meeting.ics', got %q", atts[0].Filename)
	}
	if string(atts[0].Data) != "BEGIN:VCALENDAR\r\nEND:VCALENDAR" {
		t.Errorf("Expected first data 'BEGIN:VCALENDAR\r\nEND:VCALENDAR', got %q", string(atts[0].Data))
	}

	if atts[1].Filename != "logo.png" {
		t.Errorf("Expected second filename 'logo.png', got %q", atts[1].Filename)
	}
	if string(atts[1].Data) != "testdata" {
		t.Errorf("Expected second data 'testdata', got %q", string(atts[1].Data))
	}
}

func TestHasAttachmentInMsg(t *testing.T) {
	tests := []struct {
		name    string
		headers string
		body    string
		want    bool
	}{
		{
			name:    "Plain text, no attachment",
			headers: "Content-Type: text/plain; charset=UTF-8\r\n",
			body:    "Hello world",
			want:    false,
		},
		{
			name:    "Attachment via Content-Disposition",
			headers: "Content-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"doc.pdf\"\r\n",
			body:    "pdf data",
			want:    true,
		},
		{
			name:    "Attachment via Content-Type name param",
			headers: "Content-Type: application/zip; name=\"archive.zip\"\r\n",
			body:    "zip data",
			want:    true,
		},
		{
			name:    "Multipart containing attachment",
			headers: "Content-Type: multipart/mixed; boundary=\"bound1\"\r\n",
			body: "--bound1\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"Message body\r\n" +
				"--bound1\r\n" +
				"Content-Type: image/png\r\n" +
				"Content-Disposition: attachment; filename=\"pic.png\"\r\n\r\n" +
				"img data\r\n" +
				"--bound1--\r\n",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.headers + "\r\n" + tt.body
			msg, err := mail.ReadMessage(strings.NewReader(raw))
			if err != nil {
				t.Fatalf("failed to parse message: %v", err)
			}
			got := HasAttachmentInMsg(msg.Header, msg.Body)
			if got != tt.want {
				t.Errorf("HasAttachmentInMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}
