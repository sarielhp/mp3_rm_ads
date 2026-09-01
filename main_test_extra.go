package main

import (
	"testing"
)

func TestEqualMergedIntervals(t *testing.T) {
	a := []MergedCutInterval{{1, 2}, {3, 4}}
	b := []MergedCutInterval{{1, 2}, {3, 4}}
	c := []MergedCutInterval{{1, 2}, {5, 6}}
	if !equalMergedIntervals(a, b) {
		t.Error("equal should be true")
	}
	if equalMergedIntervals(a, c) {
		t.Error("different should be false")
	}
}

func TestTrimSpace(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"  hello  ", "hello"}, {"hello", "hello"}, {"  ", ""}, {"", ""},
	}
	for _, c := range cases {
		if got := trimSpace(c.in); got != c.expected {
			t.Errorf("trimSpace(%q) = %q", c.in, got)
		}
	}
}

func TestCleanModelName(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"openrouter/a/b", "a/b"}, {"a/b", "a/b"}, {"m:latest", "m"},
	}
	for _, c := range cases {
		if got := cleanModelName(c.in); got != c.expected {
			t.Errorf("cleanModelName(%q) = %q", c.in, got)
		}
	}
}

func TestRoundFloat(t *testing.T) {
	cases := []struct {
		in       float64
		p        int
		expected float64
	}{
		{1.234, 2, 1.23}, {1.235, 2, 1.24},
	}
	for _, c := range cases {
		if got := roundFloat(c.in, c.p); got != c.expected {
			t.Errorf("roundFloat(%f,%d) = %f", c.in, c.p, got)
		}
	}
}

func TestExtractHost(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"http://localhost:8080/i", "localhost"},
		{"https://openrouter.ai/api", "openrouter.ai"},
		{"http://1.2.3.4:11434", "1.2.3.4"},
	}
	for _, c := range cases {
		if got := extractHost(c.in); got != c.expected {
			t.Errorf("extractHost(%q) = %q", c.in, got)
		}
	}
}

func TestExtractPort(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"http://h:8080/i", "8080"}, {"https://h", "443"}, {"http://h", "80"},
	}
	for _, c := range cases {
		if got := extractPort(c.in); got != c.expected {
			t.Errorf("extractPort(%q) = %q", c.in, got)
		}
	}
}

func TestIsLocalHost(t *testing.T) {
	if !isLocalHost("localhost") || !isLocalHost("127.0.0.1") || isLocalHost("x.com") {
		t.Error("isLocalHost failed")
	}
}

func TestSplitLines(t *testing.T) {
	r := splitLines("a\nb\nc")
	if len(r) != 3 || r[0] != "a" || r[1] != "b" || r[2] != "c" {
		t.Error("splitLines failed")
	}
}

func TestToLower(t *testing.T) {
	if toLower("HELLO") != "hello" {
		t.Error("toLower failed")
	}
}

func TestMatchFailedDecode(t *testing.T) {
	if !matchFailedDecode("failed to decode") || matchFailedDecode("ok") {
		t.Error("matchFailedDecode failed")
	}
}

func TestRepeatStr(t *testing.T) {
	if repeatStr("ab", 3) != "ababab" {
		t.Error("repeatStr failed")
	}
}

func TestMergeSegments(t *testing.T) {
	s := []TranscriptionSegment{{0, 5, "h", "", nil}, {5.2, 10, "w", "", nil}, {20, 25, "f", "", nil}}
	r := mergeSegments(s)
	if len(r) != 2 {
		t.Errorf("got %d segments, want 2", len(r))
	}
}

func TestPutUint16(t *testing.T) {
	b := make([]byte, 2)
	putUint16(b, 0x1234)
	if b[0] != 0x34 || b[1] != 0x12 {
		t.Error("putUint16 failed")
	}
}

func TestPutUint32(t *testing.T) {
	b := make([]byte, 4)
	putUint32(b, 0x12345678)
	if b[0] != 0x78 || b[1] != 0x56 || b[2] != 0x34 || b[3] != 0x12 {
		t.Error("putUint32 failed")
	}
}

func TestEnvOr(t *testing.T) {
	orig := envGetenv
	defer func() { envGetenv = orig }()
	envGetenv = func(k string) string {
		if k == "K" {
			return "v"
		}
		return ""
	}
	if envOr("K", "d") != "v" || envOr("X", "d") != "d" {
		t.Error("envOr failed")
	}
}
