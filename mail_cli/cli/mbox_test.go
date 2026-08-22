package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteToMbox_ValidAddress(t *testing.T) {
	var buf bytes.Buffer
	fromAddr := "test@example.com"
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rawEmail := []byte("Subject: Test\r\n\r\nBody")

	err := writeToMbox(&buf, rawEmail, fromAddr, date)
	if err != nil {
		t.Fatalf("writeToMbox failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "From test@example.com") {
		t.Errorf("Expected 'From test@example.com', got: %s", output[:50])
	}
}

func TestWriteToMbox_EmptyAddress(t *testing.T) {
	var buf bytes.Buffer
	fromAddr := ""
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rawEmail := []byte("Subject: Test\r\n\r\nBody")

	err := writeToMbox(&buf, rawEmail, fromAddr, date)
	if err != nil {
		t.Fatalf("writeToMbox failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "From unknown@example.com") {
		t.Errorf("Expected 'From unknown@example.com', got: %s", output[:50])
	}
}

func TestWriteToMbox_InvalidAddress(t *testing.T) {
	var buf bytes.Buffer
	fromAddr := "invalid address without @"
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rawEmail := []byte("Subject: Test\r\n\r\nBody")

	err := writeToMbox(&buf, rawEmail, fromAddr, date)
	if err != nil {
		t.Fatalf("writeToMbox failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "From invalid@example.com") {
		t.Errorf("Expected 'From invalid@example.com', got: %s", output[:50])
	}
}

func TestWriteToMbox_WithFromLine(t *testing.T) {
	var buf bytes.Buffer
	fromAddr := "test@example.com"
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rawEmail := []byte("From original@example.com\r\n\r\nBody")

	err := writeToMbox(&buf, rawEmail, fromAddr, date)
	if err != nil {
		t.Fatalf("writeToMbox failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, ">From original@example.com") {
		t.Errorf("Expected escaped 'From' line, got: %s", output)
	}
}

func TestReadFromMbox_Valid(t *testing.T) {
	mboxContent := `From user1@example.com Mon Jan  2 15:04:05 2006
Subject: Test 1

Body 1
From user2@example.com Tue Jan  3 16:04:05 2006
Subject: Test 2

Body 2
`

	msgs, err := readFromMbox(strings.NewReader(mboxContent))
	if err != nil {
		t.Fatalf("readFromMbox failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].From != "user1@example.com" {
		t.Errorf("Expected 'user1@example.com', got '%s'", msgs[0].From)
	}

	if msgs[1].From != "user2@example.com" {
		t.Errorf("Expected 'user2@example.com', got '%s'", msgs[1].From)
	}
}

func TestReadFromMbox_EmptyFrom(t *testing.T) {
	mboxContent := `From unknown Mon Jan  2 15:04:05 2006
Subject: Test

Body
`

	msgs, err := readFromMbox(strings.NewReader(mboxContent))
	if err != nil {
		t.Fatalf("readFromMbox failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if msgs[0].From != "unknown" {
		t.Errorf("Expected 'unknown', got '%s'", msgs[0].From)
	}
}

func TestReadFromMbox_Malformed(t *testing.T) {
	mboxContent := `Subject: Test

Body without From line
`

	msgs, err := readFromMbox(strings.NewReader(mboxContent))
	if err != nil {
		t.Fatalf("readFromMbox failed: %v", err)
	}

	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages (no From line), got %d", len(msgs))
	}
}

func TestReadFromMbox_EscapedFrom(t *testing.T) {
	mboxContent := `From user@example.com Mon Jan  2 15:04:05 2006
Subject: Test

>From escaped@example.com
Body
`

	msgs, err := readFromMbox(strings.NewReader(mboxContent))
	if err != nil {
		t.Fatalf("readFromMbox failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if !strings.Contains(string(msgs[0].Body), "From escaped@example.com") {
		t.Errorf("Expected unescaped 'From' line in body")
	}
}
