package main

import (
	"testing"
)

func TestFormatTime(t *testing.T) {
	cases := []struct {
		in       float64
		expected string
	}{
		{0, "00:00.0"}, {30, "00:30.0"}, {60, "01:00.0"},
		{90.5, "01:30.5"}, {3600, "01:00:00.0"},
	}
	for _, c := range cases {
		got := formatTime(c.in)
		if got != c.expected {
			t.Errorf("formatTime(%f) = %q, want %q", c.in, got, c.expected)
		}
	}
}

func TestFormatClock(t *testing.T) {
	cases := []struct {
		in       float64
		expected string
	}{
		{0, "00:00"}, {30, "00:30"}, {60, "01:00"}, {3661, "01:01:01"},
	}
	for _, c := range cases {
		got := formatClock(c.in)
		if got != c.expected {
			t.Errorf("formatClock(%f) = %q, want %q", c.in, got, c.expected)
		}
	}
}

func TestFormatSRTTime(t *testing.T) {
	cases := []struct {
		in       float64
		expected string
	}{
		{0, "00:00:00,000"}, {1.5, "00:00:01,500"}, {65.123, "00:01:05,123"},
	}
	for _, c := range cases {
		got := formatSRTTime(c.in)
		if got != c.expected {
			t.Errorf("formatSRTTime(%f) = %q, want %q", c.in, got, c.expected)
		}
	}
}

func TestDetectScriptLanguage(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"", ""}, {"Hello", ""}, {"שלום עולם", "he"},
		{"مرحبا", "ar"}, {"Привет", "ru"}, {"Γειά", "el"},
	}
	for _, c := range cases {
		got := detectScriptLanguage(c.in)
		if got != c.expected {
			t.Errorf("detectScriptLanguage(%q) = %q, want %q", c.in, got, c.expected)
		}
	}
}

func TestCountWords(t *testing.T) {
	cases := []struct {
		in       string
		expected int
	}{
		{"", 0}, {"hello", 1}, {"hello world", 2}, {"  a   b  ", 2},
	}
	for _, c := range cases {
		got := countWords(c.in)
		if got != c.expected {
			t.Errorf("countWords(%q) = %d, want %d", c.in, got, c.expected)
		}
	}
}

