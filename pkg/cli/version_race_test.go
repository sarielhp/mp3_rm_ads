package cli

import (
	"testing"

	"pod/pkg/util"
)

// embeddedVersion used to be a bare package variable, written by main and by
// tests, read whenever a command renders its version. Run under -race, this
// fails if the guard is removed.
func TestEmbeddedVersionIsRaceFree(t *testing.T) {
	orig := embeddedVersionValue()
	t.Cleanup(func() { SetEmbeddedVersion(orig) })

	var wg util.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 200 {
				SetEmbeddedVersion("0.0." + string(rune('0'+n)))
				_ = embeddedVersionValue()
			}
		}(i)
	}
	wg.Wait()
}
