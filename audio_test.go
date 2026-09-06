package main

import (
	"strings"
	"testing"
)

func TestBuildCutFilterComplex(t *testing.T) {
	emptyFilter := buildCutFilterComplex(nil)
	if emptyFilter != "concat=n=0:v=0:a=1[aout]" {
		t.Errorf("unexpected empty filter: %s", emptyFilter)
	}

	singleSeg := [][2]float64{{10.5, 20.25}}
	singleFilter := buildCutFilterComplex(singleSeg)
	expectedSingle := "[0:a]atrim=start=10.500:end=20.250,asetpts=PTS-STARTPTS[a0];[a0]concat=n=1:v=0:a=1[aout]"
	if singleFilter != expectedSingle {
		t.Errorf("expected %q, got %q", expectedSingle, singleFilter)
	}

	multiSegs := [][2]float64{
		{0.0, 5.0},
		{10.0, 15.0},
		{20.0, 25.0},
	}
	multiFilter := buildCutFilterComplex(multiSegs)
	if !strings.Contains(multiFilter, "[0:a]atrim=start=0.000:end=5.000,asetpts=PTS-STARTPTS[a0];") ||
		!strings.Contains(multiFilter, "[0:a]atrim=start=10.000:end=15.000,asetpts=PTS-STARTPTS[a1];") ||
		!strings.Contains(multiFilter, "[0:a]atrim=start=20.000:end=25.000,asetpts=PTS-STARTPTS[a2];") ||
		!strings.Contains(multiFilter, "[a0][a1][a2]") ||
		!strings.HasSuffix(multiFilter, "concat=n=3:v=0:a=1[aout]") {
		t.Errorf("multiFilter unexpected structure: %s", multiFilter)
	}

	largeSegs := make([][2]float64, 500)
	for i := range largeSegs {
		largeSegs[i] = [2]float64{float64(i * 10), float64(i*10 + 5)}
	}
	largeFilter := buildCutFilterComplex(largeSegs)
	expectedSuffix := "concat=n=500:v=0:a=1[aout]"
	if !strings.HasSuffix(largeFilter, expectedSuffix) {
		t.Errorf("largeFilter missing expected suffix %q", expectedSuffix)
	}
	if !strings.Contains(largeFilter, "[a499]") {
		t.Errorf("largeFilter missing last stream tag [a499]")
	}
}
