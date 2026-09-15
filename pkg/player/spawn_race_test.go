package player

import (
	"testing"

	"pod/pkg/util"
)

// playerSpawnEnabled used to be a bare package variable: tests wrote it while
// production code read it from whatever goroutine was handling playback. Run
// under -race, this fails if the guard is removed.
func TestPlayerSpawnFlagIsRaceFree(t *testing.T) {
	orig := playerSpawnAllowed()
	t.Cleanup(func() { SetPlayerSpawnEnabled(orig) })

	var wg util.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 200 {
				SetPlayerSpawnEnabled(n%2 == 0)
				_ = IsAudioSpawnDisabled()
			}
		}(i)
	}
	wg.Wait()
}
