package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHTML_Entities(t *testing.T) {
	input := "Tom &amp; Jerry &lt;3 cheese &gt; crackers &quot;quoted&quot; &apos;single&apos; space&nbsp;bar"
	expected := "Tom & Jerry <3 cheese > crackers \"quoted\" 'single' space bar"
	got := renderHTML(input)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestRenderHTML_AmpersandPerformance(t *testing.T) {
	largeAmpersands := strings.Repeat("&", 10000)
	start := time.Now()
	got := renderHTML(largeAmpersands)
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("renderHTML with 10000 ampersands took too long: %v", elapsed)
	}
	if got != largeAmpersands {
		t.Errorf("expected %d ampersands, got len %d", len(largeAmpersands), len(got))
	}

	mixed := "A & B &amp; C &&&& D &notanentity; E"
	expected := "A & B & C &&&& D &notanentity; E"
	if res := renderHTML(mixed); res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}
}
