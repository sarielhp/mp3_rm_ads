package main

import (
	"os"
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

func TestFindMP3Files(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(d+"/a.mp3", []byte("x"), 0644)
	os.WriteFile(d+"/a.txt", []byte("x"), 0644)
	os.MkdirAll(d+"/sub", 0755)
	os.WriteFile(d+"/sub/b.mp3", []byte("x"), 0644)
	files := findMP3Files(d)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}

func TestSafeMove(t *testing.T) {
	d := t.TempDir()
	src, dst := d+"/s.txt", d+"/d.txt"
	os.WriteFile(src, []byte("x"), 0644)
	safeMove(src, dst)
	if fileExists(src) || !fileExists(dst) {
		t.Error("safeMove failed")
	}
}

func TestCopyFile(t *testing.T) {
	d := t.TempDir()
	src, dst := d+"/s.txt", d+"/d.txt"
	os.WriteFile(src, []byte("x"), 0644)
	copyFile(src, dst)
	if !fileExists(dst) {
		t.Error("copyFile failed")
	}
}

func TestSelectProfile(t *testing.T) {
	cfg := Config{ActiveProfileID: 2, Profiles: []LLMProfile{{1, "One", "", "", "m1", ""}, {2, "Two", "", "", "m2", ""}}}
	if p := selectProfile(cfg, ""); p.ID != 2 {
		t.Error("default profile")
	}
	if p := selectProfile(cfg, "1"); p.ID != 1 {
		t.Error("by id")
	}
	if p := selectProfile(cfg, "Two"); p.ID != 2 {
		t.Error("by name")
	}
}

func TestAllTokensMatch(t *testing.T) {
	if !allTokensMatch("a-b", []string{"a", "b"}) {
		t.Error("should match")
	}
	if allTokensMatch("a-b", []string{"a", "x"}) {
		t.Error("should not match")
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello", []string{"hello"}) {
		t.Error("should find")
	}
	if containsAny("hello", []string{"xyz"}) {
		t.Error("should not find")
	}
}

func TestIsLetter(t *testing.T) {
	if !isLetter('a') || isLetter('1') || !isLetter(0x05D0) {
		t.Error("isLetter failed")
	}
}

func TestParseFloat(t *testing.T) {
	if parseFloat("3.14") != 3.14 || parseFloat("x") != 0 {
		t.Error("parseFloat failed")
	}
}

func TestReplaceIP(t *testing.T) {
	r := replaceIP("http://192.168.1.230:8080/t", "10.0.0.1")
	if r != "http://10.0.0.1:8080/t" {
		t.Errorf("got %q", r)
	}
}

func TestMatchProgressHMS(t *testing.T) {
	h, m, s, ok := matchProgressHMS("processing audio (01:02:03.456 -> 04:05:06.789)")
	if !ok || h != 1 || m != 2 || s != 3 {
		t.Error("matchProgressHMS failed")
	}
}

func TestMatchProgressPercent(t *testing.T) {
	p, ok := matchProgressPercent("progress_common: 45.5%")
	if !ok || p != 45.5 {
		t.Error("matchProgressPercent failed")
	}
}

func TestSortSegments(t *testing.T) {
	s := []TranscriptionSegment{{10, 20, "", "", nil}, {0, 5, "", "", nil}, {5, 10, "", "", nil}}
	sortSegments(s)
	if s[0].Start != 0 || s[1].Start != 5 || s[2].Start != 10 {
		t.Error("sortSegments failed")
	}
}

func TestSortBounds(t *testing.T) {
	b := [][2]float64{{10, 20}, {0, 5}, {5, 10}}
	sortBounds(b)
	if b[0][0] != 0 || b[1][0] != 5 || b[2][0] != 10 {
		t.Error("sortBounds failed")
	}
}

func TestContainsStr(t *testing.T) {
	if !containsStr("hello", "ell") {
		t.Error("should find")
	}
	if containsStr("hello", "xyz") {
		t.Error("should not find")
	}
}

func TestAllTokensMatchEmpty(t *testing.T) {
	if !allTokensMatch("x", nil) {
		t.Error("should match nil")
	}
}

func TestCleanModelNameEmpty(t *testing.T) {
	if cleanModelName("") != "" {
		t.Error("should be empty")
	}
}

func TestContainsStrEmpty(t *testing.T) {
	if !containsStr("hello", "") {
		t.Error("should be true - empty string is contained in any string")
	}
}

func TestContainsAnyEmpty(t *testing.T) {
	if containsAny("hello", nil) {
		t.Error("should be false")
	}
}

func TestIsLetterCJK(t *testing.T) {
	if !isLetter(0x4E00) {
		t.Error("CJK should be letter")
	}
}

func TestIsLetterHiragana(t *testing.T) {
	if !isLetter(0x3042) {
		t.Error("hiragana should be letter")
	}
}

func TestIsLetterKatakana(t *testing.T) {
	if !isLetter(0x30A2) {
		t.Error("katakana should be letter")
	}
}

func TestIsLetterGlagolitic(t *testing.T) {
	if !isLetter(0x2C00) {
		t.Error("glagolitic should be letter")
	}
}

func TestIsLetterLatinExtended(t *testing.T) {
	if !isLetter(0x1E00) {
		t.Error("latin extended should be letter")
	}
}

func TestDetectScriptLanguageMixed(t *testing.T) {
	if r := detectScriptLanguage("שלום hello שלום"); r != "he" {
		t.Errorf("got %q", r)
	}
}

func TestDetectScriptLanguageNoMatch(t *testing.T) {
	if r := detectScriptLanguage("123"); r != "" {
		t.Errorf("got %q", r)
	}
}

func TestCountWordsEmpty(t *testing.T) {
	if countWords("") != 0 {
		t.Error("should be 0")
	}
}

func TestCountWordsWhitespace(t *testing.T) {
	if countWords("   ") != 0 {
		t.Error("should be 0")
	}
}

func TestFormatTimeZero(t *testing.T) {
	if formatTime(0) != "00:00.0" {
		t.Error("wrong")
	}
}

func TestFormatClockZero(t *testing.T) {
	if formatClock(0) != "00:00" {
		t.Error("wrong")
	}
}

func TestFormatSRTTimeZero(t *testing.T) {
	if formatSRTTime(0) != "00:00:00,000" {
		t.Error("wrong")
	}
}

func TestRepeatStrZero(t *testing.T) {
	if repeatStr("x", 0) != "" {
		t.Error("should be empty")
	}
}

func TestSortAdsEmpty(t *testing.T) { sortAds(nil) }

func TestSortBoundsEmpty(t *testing.T) { sortBounds(nil) }

func TestMergeBoundsEmpty(t *testing.T) {
	if r := mergeBounds(nil); r != nil {
		t.Error("should be nil")
	}
}

func TestMergeBoundsSingle(t *testing.T) {
	r := mergeBounds([][2]float64{{0, 10}})
	if len(r) != 1 || r[0][0] != 0 || r[0][1] != 10 {
		t.Error("wrong")
	}
}

func TestEqualMergedIntervalsDifferentLength(t *testing.T) {
	if equalMergedIntervals(nil, []MergedCutInterval{{1, 2}}) {
		t.Error("should be false")
	}
}

func TestEqualMergedIntervalsBothNil(t *testing.T) {
	if !equalMergedIntervals(nil, nil) {
		t.Error("should be true")
	}
}

func TestTrimSpaceOnlyWhitespace(t *testing.T) {
	if trimSpace("   ") != "" {
		t.Error("should be empty")
	}
}

func TestMergeSegmentsEmpty(t *testing.T) {
	if r := mergeSegments(nil); r != nil {
		t.Error("should be nil")
	}
}

func TestMergeSegmentsSingle(t *testing.T) {
	r := mergeSegments([]TranscriptionSegment{{0, 5, "h", "", nil}})
	if len(r) != 1 {
		t.Error("should be 1")
	}
}

func TestSortSegmentsEmpty(t *testing.T) { sortSegments(nil) }

func TestPutUint16Zero(t *testing.T) {
	b := make([]byte, 2)
	putUint16(b, 0)
	if b[0] != 0 || b[1] != 0 {
		t.Error("wrong")
	}
}

func TestPutUint32Zero(t *testing.T) {
	b := make([]byte, 4)
	putUint32(b, 0)
	if b[0] != 0 || b[1] != 0 || b[2] != 0 || b[3] != 0 {
		t.Error("wrong")
	}
}

func TestBuildWavHeaderZero(t *testing.T) {
	h := buildWavHeader(0)
	if len(h) != 44 {
		t.Error("wrong length")
	}
}

func TestExtractHostNoProtocol(t *testing.T) {
	if r := extractHost("localhost:8080"); r != "localhost" {
		t.Errorf("got %q", r)
	}
}

func TestExtractPortNoPort(t *testing.T) {
	if r := extractPort("http://localhost"); r != "80" {
		t.Errorf("got %q", r)
	}
}

func TestExtractPortHTTPS(t *testing.T) {
	if r := extractPort("https://example.com"); r != "443" {
		t.Errorf("got %q", r)
	}
}

func TestSplitLinesEmpty(t *testing.T) {
	if r := splitLines(""); r != nil {
		t.Error("should be nil")
	}
}

func TestSplitTabEmpty(t *testing.T) {
	r := splitTab("")
	if len(r) != 1 || r[0] != "" {
		t.Error("wrong")
	}
}

func TestToLowerAlreadyLower(t *testing.T) {
	if toLower("hello") != "hello" {
		t.Error("wrong")
	}
}

func TestToLowerEmpty(t *testing.T) {
	if toLower("") != "" {
		t.Error("should be empty")
	}
}

func TestMatchFailedDecodeEmpty(t *testing.T) {
	if matchFailedDecode("") {
		t.Error("should be false")
	}
}

func TestRoundFloatZero(t *testing.T) {
	if roundFloat(0, 2) != 0 {
		t.Error("should be 0")
	}
}

func TestParseFloatEmpty(t *testing.T) {
	if parseFloat("") != 0 {
		t.Error("should be 0")
	}
}

func TestReplaceIPNoMatch(t *testing.T) {
	r := replaceIP("http://10.0.0.1:8080/t", "192.168.1.1")
	if r != "http://10.0.0.1:8080/t" {
		t.Errorf("got %q", r)
	}
}

func TestReplaceIPEmpty(t *testing.T) {
	if replaceIP("", "1.2.3.4") != "" {
		t.Error("should be empty")
	}
}

func TestMatchProgressMS(t *testing.T) {
	m, s, ok := matchProgressMS("processing audio (01:02.3 -> 04:05.6)")
	if !ok || m != 1 || s != 2 {
		t.Error("matchProgressMS failed")
	}
}
