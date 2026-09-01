package main

import (
	"testing"
)

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
