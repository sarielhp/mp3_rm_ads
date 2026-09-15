package transcribe

import (
	"testing"

	"pod/pkg/util"
)

// extraWhisperModelDirs used to be a bare package variable, and a slice at
// that: a reader could observe a half-assigned header. Run under -race, this
// fails if the guard is removed.
func TestExtraWhisperModelDirsIsRaceFree(t *testing.T) {
	t.Cleanup(func() { SetExtraWhisperModelDirs(nil) })

	var wg util.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 200 {
				SetExtraWhisperModelDirs([]string{"/a", "/b"})
				_ = GetWhisperModelSearchDirs()
			}
		}(i)
	}
	wg.Wait()
}
