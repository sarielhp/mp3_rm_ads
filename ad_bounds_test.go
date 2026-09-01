package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAdSegmentsRejectsImplausibleBounds(t *testing.T) {
	const dur = 3600.0
	cases := []struct {
		name string
		in   AdSegment
		want bool
	}{
		{"past end is clamped", AdSegment{Start: 1, End: 99999}, true},
		{"negative start is clamped", AdSegment{Start: -500, End: 60}, true},
		{"entirely negative is dropped", AdSegment{Start: -500, End: -1}, false},
		{"inverted is dropped", AdSegment{Start: 100, End: 50}, false},
		{"empty is dropped", AdSegment{Start: 100, End: 100}, false},
		{"NaN is dropped", AdSegment{Start: math.NaN(), End: 60}, false},
		{"Inf is dropped", AdSegment{Start: 0, End: math.Inf(1)}, false},
		{"ordinary is kept", AdSegment{Start: 60, End: 120}, true},
	}
	for _, tc := range cases {
		got := sanitizeAdSegments([]AdSegment{tc.in}, dur)
		if (len(got) == 1) != tc.want {
			t.Errorf("%s: got %v, want kept=%v", tc.name, got, tc.want)
			continue
		}
		for _, s := range got {
			if s.Start < 0 || s.End > dur || s.End <= s.Start {
				t.Errorf("%s: escaped sanitizer: %+v", tc.name, s)
			}
		}
	}
}

func TestSanitizeAdSegmentsCapsSegmentCount(t *testing.T) {
	var many []AdSegment
	for i := 0; i < 10000; i++ {
		many = append(many, AdSegment{Start: float64(i) * 0.1, End: float64(i)*0.1 + 0.05})
	}
	if got := len(sanitizeAdSegments(many, 3600)); got > maxAdSegments {
		t.Errorf("expected at most %d segments, got %d", maxAdSegments, got)
	}
}

func TestCutIsRefusedWhenAlmostNothingWouldBeKept(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "ep.mp3")
	out := filepath.Join(dir, ".work", "ep.mp3.tmp.mp3")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	writeRealMP3(t, src, 10)

	// What calculateKeepSegments produces for an LLM claiming the whole episode
	// after the first second is a sponsor read.
	keep := calculateKeepSegments(10, []AdSegment{{Start: 1, End: 99999, Reason: "sponsor"}})

	if cutAudioFFmpegWithHost(src, keep, out, "") {
		t.Errorf("cut was accepted; it would have replaced a 10s episode with ~1s of audio")
	}
	if fileExists(out) {
		t.Errorf("a refused cut still wrote an output file")
	}
}

func TestOrdinaryCutIsStillAccepted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "ep.mp3")
	out := filepath.Join(dir, ".work", "ep.mp3.tmp.mp3")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	writeRealMP3(t, src, 10)

	keep := calculateKeepSegments(10, []AdSegment{{Start: 2, End: 4, Reason: "ad"}})
	if !cutAudioFFmpegWithHost(src, keep, out, "") {
		t.Errorf("a normal 20%% cut was refused; the guard is too aggressive")
	}
}

func TestCalculateKeepSegmentsIgnoresInvertedSegments(t *testing.T) {
	// An inverted segment used to produce overlapping, out-of-order keep ranges,
	// which ffmpeg then concatenated into garbage.
	keep := calculateKeepSegments(3600, []AdSegment{{Start: 100, End: 50}})
	if len(keep) != 1 || keep[0][0] != 0 || keep[0][1] != 3600 {
		t.Errorf("expected the whole episode kept, got %v", keep)
	}
	for i, seg := range keep {
		if seg[1] <= seg[0] {
			t.Errorf("keep[%d] is empty or inverted: %v", i, seg)
		}
		if i > 0 && seg[0] < keep[i-1][1] {
			t.Errorf("keep[%d] overlaps keep[%d]: %v vs %v", i, i-1, seg, keep[i-1])
		}
	}
}
