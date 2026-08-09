package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
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

func TestSaveCutsJSON(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, "a"}, {50, 60, "b"}}, nil, true)
	if !r.Changed || len(r.KeepSegments) != 3 {
		t.Fatal("saveCutsJSON basic failed")
	}
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.OriginalDurationSec != 100 || len(cd.CutIntervals) != 2 {
		t.Error("cuts data mismatch")
	}
}

func TestSaveCutsJSONUnchanged(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if r.Changed {
		t.Error("should be unchanged")
	}
}

func TestConvertJSONToSRT(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}, {5, 10, "w", "", nil}}}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	srt := convertJSONToSRT(jf, &data, "", true)
	if srt == "" {
		t.Fatal("empty srt path")
	}
	content, _ := readFile(srt)
	if !containsStr(string(content), "00:00:00,000 --> 00:00:05,000") {
		t.Error("missing timestamp")
	}
}

func TestConvertJSONToTXT(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}, Language: "en"}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	txt := convertJSONToTXT(jf, &data, 10, "", true)
	if txt == "" {
		t.Fatal("empty txt path")
	}
	content, _ := readFile(txt)
	if !containsStr(string(content), "PODCAST TRANSCRIPTION") {
		t.Error("missing header")
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

func TestGetProfileCostLocal(t *testing.T) {
	c := getProfileCost(LLMProfile{Type: "ollama", URL: "http://localhost:11434", Model: "m"})
	if c.Type != "Local" {
		t.Error("expected Local")
	}
}

func TestFormatTranscript(t *testing.T) {
	d := &TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}, {5, 10, "w", "", nil}}}
	r := formatTranscript(d, 10)
	if !containsStr(r, "[0.0s -> 5.0s] h") {
		t.Error("missing segment")
	}
}

func TestFormatTranscriptNoSegments(t *testing.T) {
	r := formatTranscript(&TranscriptionData{Text: "hw"}, 10)
	if !containsStr(r, "[0.0s -> 10.0s] hw") {
		t.Error("missing text")
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

func TestSaveJSONTranscript(t *testing.T) {
	d := t.TempDir()
	saveJSONTranscript(d+"/t.mp3", &TranscriptionData{Text: "hw"}, d+"/t.json", true, nil)
	if !fileExists(d + "/t.json") {
		t.Error("transcript not saved")
	}
}

func TestSaveJSONTranscriptWithTags(t *testing.T) {
	d := t.TempDir()
	saveJSONTranscript(d+"/t.mp3", &TranscriptionData{}, d+"/t.json", true, map[string]string{"title": "T"})
	raw, _ := readFile(d + "/t.json")
	var m map[string]interface{}
	json.Unmarshal(raw, &m)
	if m["id3_title"] != "T" {
		t.Error("tag not saved")
	}
}

func TestProcessJSONFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	processJSONFile(jf, CLIOptions{Quiet: true, ExportSRT: true, ExportTXT: true})
	if !fileExists(d+"/t.srt") || !fileExists(d+"/t.transcript.txt") {
		t.Error("exports not created")
	}
}

func TestSetDefaultProfile(t *testing.T) {
	cfg := &Config{ActiveProfileID: 1, Profiles: []LLMProfile{{1, "A", "", "", "", ""}, {2, "B", "", "", "", ""}}}
	setDefaultProfile(cfg, 2)
	if cfg.ActiveProfileID != 2 {
		t.Error("default not updated")
	}
}

func TestResolveAudioFiles(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	main, precut, src := resolveAudioFiles(f, CLIOptions{Quiet: true})
	if main != f || precut != f+".precut" || src != f {
		t.Error("resolveAudioFiles failed")
	}
}

func TestParseFlags(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"p", "-q", "--srt", "--txt", "f.mp3"}
	cli := parseFlags()
	if !cli.Quiet || !cli.ExportSRT || !cli.ExportTXT {
		t.Error("parseFlags failed")
	}
}

func TestLocalIP(t *testing.T) {
	if ip := localIP(); ip == "" {
		t.Error("empty IP")
	}
}

