package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writeRealMP3(t *testing.T, path string, seconds int) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-t", itoa(seconds), "-c:a", "libmp3lame", "-b:a", "64k", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not synthesize test mp3: %v: %s", err, out)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func writeSaneTranscript(t *testing.T, path string, dur float64) {
	t.Helper()
	body := `{"text":"one two three four five six seven eight nine ten eleven twelve ` +
		`thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty twenty-one",` +
		`"segments":[{"start":0.0,"end":` + "10.0" + `,"text":"one two three four five six seven eight nine ten ` +
		`eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty twenty-one"}]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// offlineProfile has no URL, so detectAdsLLM returns nil without any network call.
func offlineProfile() LLMProfile {
	return LLMProfile{ID: 1, Name: "offline", Type: "ollama", Model: "none"}
}

func TestTranscribeFailureReturnsInsteadOfPanicking(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "ep.mp3")
	if err := os.WriteFile(mp3, []byte("not an mp3"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ep.transcript.json"), []byte("{{{ bad"), 0644); err != nil {
		t.Fatal(err)
	}

	hasError, _, _ := processSingleAudioFile(0, 1, 0, mp3,
		CLIOptions{Quiet: true}, Config{}, "proc", time.Now(), offlineProfile())

	if !hasError {
		t.Errorf("expected hasError=true when the transcript cannot be parsed")
	}
}

func TestRecutDoesNotFallThroughIntoTheFullPipeline(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "ep.mp3")
	writeRealMP3(t, mp3, 10)
	cuts := `{"version":1,"generator":"test","target_file":"ep.mp3","original_duration_sec":10,
	  "cut_intervals":[{"start_sec":2,"end_sec":3,"reason":"ad"}]}`
	if err := os.WriteFile(filepath.Join(dir, "ep.cuts.json"), []byte(cuts), 0644); err != nil {
		t.Fatal(err)
	}
	// A transcript that cannot be parsed. recut must never look at it; if control
	// falls through into the transcribe pipeline this panics at the nil deref.
	if err := os.WriteFile(filepath.Join(dir, "ep.transcript.json"), []byte("{{{ bad"), 0644); err != nil {
		t.Fatal(err)
	}

	processSingleAudioFile(0, 1, 0, mp3,
		CLIOptions{Quiet: true, Recut: true}, Config{}, "recut", time.Now(), offlineProfile())
}

func TestTranscribeMinDoesNotWriteCutMetadata(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "ep.mp3")
	writeRealMP3(t, mp3, 10)
	writeSaneTranscript(t, filepath.Join(dir, "ep.transcript.json"), 10)

	before, err := os.ReadFile(mp3)
	if err != nil {
		t.Fatal(err)
	}

	processSingleAudioFile(0, 1, 0, mp3,
		CLIOptions{Quiet: true, TranscribeMin: "1"}, Config{}, "proc", time.Now(), offlineProfile())

	if fileExists(filepath.Join(dir, "ep.cuts.json")) {
		t.Errorf("--tminutes wrote ep.cuts.json; a preview run must not touch cut metadata")
	}
	if fileExists(mp3 + ".precut") {
		t.Errorf("--tminutes created a .precut; it claims the original is not modified")
	}
	after, err := os.ReadFile(mp3)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("--tminutes modified the original audio after printing that it did not")
	}
}

func TestNoAdsDetectedDoesNotReEncodeOrCreatePrecut(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "ep.mp3")
	writeRealMP3(t, mp3, 10)
	writeSaneTranscript(t, filepath.Join(dir, "ep.transcript.json"), 10)

	before, err := os.ReadFile(mp3)
	if err != nil {
		t.Fatal(err)
	}

	processSingleAudioFile(0, 1, 0, mp3,
		CLIOptions{Quiet: true}, Config{}, "proc", time.Now(), offlineProfile())

	if fileExists(mp3 + ".precut") {
		t.Errorf("no ads were detected, but the original was moved to .precut")
	}
	after, err := os.ReadFile(mp3)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("no ads were detected, but the audio was re-encoded (%d -> %d bytes)",
			len(before), len(after))
	}
}
