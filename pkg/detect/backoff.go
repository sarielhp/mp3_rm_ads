package detect

import (
	"time"

	"pod/pkg/util"
)

var (
	backoffMu       util.Mutex
	backoffOverride func(attempt int) time.Duration
)

// SetRetryBackoff overrides the wait between LLM request attempts. Pass nil to
// restore the default.
//
// Tests set a zero backoff. Without this, exercising the retry path meant
// waiting it out: one test in pkg/detect and another in pkg/adremoval spent
// over three seconds each asleep, which was a fifth of the whole suite.
func SetRetryBackoff(f func(attempt int) time.Duration) {
	backoffMu.Lock()
	defer backoffMu.Unlock()
	backoffOverride = f
}

func retryBackoff(attempt int) time.Duration {
	backoffMu.Lock()
	f := backoffOverride
	backoffMu.Unlock()
	if f != nil {
		return f(attempt)
	}
	return time.Duration(1<<attempt) * 500 * time.Millisecond
}
