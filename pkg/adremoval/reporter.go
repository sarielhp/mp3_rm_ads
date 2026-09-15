package adremoval

import (
	"os"

	"pod/pkg/progress"
)

// stdoutReporter adapts this package's quiet flag to the progress.Reporter
// that library calls now take.
func stdoutReporter(quiet bool) progress.Reporter {
	if quiet {
		return progress.Discard
	}
	return progress.Writer(os.Stdout, os.Stderr, false)
}
