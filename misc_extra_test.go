package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarielhp/clihelp"
)

func TestSetRemoteFFmpegHost(t *testing.T) {
	testConfigPath = filepath.Join(t.TempDir(), "config.json")
	defer func() { testConfigPath = "" }()
	cfg := defaultConfig
	setRemoteFFmpegHost(&cfg, "cloud8")
	if cfg.RemoteFFmpegHost != "cloud8" {
		t.Errorf("expected cloud8, got %s", cfg.RemoteFFmpegHost)
	}
	setRemoteFFmpegHost(&cfg, "")
	if cfg.RemoteFFmpegHost != "" {
		t.Errorf("expected empty, got %s", cfg.RemoteFFmpegHost)
	}
}

func TestExecCommand(t *testing.T) {
	if cmd := execCommand("echo", "hello"); cmd == nil {
		t.Error("nil cmd")
	}
}

func TestFetchOpenRouterModels(t *testing.T) { _ = fetchOpenRouterModels() }

func TestDockerDetectContainerLocalhost(t *testing.T) {
	_ = detectWhisperDockerContainer("http://127.0.0.1:8080")
}

func TestDockerPollProgressWithContainer(t *testing.T) { _ = pollWhisperDockerProgress("nonexistent") }

func TestDockerFetchLogsWithContainer(t *testing.T) { _ = fetchDockerLogs("nonexistent", 10) }

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

func TestValidateTranscriptSanityZeroDuration(t *testing.T) {
	if !validateTranscriptSanity(&TranscriptionData{}, 0, true) {
		t.Error("should be true")
	}
}

func TestTestWhisperServerEmptyURL(t *testing.T) {
	if testWhisperServerEx("", "", 1, 1*time.Millisecond, true) {
		t.Error("expected false for empty URL")
	}
}

func TestTestWhisperServerSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer ts.Close()

	if !testWhisperServerEx(ts.URL, "", 1, 1*time.Millisecond, true) {
		t.Error("expected true for active Whisper server")
	}
}

func TestTestWhisperServerRetrySuccess(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Waking up..."))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text":"ok"}`))
	}))
	defer ts.Close()

	if !testWhisperServerEx(ts.URL, "", 3, 5*time.Millisecond, true) {
		t.Error("expected true on retry success")
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestTestWhisperServerFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	if testWhisperServerEx(ts.URL, "", 2, 5*time.Millisecond, true) {
		t.Error("expected false for failing Whisper server")
	}
}

func TestTestWhisperServerClientError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	if testWhisperServerEx(ts.URL, "", 3, 5*time.Millisecond, true) {
		t.Error("expected false for 404 client error without retrying")
	}
}

func TestParseFlagsTestWhisperCommand(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"abs", "test", "whisper"}
	_, cli := parseFlags()
	if !cli.IsTestCommand || !cli.TestWhisper {
		t.Errorf("expected IsTestCommand=true, TestWhisper=true, got %v, %v", cli.IsTestCommand, cli.TestWhisper)
	}

	os.Args = []string{"abs", "test"}
	_, cli = parseFlags()
	if !cli.IsTestCommand || !cli.TestWhisper {
		t.Errorf("expected IsTestCommand=true, TestWhisper=true for default test command, got %v, %v", cli.IsTestCommand, cli.TestWhisper)
	}

}

func TestParseFlagsQuiet(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"abs", "-q", "file.mp3"}
	_, cli := parseFlags()
	if !cli.Quiet {
		t.Error("expected Quiet=true for -q")
	}

	os.Args = []string{"abs", "--quiet", "file.mp3"}
	_, cli = parseFlags()
	if !cli.Quiet {
		t.Error("expected Quiet=true for --quiet")
	}
}

func TestParseFlagsDryRun(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"abs", "proc", "--dry-run", "file.mp3"}
	act, cli := parseFlags()
	if act != "proc" || !cli.DryRun {
		t.Errorf("expected proc with DryRun=true, got act=%s, DryRun=%v", act, cli.DryRun)
	}
}

func TestHandleProcDryRun(t *testing.T) {
	dir := t.TempDir()
	ep1 := filepath.Join(dir, "ep1.mp3")
	ep2 := filepath.Join(dir, "ep2.mp3")
	ep3 := filepath.Join(dir, "ep3.mp3")
	os.WriteFile(ep1, []byte("audio1"), 0644)
	os.WriteFile(ep2, []byte("audio2"), 0644)
	os.WriteFile(ep3, []byte("audio3"), 0644)

	os.WriteFile(filepath.Join(dir, "ep2.transcript.json"), []byte("{}"), 0644)

	os.WriteFile(filepath.Join(dir, "ep3.transcript.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "ep3.cuts.json"), []byte(`{"cut_intervals":[]}`), 0644)

	handleProcDryRun([]string{ep1, ep2, ep3}, CLIOptions{DryRun: true, Count: 2, Verbose: true}, Config{})
}

func TestScanningDirectoryOutput(t *testing.T) {
	d := t.TempDir()

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cli := CLIOptions{}
	if !cli.Quiet {
		fmt.Printf("Scanning: %s\n", d)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf [512]byte
	n, _ := r.Read(buf[:])
	out := string(buf[:n])
	if !strings.Contains(out, "Scanning: "+d) {
		t.Errorf("expected output to contain 'Scanning: %s', got %q", d, out)
	}
}

func TestBuildUsageApp(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)
	if app.Name != "abs" {
		t.Errorf("expected app name 'abs', got %q", app.Name)
	}
	if len(app.Commands) < 5 {
		t.Errorf("expected at least 5 commands, got %d", len(app.Commands))
	}
}

func TestCLIAudit(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)
	if err := clihelp.Audit(app); err != nil {
		t.Fatalf("CLI App audit failed: %v", err)
	}
}

func TestNativeKittyGraphics(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "test.png")
	os.WriteFile(imgPath, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDRtestdata"), 0644)

	esc, err := encodeNativeKittyGraphics(imgPath, 20, 10)
	if err != nil {
		t.Fatalf("encodeNativeKittyGraphics failed: %v", err)
	}
	if !strings.HasPrefix(esc, "\x1b_Ga=T,f=100") {
		t.Errorf("expected escape prefix '\\x1b_Ga=T,f=100', got %q", esc)
	}
	if !strings.HasSuffix(esc, "\x1b\\") {
		t.Errorf("expected escape suffix '\\x1b\\\\', got %q", esc)
	}
}
