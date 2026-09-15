package detect

import (
	"os"
	"testing"
	"time"
)

// TestMain removes the retry backoff. Exercising the retry path is the point
// of several tests here; waiting out 1s + 2s of real sleep to do it is not.
func TestMain(m *testing.M) {
	SetRetryBackoff(func(int) time.Duration { return 0 })
	os.Exit(m.Run())
}
