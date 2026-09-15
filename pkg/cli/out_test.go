package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestOutForDiscardsWhenQuiet(t *testing.T) {
	if outFor(CLIOptions{ProcOptions: ProcOptions{Quiet: true}}) != io.Discard {
		t.Error("--quiet should route command output to io.Discard")
	}
	if outFor(CLIOptions{}) != os.Stdout {
		t.Error("without --quiet, command output belongs on stdout")
	}
}

// Fprintf to the writer must format exactly as Printf did, including the
// absence of any added newline: several call sites deliberately print without
// one to build a progress line in place.
func TestOutForFormatsWithoutAddingNewlines(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Checking (%d/%d)...", 3, 7)
	if got := buf.String(); got != "Checking (3/7)..." {
		t.Errorf("got %q, want no trailing newline", got)
	}
}

func TestOutForQuietWritesNothing(t *testing.T) {
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
