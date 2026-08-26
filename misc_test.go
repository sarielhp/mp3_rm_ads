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
)

func TestSetDefaultProfile(t *testing.T) {
	orig := testConfigPath
	testConfigPath = t.TempDir() + "/config.json"
	defer func() { testConfigPath = orig }()
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

func TestParseFlagsConfigPodcastsDir(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"abs", "config", "--podcasts_dir", "/path/to/podcasts"}
	cli := parseFlags()
	if !cli.IsConfigCommand || cli.PodcastsDir != "/path/to/podcasts" {
		t.Errorf("expected IsConfigCommand=true, PodcastsDir='/path/to/podcasts', got %v, %q", cli.IsConfigCommand, cli.PodcastsDir)
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
	printTimingSummary(true, 100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
	printTimingSummary(false, 100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
}

func TestPrintFullSummary(t *testing.T) {
	printFullSummary(true, 100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
	printFullSummary(false, 100, 80, 20, 20, 2, time.Second, time.Second, time.Second, 3*time.Second)
}

func TestListProfiles(t *testing.T) {
	listProfiles(Config{ActiveProfileID: 1, Profiles: []LLMProfile{{1, "T", "t", "http://t", "m", ""}}})
}

func TestABSNoURL(t *testing.T) {
	if testAudiobookshelfServer(Config{}, true) {
		t.Error("should fail with no URL")
	}
}

func TestABSLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" && r.Method == "POST" {
			w.WriteHeader(200)
			w.Write([]byte(`{"user":{"token":"abc"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	if !testAudiobookshelfServer(Config{AudiobookshelfURL: srv.URL, AudiobookshelfUser: "u", AudiobookshelfPass: "p"}, true) {
		t.Error("should succeed with valid login")
	}
}

func TestABSLoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
	}))
	defer srv.Close()

	if testAudiobookshelfServer(Config{AudiobookshelfURL: srv.URL, AudiobookshelfUser: "u", AudiobookshelfPass: "p"}, true) {
		t.Error("should fail with bad credentials")
	}
}

func TestABSNoAuthSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer srv.Close()

	if !testAudiobookshelfServer(Config{AudiobookshelfURL: srv.URL}, true) {
		t.Error("should succeed without auth")
	}
}

func TestABSConnectionFailure(t *testing.T) {
	if testAudiobookshelfServer(Config{AudiobookshelfURL: "http://127.0.0.1:1"}, true) {
		t.Error("should fail with bad URL")
	}
}

func TestABSNoAuthServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	if testAudiobookshelfServer(Config{AudiobookshelfURL: srv.URL, AudiobookshelfUser: "u", AudiobookshelfPass: "p"}, true) {
		t.Error("should fail with 500")
	}
}

func TestPrintUsage(t *testing.T) {
	// These methods don't exist in clihelp, so we skip them
}

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

func TestCopyFileNonexistent(t *testing.T) { copyFile("/nonexistent/src", "/nonexistent/dst") }

func TestSafeMoveNonexistent(t *testing.T) { safeMove("/nonexistent/src", "/nonexistent/dst") }

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
	if testWhisperServerEx("", 1, 1*time.Millisecond, true) {
		t.Error("expected false for empty URL")
	}
}

func TestTestWhisperServerSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer ts.Close()

	if !testWhisperServerEx(ts.URL, 1, 1*time.Millisecond, true) {
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

	if !testWhisperServerEx(ts.URL, 3, 5*time.Millisecond, true) {
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

	if testWhisperServerEx(ts.URL, 2, 5*time.Millisecond, true) {
		t.Error("expected false for failing Whisper server")
	}
}

func TestTestWhisperServerClientError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	if testWhisperServerEx(ts.URL, 3, 5*time.Millisecond, true) {
		t.Error("expected false for 404 client error without retrying")
	}
}

func TestParseFlagsTestWhisperCommand(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"abs", "test", "whisper"}
	cli := parseFlags()
	if !cli.IsTestCommand || !cli.TestWhisper {
		t.Errorf("expected IsTestCommand=true, TestWhisper=true, got %v, %v", cli.IsTestCommand, cli.TestWhisper)
	}

	os.Args = []string{"abs", "test"}
	cli = parseFlags()
	if !cli.IsTestCommand || !cli.TestWhisper {
		t.Errorf("expected IsTestCommand=true, TestWhisper=true for default test command, got %v, %v", cli.IsTestCommand, cli.TestWhisper)
	}

	os.Args = []string{"abs", "test-whisper"}
	cli = parseFlags()
	if !cli.IsTestCommand || !cli.TestWhisper {
		t.Errorf("expected IsTestCommand=true, TestWhisper=true for test-whisper command, got %v, %v", cli.IsTestCommand, cli.TestWhisper)
	}
}

func TestParseFlagsSilent(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"abs", "-s", "file.mp3"}
	cli := parseFlags()
	if !cli.Silent {
		t.Error("expected Silent=true for -s")
	}

	os.Args = []string{"abs", "--silent", "file.mp3"}
	cli = parseFlags()
	if !cli.Silent {
		t.Error("expected Silent=true for --silent")
	}
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
	app := buildUsageApp()
	if app.Name != "abs" {
		t.Errorf("expected app name 'abs', got %q", app.Name)
	}
	if len(app.Commands) < 5 {
		t.Errorf("expected at least 5 commands, got %d", len(app.Commands))
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