func TestMergeIntervals(t *testing.T) {
	cases := []struct {
		name     string
		input    []AdSegment
		expected []AdSegment
	}{
		{"empty", nil, nil},
		{"no overlap", []AdSegment{{0, 10, "a"}, {20, 30, "b"}}, []AdSegment{{0, 10, "a"}, {20, 30, "b"}}},
		{"adjacent gap", []AdSegment{{0, 10, "a"}, {12, 20, "b"}}, []AdSegment{{0, 10, "a"}, {12, 20, "b"}}},
		{"overlap", []AdSegment{{0, 15, "a"}, {10, 20, "b"}}, []AdSegment{{0, 20, "a"}}},
		{"large gap", []AdSegment{{0, 5, "a"}, {20, 25, "b"}}, []AdSegment{{0, 5, "a"}, {20, 25, "b"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeIntervals(c.input)
			if len(got) != len(c.expected) {
				t.Fatalf("len %d != %d", len(got), len(c.expected))
			}
			for i := range got {
				if got[i].Start != c.expected[i].Start || got[i].End != c.expected[i].End {
					t.Errorf("[%d] = {%f,%f}, want {%f,%f}", i, got[i].Start, got[i].End, c.expected[i].Start, c.expected[i].End)
				}
			}
		})
	}
}

func TestCalculateKeepSegments(t *testing.T) {
	cases := []struct {
		name     string
		dur      float64
		ads      []AdSegment
		expected [][2]float64
	}{
		{"no ads", 100, nil, [][2]float64{{0, 100}}},
		{"middle", 100, []AdSegment{{20, 30, ""}}, [][2]float64{{0, 20}, {30, 100}}},
		{"start", 100, []AdSegment{{0, 10, ""}}, [][2]float64{{10, 100}}},
		{"end", 100, []AdSegment{{90, 100, ""}}, [][2]float64{{0, 90}}},
		{"multiple", 100, []AdSegment{{10, 20, ""}, {50, 60, ""}}, [][2]float64{{0, 10}, {20, 50}, {60, 100}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := calculateKeepSegments(c.dur, c.ads)
			if len(got) != len(c.expected) {
				t.Fatalf("len %d != %d", len(got), len(c.expected))
			}
			for i := range got {
				if got[i][0] != c.expected[i][0] || got[i][1] != c.expected[i][1] {
					t.Errorf("[%d] = {%f,%f}, want {%f,%f}", i, got[i][0], got[i][1], c.expected[i][0], c.expected[i][1])
				}
			}
		})
	}
}

func TestBuildWavHeader(t *testing.T) {
	h := buildWavHeader(1000)
	if string(h[0:4]) != "RIFF" || string(h[8:12]) != "WAVE" || len(h) != 44 {
		t.Error("invalid WAV header")
	}
}

func TestExtractJSONArray(t *testing.T) {
	cases := []struct {
		name, in string
		expected []AdSegment
	}{
		{"simple", `[{"start":10,"end":20,"reason":"t"}]`, []AdSegment{{10, 20, "t"}}},
		{"empty", `[]`, nil},
		{"no array", `no json`, nil},
		{"with text", `x [{"start":1,"end":2}]`, []AdSegment{{1, 2, ""}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractJSONArray(c.in)
			if len(got) != len(c.expected) {
				t.Fatalf("len %d != %d", len(got), len(c.expected))
			}
			for i := range got {
				if got[i].Start != c.expected[i].Start || got[i].End != c.expected[i].End {
					t.Errorf("[%d] = {%f,%f}", i, got[i].Start, got[i].End)
				}
			}
		})
	}
}

func TestWorkDirFor(t *testing.T) {
	if got := workDirFor("/p/f.mp3"); got != "/p/.work" {
		t.Errorf("got %q", got)
	}
}

func TestStripExt(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"f.mp3", "f"}, {"p/f.mp3", "p/f"}, {"f", "f"},
	}
	for _, c := range cases {
		if got := stripExt(c.in); got != c.expected {
			t.Errorf("stripExt(%q) = %q", c.in, got)
		}
	}
}

func TestFilepathBase(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"/a/b.mp3", "b.mp3"}, {"f.mp3", "f.mp3"},
	}
	for _, c := range cases {
		if got := filepathBase(c.in); got != c.expected {
			t.Errorf("filepathBase(%q) = %q", c.in, got)
		}
	}
}

func TestSortAds(t *testing.T) {
	a := []AdSegment{{10, 20, ""}, {0, 5, ""}, {5, 10, ""}}
	sortAds(a)
	if a[0].Start != 0 || a[1].Start != 5 || a[2].Start != 10 {
		t.Error("sortAds failed")
	}
}

func TestMergeBounds(t *testing.T) {
	cases := []struct {
		name     string
		in       [][2]float64
		expected [][2]float64
	}{
		{"empty", nil, nil},
		{"single", [][2]float64{{0, 10}}, [][2]float64{{0, 10}}},
		{"adjacent", [][2]float64{{0, 10}, {12, 20}}, [][2]float64{{0, 10}, {12, 20}}},
		{"large gap", [][2]float64{{0, 5}, {20, 25}}, [][2]float64{{0, 5}, {20, 25}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeBounds(c.in)
			if len(got) != len(c.expected) {
				t.Fatalf("len %d != %d", len(got), len(c.expected))
			}
			for i := range got {
				if got[i][0] != c.expected[i][0] || got[i][1] != c.expected[i][1] {
					t.Errorf("[%d] = {%f,%f}", i, got[i][0], got[i][1])
				}
			}
		})
	}
}

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