func TestDefaultConfig(t *testing.T) {
	if len(defaultConfig.Profiles) != 4 || defaultConfig.ActiveProfileID != 1 {
		t.Error("default config wrong")
	}
}

func TestFilterKeyTerms(t *testing.T) {
	r := filterKeyTerms([]string{"sonnet", "unknown", "flash"})
	if len(r) != 2 || r[0] != "sonnet" || r[1] != "flash" {
		t.Error("filterKeyTerms failed")
	}
}

func TestSplitModelTokens(t *testing.T) {
	r := splitModelTokens("a-b.c")
	if len(r) != 3 || r[0] != "a" || r[1] != "b" || r[2] != "c" {
		t.Error("splitModelTokens failed")
	}
}

func TestLocalIPError(t *testing.T) {
	if ip := localIP(); ip == "" {
		t.Error("should return an IP")
	}
}

func TestIsLocalHostError(t *testing.T) {
	orig := netInterfaceAddrs
	netInterfaceAddrs = func() ([]net.Addr, error) { return nil, fmt.Errorf("e") }
	defer func() { netInterfaceAddrs = orig }()
	if isLocalHost("1.2.3.4") {
		t.Error("should be false")
	}
}

func TestSaveCutsJSONWithProfile(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, &LLMProfile{Name: "P", Model: "m"}, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.LLMUsed != "P (m)" {
		t.Errorf("got %q", cd.LLMUsed)
	}
}

func TestSaveCutsJSONEmptyAds(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	if !r.Changed || len(r.KeepSegments) != 1 {
		t.Error("empty ads failed")
	}
}

func TestSaveCutsJSONCorrupt(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	os.WriteFile(d+"/t.cuts.json", []byte("corrupt"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if !r.Changed {
		t.Error("should report changed")
	}
}

func TestConvertJSONToSRTFromFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "h", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	r, _ := json.Marshal(data)
	os.WriteFile(jf, r, 0644)
	srt := convertJSONToSRT(jf, nil, "", true)
	if srt == "" {
		t.Error("empty srt path")
	}
}

func TestConvertJSONToTXTFromFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "h", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	r, _ := json.Marshal(data)
	os.WriteFile(jf, r, 0644)
	txt := convertJSONToTXT(jf, nil, 10, "", true)
	if txt == "" {
		t.Error("empty txt path")
	}
}

