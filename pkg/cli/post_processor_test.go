package cli

import (
	"bytes"
	"strings"
	"testing"
)

// A failing post-processor used to be silent: the error from cmd.Run() was
// discarded, so a broken pipeline step looked exactly like a working one.
func TestRunPostProcessorsReportsFailure(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer

	err := runPostProcessors(&out, &errOut, []string{"false"}, false)
	if err == nil {
		t.Fatal("a failing post-processor must be reported, not swallowed")
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("the error should name the processor, got: %v", err)
	}
	if !strings.Contains(errOut.String(), "failed") {
		t.Errorf("the failure should be written to stderr, got: %q", errOut.String())
	}
}

func TestRunPostProcessorsSucceedsQuietly(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer

	if err := runPostProcessors(&out, &errOut, []string{"true"}, false); err != nil {
		t.Fatalf("a successful processor should not error: %v", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("nothing should reach stderr on success, got: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Running post-processor: true") {
		t.Errorf("expected progress on stdout, got: %q", out.String())
	}
}

// One failure must not prevent the rest from running — the processors are
// independent steps, not a chain.
func TestRunPostProcessorsContinuesAfterAFailure(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer

	err := runPostProcessors(&out, &errOut, []string{"false", "echo second-ran"}, false)
	if err == nil {
		t.Fatal("expected the failure to be reported")
	}
	if !strings.Contains(out.String(), "second-ran") {
		t.Errorf("the second processor should still have run, got: %q", out.String())
	}
	if !strings.Contains(err.Error(), "1 of 2") {
		t.Errorf("the error should count the failures, got: %v", err)
	}
}

// The child's output goes to the supplied writers, not straight to the
// process streams, so a caller can capture or redirect it.
func TestRunPostProcessorsWritesChildOutputToTheSuppliedWriter(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer

	if err := runPostProcessors(&out, &errOut, []string{"echo hello-from-child"}, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hello-from-child") {
		t.Errorf("child stdout did not reach the supplied writer, got: %q", out.String())
	}
	// quiet suppressed the progress lines but not the child's own output
	if strings.Contains(out.String(), "Running post-processor") {
		t.Errorf("quiet should suppress progress, got: %q", out.String())
	}
}

func TestRunPostProcessorsSkipsBlankEntries(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := runPostProcessors(&out, &errOut, []string{"", "   "}, true); err != nil {
		t.Fatalf("blank entries should be skipped, got: %v", err)
	}
}
