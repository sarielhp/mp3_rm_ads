package main

import (
	"os/exec"
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
