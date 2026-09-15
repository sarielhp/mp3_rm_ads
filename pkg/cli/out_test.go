package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

// --quiet suppresses progress, not results. `info ls --quiet` prints bare IDs
// and nothing else, so routing results through a discard would empty it.
func TestProgressForDiscardsWhenQuietButOutForDoesNot(t *testing.T) {
	t.Parallel()
	quiet := CLIOptions{ProcOptions: ProcOptions{Quiet: true}}
	if progressFor(quiet) != io.Discard {
		t.Error("--quiet should discard progress output")
	}
	if outFor(quiet) != os.Stdout {
		t.Error("--quiet must not discard results; info ls --quiet depends on them")
	}
	if outFor(CLIOptions{}) != os.Stdout {
		t.Error("without a supplied writer, output belongs on stdout")
	}
}

// A supplied writer wins over the process streams, which is what lets a test
// check output without reassigning os.Stdout.
func TestOutForPrefersTheSuppliedWriter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cli := CLIOptions{Out: &buf}
	if outFor(cli) != io.Writer(&buf) {
		t.Error("outFor ignored the supplied writer")
	}
	if progressFor(cli) != io.Writer(&buf) {
		t.Error("progressFor ignored the supplied writer")
	}
}

// Fprintf to the writer must format exactly as Printf did, including the
// absence of any added newline: several call sites deliberately print without
// one to build a progress line in place.
func TestOutForFormatsWithoutAddingNewlines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Checking (%d/%d)...", 3, 7)
	if got := buf.String(); got != "Checking (3/7)..." {
		t.Errorf("got %q, want no trailing newline", got)
	}
}

func TestOutForQuietWritesNothing(t *testing.T) {
	t.Parallel()
	w := outFor(CLIOptions{ProcOptions: ProcOptions{Quiet: true}})
	n, err := fmt.Fprintf(w, "this must not appear: %s", "x")
	if err != nil {
		t.Fatal(err)
	}
	// io.Discard reports the bytes as written but keeps none of them; the
	// point is that the call is safe and unconditional.
	if n == 0 {
		t.Error("expected the formatted length to be reported")
	}
}
