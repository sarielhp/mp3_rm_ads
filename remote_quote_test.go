package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var hostileStrings = []string{
	"Tech Talk `whoami`/ep.mp3",
	"Show $(id)/ep.mp3",
	"A;rm -rf ~/ep.mp3",
	"My Podcast/Ep 01.mp3",
	"quote'inside/ep.mp3",
	"back\\slash/ep.mp3",
	"newline\nin/ep.mp3",
	"tab\tin/ep.mp3",
	"$HOME/ep.mp3",
	"a&&b/ep.mp3",
	"pipe|d/ep.mp3",
}

// The real test: hand the quoted string to a shell and require it back verbatim.
func TestShellQuoteSurvivesARealShell(t *testing.T) {
	for _, s := range hostileStrings {
		out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(s)).Output()
		if err != nil {
			t.Errorf("%q: shell rejected the quoted form: %v", s, err)
			continue
		}
		if string(out) != s {
			t.Errorf("%q: shell produced %q; metacharacters were interpreted", s, out)
		}
	}
}

func TestRemoteCleanupCommandPassesExactlyThreeOperands(t *testing.T) {
	// An ordinary podcast name, with the spaces sanitizePodcastName preserves.
	rel := "My Podcast/Ep 01.mp3"
	cmd := remoteCleanupCommand("~/abs_remote", rel)

	out, err := exec.Command("sh", "-c",
		"set -- "+strings.TrimPrefix(cmd, "rm -f ")+`; for a in "$@"; do printf '%s\n' "$a"; done`).Output()
	if err != nil {
		t.Fatalf("shell rejected %q: %v", cmd, err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 operands, the shell saw %d: %q", len(lines), lines)
	}
	for _, l := range lines {
		if !strings.Contains(l, "My Podcast/Ep 01.mp3") {
			t.Errorf("operand %q lost the path", l)
		}
	}
}

func TestRemoteCleanupCommandNeutralisesInjection(t *testing.T) {
	cmd := remoteCleanupCommand("~/abs_remote", "x`touch /tmp/abs_pwned_marker`/ep.mp3")
	if strings.Contains(cmd, "`touch") && !strings.Contains(cmd, "'") {
		t.Errorf("command substitution left unquoted: %s", cmd)
	}
	out, err := exec.Command("sh", "-c",
		"set -- "+strings.TrimPrefix(cmd, "rm -f ")+`; printf '%s\n' "$1"`).Output()
	if err != nil {
		t.Fatalf("shell rejected %q: %v", cmd, err)
	}
	if !strings.Contains(string(out), "`touch /tmp/abs_pwned_marker`") {
		t.Errorf("the backtick expression did not survive as literal text: %q", out)
	}
}

func TestSafeRelUnderRejectsEscapes(t *testing.T) {
	base := "/srv/podcasts"
	cases := []struct {
		rel string
		ok  bool
	}{
		{"Show/ep.mp3", true},
		{"a/b/c.mp3", true},
		{"../escape.mp3", false},
		{"a/../../escape.mp3", false},
		{"../../../../.ssh/authorized_keys", false},
		{"/etc/passwd", false},
		{"", false},
		{"..", false},
	}
	for _, tc := range cases {
		got, ok := safeRelUnder(base, tc.rel)
		if ok != tc.ok {
			t.Errorf("safeRelUnder(%q, %q) ok=%v, want %v (got %q)", base, tc.rel, ok, tc.ok, got)
			continue
		}
		if ok && !strings.HasPrefix(got, base+"/") {
			t.Errorf("safeRelUnder(%q, %q) escaped: %q", base, tc.rel, got)
		}
	}
}

func TestResolveManifestDestRejectsPathsOutsideTheLibrary(t *testing.T) {
	base := "/srv/podcasts"
	if _, ok := resolveManifestDest(base, RemoteBatchJobItem{SourceFile: "/home/someone/.config/abs/config.json"}); ok {
		t.Errorf("an absolute source_file outside the library was accepted")
	}
	if _, ok := resolveManifestDest(base, RemoteBatchJobItem{RelativePath: "../../etc/cron.d/x"}); ok {
		t.Errorf("a traversing relative_path was accepted")
	}
	if p, ok := resolveManifestDest(base, RemoteBatchJobItem{RelativePath: "Show/ep.mp3"}); !ok || p != "/srv/podcasts/Show/ep.mp3" {
		t.Errorf("a legitimate relative_path was rejected: %q ok=%v", p, ok)
	}
	if p, ok := resolveManifestDest(base, RemoteBatchJobItem{SourceFile: "/srv/podcasts/Show/ep.mp3"}); !ok || p != "/srv/podcasts/Show/ep.mp3" {
		t.Errorf("a legitimate in-library source_file was rejected: %q ok=%v", p, ok)
	}
}

func TestValidateBatchID(t *testing.T) {
	valid := []string{
		"batch-20260906-123456-abcdef12",
		"test-batch-001",
		"batch_pull_test",
		"Job123",
		"a-b_c",
	}
	for _, id := range valid {
		if !validateBatchID(id) {
			t.Errorf("validateBatchID(%q) = false, want true", id)
		}
	}

	invalid := []string{
		"",
		" ",
		".",
		"..",
		"../escape",
		"../../var/log",
		"x'; touch /tmp/pwned; #",
		"$(reboot)",
		"`whoami`",
		"batch;rm -rf /",
		"batch|pipe",
		"batch&bg",
		"batch\nnewline",
		"batch with spaces",
		"batch/slash",
		"batch\\backslash",
		"batch*wildcard",
		"batch?query",
	}
	for _, id := range invalid {
		if validateBatchID(id) {
			t.Errorf("validateBatchID(%q) = true, want false", id)
		}
	}
}

func TestRunRemoteCancelRejectsHostileBatchID(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	cfg := &Config{
		RemoteHost:    "status-box",
		RemoteWorkDir: filepath.Join(tempDir, "remote"),
	}

	hostileIDs := []string{
		"../../tmp",
		"x'; touch /tmp/abs_pwned; #",
		"$(reboot)",
		"`whoami`",
		"batch; rm -rf /",
	}

	for _, id := range hostileIDs {
		err := runRemoteCancel(cfg, "status-box", id, mock, true)
		if err == nil {
			t.Errorf("runRemoteCancel accepted hostile batch ID %q without error", id)
		}
		for _, cmd := range mock.ExecutedCmds {
			if strings.Contains(cmd, "touch /tmp/abs_pwned") || strings.Contains(cmd, "reboot") {
				t.Fatalf("hostile payload executed in command: %s", cmd)
			}
		}
	}
}

func TestRemoteQuotedCommandsAvoidShellInjection(t *testing.T) {
	for _, hostile := range hostileStrings {
		catCmd := fmt.Sprintf("printf %%s %s", shellQuoteHomePath(hostile))
		out, err := exec.Command("sh", "-c", catCmd).Output()
		if err != nil {
			t.Errorf("shell failed to execute quoted cat command for %q: %v", hostile, err)
			continue
		}
		if string(out) != hostile && !strings.HasPrefix(hostile, "$HOME/") {
			t.Errorf("shell evaluated command with side effects for %q: got %q", hostile, string(out))
		}
	}
}

func TestRemoteFFmpegCommandQuoting(t *testing.T) {
	remIn := ".work/abs_123_456_in.part 1; reboot.mp3"
	remOut := ".work/abs_123_456_out.mp3"
	filter := "[0:a]atrim=start=0.000:end=10.000,asetpts=PTS-STARTPTS[a0];[a0]concat=n=1:v=0:a=1[aout]"

	cleanupCmd := buildRemoteCutCleanupCmd(remIn, remOut)
	if !strings.Contains(cleanupCmd, shellQuote(remIn)) {
		t.Errorf("cleanupCmd did not shellQuote remIn: %s", cleanupCmd)
	}

	ffmpegCmd := buildRemoteFFmpegCmd(remIn, filter, remOut)
	if !strings.Contains(ffmpegCmd, shellQuote(remIn)) {
		t.Errorf("ffmpegCmd did not shellQuote remIn: %s", ffmpegCmd)
	}
	if !strings.Contains(ffmpegCmd, shellQuote(filter)) {
		t.Errorf("ffmpegCmd did not shellQuote filter: %s", ffmpegCmd)
	}

	testScript := fmt.Sprintf("cmd=%s; [ \"$cmd\" != \"\" ]", shellQuote(ffmpegCmd))
	if err := exec.Command("sh", "-c", testScript).Run(); err != nil {
		t.Errorf("sh rejected shell-quoted ffmpeg command: %v", err)
	}
}
