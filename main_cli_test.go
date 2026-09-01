package main

import (
	"os"
	"testing"
)

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
