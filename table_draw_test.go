package main

import (
	"strings"
	"testing"
)

func TestStringDisplayWidth(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"1bcef", 5},
		{"\x1b[1;36m1bcef\x1b[0m", 5},
		{"5-4", 3},
		{"🎙️", 2},
		{"✨", 2},
		{"⬇️", 2},
		{"✂️", 2},
		{"⏳", 2},
		{"📅", 2},
		{"Top 3", 5},
		{"2026-09-08", 10},
		{"שלום", 4},
	}

	for _, tc := range cases {
		got := stringDisplayWidth(tc.input)
		if got != tc.want {
			t.Errorf("stringDisplayWidth(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestPadCell(t *testing.T) {
	left := padCell("abc", 5, AlignLeft)
	if left != "abc  " {
		t.Errorf("AlignLeft want %q, got %q", "abc  ", left)
	}

	right := padCell("abc", 5, AlignRight)
	if right != "  abc" {
		t.Errorf("AlignRight want %q, got %q", "  abc", right)
	}

	center := padCell("ab", 6, AlignCenter)
	if center != "  ab  " {
		t.Errorf("AlignCenter want %q, got %q", "  ab  ", center)
	}

	emojiCenter := padCell("✨", 4, AlignCenter)
	if stringDisplayWidth(emojiCenter) != 4 {
		t.Errorf("emojiCenter width want 4, got %d (%q)", stringDisplayWidth(emojiCenter), emojiCenter)
	}
}

func TestRenderTableBordersConnected(t *testing.T) {
	cols := []TableColumn{
		{Header: "ID", Width: 5, Align: AlignCenter},
		{Header: "Title", Width: 10, Align: AlignLeft},
		{Header: "✨", Width: 4, Align: AlignCenter},
	}

	top := renderTableTop(cols)
	hdr := renderTableHeader(cols)
	div := renderTableDivider(cols)
	row := renderTableRow([]string{"12345", "Test Title", "0"}, cols)
	bot := renderTableBottom(cols)

	expectedWidth := stringDisplayWidth(top)
	if stringDisplayWidth(hdr) != expectedWidth {
		t.Errorf("hdr width %d != top width %d", stringDisplayWidth(hdr), expectedWidth)
	}
	if stringDisplayWidth(div) != expectedWidth {
		t.Errorf("div width %d != top width %d", stringDisplayWidth(div), expectedWidth)
	}
	if stringDisplayWidth(row) != expectedWidth {
		t.Errorf("row width %d != top width %d", stringDisplayWidth(row), expectedWidth)
	}
	if stringDisplayWidth(bot) != expectedWidth {
		t.Errorf("bot width %d != top width %d", stringDisplayWidth(bot), expectedWidth)
	}

	if !strings.HasPrefix(top, "┌") || !strings.HasSuffix(top, "┐") {
		t.Errorf("invalid top border: %s", top)
	}
	if !strings.HasPrefix(div, "├") || !strings.HasSuffix(div, "┤") {
		t.Errorf("invalid divider border: %s", div)
	}
	if !strings.HasPrefix(bot, "└") || !strings.HasSuffix(bot, "┘") {
		t.Errorf("invalid bottom border: %s", bot)
	}
	if !strings.HasPrefix(row, "│") || !strings.HasSuffix(row, "│") {
		t.Errorf("invalid row border: %s", row)
	}
}

func TestCompactDownloadPolicy(t *testing.T) {
	if compactDownloadPolicy("latest", 0) != "New" {
		t.Errorf("expected 'New'")
	}
	if compactDownloadPolicy("all", 0) != "All" {
		t.Errorf("expected 'All'")
	}
	if compactDownloadPolicy("none", 0) != "Off" {
		t.Errorf("expected 'Off'")
	}
	if compactDownloadPolicy("latest_k", 5) != "Top 5" {
		t.Errorf("expected 'Top 5'")
	}
}

func TestCompactAdRemoval(t *testing.T) {
	if compactAdRemoval("all") != "All" {
		t.Errorf("expected 'All'")
	}
	if compactAdRemoval("none") != "Off" {
		t.Errorf("expected 'Off'")
	}
	if compactAdRemoval("latest") != "New" {
		t.Errorf("expected 'New'")
	}
}
