package backend

import "fmt"

var (
	disabledMu             syncRWMutex
	AudiobookshelfDisabled bool
	PodfetchDisabled       bool
)

func verifyAudiobookshelfNotDisabled(op string) {
	disabledMu.RLock()
	disabled := AudiobookshelfDisabled
	disabledMu.RUnlock()
	if disabled {
		panic(fmt.Sprintf("FATAL: Runtime verification failed: Audiobookshelf operation '%s' blocked because Audiobookshelf is disabled", op))
	}
}

func verifyPodfetchNotDisabled(op string) {
	disabledMu.RLock()
	disabled := PodfetchDisabled
	disabledMu.RUnlock()
	if disabled {
		panic(fmt.Sprintf("FATAL: Runtime verification failed: Podfetch operation '%s' blocked because Podfetch is disabled", op))
	}
}

func SetAudiobookshelfDisabled(disabled bool) {
	disabledMu.Lock()
	AudiobookshelfDisabled = disabled
	disabledMu.Unlock()
}

func SetPodfetchDisabled(disabled bool) {
	disabledMu.Lock()
	PodfetchDisabled = disabled
	disabledMu.Unlock()
}
