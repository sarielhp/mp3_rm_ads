package audio

import (
	"testing"
)

func TestBuildCutFilterComplex(t *testing.T) {
	keep := [][2]float64{
		{0.0, 10.5},
		{20.0, 35.2},
	}
	res := BuildCutFilterComplex(keep)
	expected := "[0:a]atrim=start=0.000:end=10.500,asetpts=PTS-STARTPTS[a0];[0:a]atrim=start=20.000:end=35.200,asetpts=PTS-STARTPTS[a1];[a0][a1]concat=n=2:v=0:a=1[aout]"
	if res != expected {
		t.Errorf("BuildCutFilterComplex() = %q; want %q", res, expected)
	}
}

func TestFormatCUETime(t *testing.T) {
	if got := FormatCUETime(0); got != "00:00:00" {
		t.Errorf("FormatCUETime(0) = %q; want 00:00:00", got)
	}
	if got := FormatCUETime(65.5); got != "01:05:37" {
		t.Errorf("FormatCUETime(65.5) = %q; want 01:05:37", got)
	}
}

func TestComputeSplitPoints(t *testing.T) {
	keep := [][2]float64{
		{10.0, 20.0},
		{30.0, 40.0},
	}
	pts := ComputeSplitPoints(keep, 50.0)
	if len(pts) != 5 {
		t.Fatalf("expected 5 split points, got %d (%v)", len(pts), pts)
	}
}