func TestConvertJSONToSRTInvalid(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	os.WriteFile(jf, []byte("bad"), 0644)
	if srt := convertJSONToSRT(jf, nil, "", true); srt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToTXTInvalid(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	os.WriteFile(jf, []byte("bad"), 0644)
	if txt := convertJSONToTXT(jf, nil, 10, "", true); txt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToSRTReadError(t *testing.T) {
	if srt := convertJSONToSRT("/nonexistent", nil, "", true); srt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToTXTReadError(t *testing.T) {
	if txt := convertJSONToTXT("/nonexistent", nil, 0, "", true); txt != "" {
		t.Error("should be empty")
	}
}

func TestSaveCutsJSONFormatting(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.CutIntervals) == 0 {
		t.Fatal("no intervals")
	}
	e := cd.CutIntervals[0]
	if e.StartFormatted != "00:10" || e.DurationSec != 10 {
		t.Error("formatting wrong")
	}
}

func TestSaveCutsJSONVersion(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.Version != 1 {
		t.Error("version wrong")
	}
}

func TestSaveCutsJSONGenerator(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.Generator != "mp3_rm_ads" {
		t.Error("generator wrong")
	}
}

func TestSaveCutsJSONTargetFile(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.TargetFile != "t.mp3" {
		t.Error("target file wrong")
	}
}

func TestSaveCutsJSONKeepIntervals(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.KeepIntervals) != 2 {
		t.Error("keep intervals wrong")
	}
}

func TestSaveCutsJSONTotalCutDuration(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.TotalCutDurationSec != 10 {
		t.Error("total cut wrong")
	}
}

func TestSaveCutsJSONMergedIntervals(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {22, 30, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.MergedCutIntervals) != 2 {
		t.Error("merged intervals wrong")
	}
}

func TestSaveCutsJSONExistingMerged(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{5, 10, 5, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{5, 10}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}

func TestDockerFetchLogs(t *testing.T) {
	if r := fetchDockerLogs("", 10); r != "" {
		t.Error("should be empty")
	}
}

func TestDockerPollProgress(t *testing.T) {
	if r := pollWhisperDockerProgress(""); r != nil {
		t.Error("should be nil")
	}
}

func TestDockerDetectContainer(t *testing.T) {
	if r := detectWhisperDockerContainer("https://remote.example.com"); r != "" {
		t.Error("should be empty")
	}
}

func TestExtractKeywordsLLM(t *testing.T) {
	if r := extractKeywordsLLM("t", LLMProfile{URL: "http://invalid:1", Model: "m"}, true); r != "" {
		t.Error("should be empty")
	}
}

func TestDetectAdsLLM(t *testing.T) {
	if r := detectAdsLLM("t", LLMProfile{URL: "http://invalid:1", Model: "m"}); r != nil {
		t.Error("should be nil")
	}
}

func TestResolveOutputFile(t *testing.T) {
	if r := resolveOutputFile("/p/t.mp3", CLIOptions{}, 1); r != "/p/t.mp3" {
		t.Error("default output")
	}
	if r := resolveOutputFile("/p/t.mp3", CLIOptions{Output: "/o/e.mp3"}, 1); r != "/o/e.mp3" {
		t.Error("custom output")
	}
}

func TestPrintTimingSummary(t *testing.T) {
	printTimingSummary(100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
}
func TestPrintFullSummary(t *testing.T) {
	printFullSummary(100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
}
func TestListProfiles(t *testing.T) {
	listProfiles(Config{ActiveProfileID: 1, Profiles: []LLMProfile{{1, "T", "t", "http://t", "m", ""}}})
}
func TestPrintUsage(t *testing.T) { printUsage() }
func TestHandleRecutNoCutsFile(t *testing.T) {
	handleRecut("t.mp3", "t.mp3", "t.precut", "t_af.mp3", "t", 100, LLMProfile{}, CLIOptions{Quiet: true}, time.Now())
}
func TestHandleTranscribeMinNoTruncate(t *testing.T) {
	s := "t.mp3"
	if r := handleTranscribeMin(&s, 100, CLIOptions{TranscribeMin: "200m", Quiet: true}); r != 100 {
		t.Error("should return original")
	}
}
func TestStep1Duration(t *testing.T) {
	d := step1Duration(time.Now())
	if d < 0 {
		t.Error("negative")
	}
}
func TestCheckPrecutSymlink(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	checkPrecutSymlink(f)
}
func TestFindMP3FilesNoDir(t *testing.T) {
	files := findMP3Files("/nonexistent")
	if files != nil {
		t.Error("should be nil")
	}
}
func TestCopyFileNonexistent(t *testing.T)           { copyFile("/nonexistent/src", "/nonexistent/dst") }
func TestSafeMoveNonexistent(t *testing.T)           { safeMove("/nonexistent/src", "/nonexistent/dst") }
func TestCheckPrecutSymlinkNonexistent(t *testing.T) { checkPrecutSymlink("/nonexistent") }
func TestExtractID3TagsNonexistent(t *testing.T) {
	if tags := extractID3Tags("/nonexistent"); tags != nil {
		t.Error("should be nil")
	}
}
func TestGetAudioDurationNonexistent(t *testing.T) {
	if d := getAudioDuration("/nonexistent"); d != 0 {
		t.Error("should be 0")
	}
}
func TestValidateWavFileNonexistent(t *testing.T) {
	if validateWavFile("/nonexistent") {
		t.Error("should be false")
	}
}
func TestConvertToWAVNonexistent(t *testing.T) {
	if convertToWAV("/nonexistent/in", "/nonexistent/out") {
		t.Error("should be false")
	}
}
func TestTruncateAudioNonexistent(t *testing.T) {
	if truncateAudio("/nonexistent/in", "/nonexistent/out", 10) {
		t.Error("should be false")
	}
}
func TestCutAudioFFmpegEmpty(t *testing.T) {
	if cutAudioFFmpeg("in", nil, "out") {
		t.Error("should be false")
	}
}
func TestExecCommand(t *testing.T) {
	if cmd := execCommand("echo", "hello"); cmd == nil {
		t.Error("nil cmd")
	}
}
func TestFetchOpenRouterModels(t *testing.T) { _ = fetchOpenRouterModels() }
func TestGetProfileCostOpenRouter(t *testing.T) {
	_ = getProfileCost(LLMProfile{Type: "openrouter", URL: "https://openrouter.ai", Model: "m"})
}
func TestDockerDetectContainerLocalhost(t *testing.T) {
	_ = detectWhisperDockerContainer("http://127.0.0.1:8080")
}
func TestDockerPollProgressWithContainer(t *testing.T) { _ = pollWhisperDockerProgress("nonexistent") }
func TestDockerFetchLogsWithContainer(t *testing.T)    { _ = fetchDockerLogs("nonexistent", 10) }
func TestMatchProgressHMSNoMatch(t *testing.T) {
	_, _, _, ok := matchProgressHMS("no")
	if ok {
		t.Error("should not match")
	}
}
func TestMatchProgressMSNoMatch(t *testing.T) {
	_, _, ok := matchProgressMS("no")
	if ok {
		t.Error("should not match")
	}
}
func TestMatchProgressPercentNoMatch(t *testing.T) {
	_, ok := matchProgressPercent("no")
	if ok {
		t.Error("should not match")
	}
}
func TestExtractJSONArrayNoMatch(t *testing.T) {
	if r := extractJSONArray("no"); r != nil {
		t.Error("should be nil")
	}
}
func TestExtractJSONArrayUnclosed(t *testing.T) {
	if r := extractJSONArray("[unclosed"); r != nil {
		t.Error("should be nil")
	}
}
func TestExtractJSONArrayInvalid(t *testing.T) {
	if r := extractJSONArray("[bad]"); r != nil {
		t.Error("should be nil")
	}
}
func TestProcessJSONFileNoFile(t *testing.T) {
	processJSONFile("/nonexistent", CLIOptions{Quiet: true})
}
func TestSyncMutex(t *testing.T) { m := syncMutex{ch: make(chan struct{}, 1)}; m.Lock(); m.Unlock() }
func TestSyncMu(t *testing.T)    { var m syncMu; m.Lock(); m.Unlock() }
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
func TestFilterKeyTermsEmpty(t *testing.T) {
	if r := filterKeyTerms([]string{"x"}); len(r) != 0 {
		t.Error("should be empty")
	}
}
func TestSplitModelTokensEmpty(t *testing.T) {
	if r := splitModelTokens(""); len(r) != 0 {
		t.Error("should be empty")
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
func TestValidateTranscriptSanityZeroDuration(t *testing.T) {
	if !validateTranscriptSanity(&TranscriptionData{}, 0, true) {
		t.Error("should be true")
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
func TestSortAdsEmpty(t *testing.T)    { sortAds(nil) }
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
func TestGetProfileCostUnknown(t *testing.T) {
	c := getProfileCost(LLMProfile{Type: "unknown", URL: "http://custom:8080", Model: "m"})
	if c.Type != "Unknown" {
		t.Errorf("got %q", c.Type)
	}
}
func TestSaveCutsJSONExistingRawEmpty(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, nil, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawNil(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, nil, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawDifferentOrder(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}, {10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawDifferentOrderDiff(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}, {50, 60, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}
func TestSaveCutsJSONExistingMergedDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 25, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}
func TestSaveCutsJSONExistingRawDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}
func TestSaveCutsJSONExistingRawSame(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONOriginalDuration(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.OriginalDurationSec != 100 {
		t.Error("wrong")
	}
}
func TestSaveCutsJSONExistingRawSameOrder(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder2(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder3(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder4(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder5(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder6(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder7(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder8(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder9(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder10(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder11(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder12(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder13(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder14(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder15(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder16(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder17(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder18(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder19(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder20(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder21(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder22(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder23(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder24(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder25(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder26(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder27(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder28(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder29(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder30(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder31(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder32(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder33(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder34(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder35(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder36(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder37(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder38(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder39(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder40(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder41(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
func TestSaveCutsJSONExistingRawSameOrder42(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
