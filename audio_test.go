package main

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
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

func TestFormatCUETime(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{-1.0, "00:00:00"},
		{0.0, "00:00:00"},
		{59.0, "00:59:00"},
		{60.0, "01:00:00"},
		{61.5, "01:01:37"},
		{3661.0, "61:01:00"},
	}
	for _, tc := range cases {
		got := formatCUETime(tc.in)
		if got != tc.want {
			t.Errorf("formatCUETime(%f) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestComputeSplitPoints(t *testing.T) {
	keep := [][2]float64{{0.0, 2.0}, {4.0, 7.0}, {9.0, 10.0}}
	pts := computeSplitPoints(keep, 10.0)
	expected := []float64{0.0, 2.0, 4.0, 7.0, 9.0}
	if len(pts) != len(expected) {
		t.Fatalf("expected %d points, got %d: %v", len(expected), len(pts), pts)
	}
	for i, exp := range expected {
		if math.Abs(pts[i]-exp) > 0.01 {
			t.Errorf("pts[%d] = %f, want %f", i, pts[i], exp)
		}
	}

	keepPreroll := [][2]float64{{3.0, 10.0}}
	ptsPreroll := computeSplitPoints(keepPreroll, 10.0)
	expectedPreroll := []float64{0.0, 3.0}
	if len(ptsPreroll) != len(expectedPreroll) {
		t.Fatalf("expected %d points, got %d: %v", len(expectedPreroll), len(ptsPreroll), ptsPreroll)
	}
}

func TestDetermineKeepChunks(t *testing.T) {
	keep := [][2]float64{{0.0, 2.0}, {4.0, 7.0}, {9.0, 10.0}}
	pts := computeSplitPoints(keep, 10.0)
	chunks := determineKeepChunks(pts, keep, "/work", 10.0, "pfx")
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	expectedSuffixes := []string{"pfx_chunk_01.mp3", "pfx_chunk_03.mp3", "pfx_chunk_05.mp3"}
	for i, suffix := range expectedSuffixes {
		if !strings.HasSuffix(chunks[i], suffix) {
			t.Errorf("chunks[%d] = %s, want suffix %s", i, chunks[i], suffix)
		}
	}
}

func TestWriteConcatManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "concat.txt")
	paths := []string{"/path/to/part 1.mp3", "/path/to/part's 2.mp3"}
	if err := writeConcatManifest(manifest, paths); err != nil {
		t.Fatalf("writeConcatManifest error: %v", err)
	}
	content, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest error: %v", err)
	}
	exp := "file '/path/to/part 1.mp3'\nfile '/path/to/part'\\''s 2.mp3'\n"
	if string(content) != exp {
		t.Errorf("got %q, want %q", string(content), exp)
	}
}

func TestFastCutMultipleAdSegments(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp3")
	workDir := filepath.Join(dir, ".work")
	_ = os.MkdirAll(workDir, 0755)
	out := filepath.Join(workDir, "out.mp3")

	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=10",
		"-metadata", "title=SpeedTestEpisode",
		"-metadata", "artist=SpeedArtist",
		"-c:a", "libmp3lame", src)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg create failed: %v, out: %s", err, string(outBytes))
	}

	keep := [][2]float64{{0.0, 3.0}, {6.0, 10.0}}
	if !cutAudioFFmpeg(src, keep, out) {
		t.Fatalf("cutAudioFFmpeg failed")
	}
	if !fileExists(out) {
		t.Fatalf("output file does not exist")
	}

	dur := getAudioDuration(out)
	if math.Abs(dur-7.0) > 0.5 {
		t.Errorf("expected duration ~7.0s, got %f", dur)
	}

	tags := extractID3Tags(out)
	if tags["title"] != "SpeedTestEpisode" {
		t.Errorf("expected title 'SpeedTestEpisode', got %q", tags["title"])
	}
	if tags["artist"] != "SpeedArtist" {
		t.Errorf("expected artist 'SpeedArtist', got %q", tags["artist"])
	}
}

func TestCutAudioFFmpegStreamCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "stream_src.mp3")
	workDir := filepath.Join(dir, ".work")
	_ = os.MkdirAll(workDir, 0755)
	out := filepath.Join(workDir, "stream_out.mp3")

	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=800:duration=8",
		"-metadata", "title=StreamTest",
		"-c:a", "libmp3lame", src)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg create failed: %v, out: %s", err, string(outBytes))
	}

	keep := [][2]float64{{0.0, 2.0}, {4.0, 8.0}}
	if !cutAudioFFmpegStreamCopy(src, keep, out, workDir) {
		t.Fatalf("cutAudioFFmpegStreamCopy failed")
	}
	if !fileExists(out) {
		t.Fatalf("output file does not exist")
	}

	dur := getAudioDuration(out)
	if math.Abs(dur-6.0) > 0.5 {
		t.Errorf("expected duration ~6.0s, got %f", dur)
	}

	tags := extractID3Tags(out)
	if tags["title"] != "StreamTest" {
		t.Errorf("expected title 'StreamTest', got %q", tags["title"])
	}
}
