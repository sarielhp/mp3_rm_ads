package tui

import (
	"os"
	"testing"
	"time"

	"pod/pkg/podcast"
	"pod/pkg/podcast/podtest"
)

// TestMain redirects the user cache directory into a throwaway one. Several
// code paths reach a process-wide cache keyed off XDG_CACHE_HOME, and a test
// run must never write into the real one.
func TestMain(m *testing.M) {
	// Tests must not reach the network, and must not wait out a retry backoff.
	zero := time.Duration(0)
	podcast.SetFeedRetryDelay(&zero)
	podcast.SetFeedTransport(podtest.OfflineTransport())

	dir, err := os.MkdirTemp("", "abs-test-cache-")
	if err == nil {
		os.Setenv("XDG_CACHE_HOME", dir)
	}
	code := m.Run()
	if dir != "" {
		// os.Exit skips deferred calls, so clean up before it.
		os.RemoveAll(dir)
	}
	os.Exit(code)
}
