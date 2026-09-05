package backend

import "fmt"

var AudiobookshelfDisabled bool
var PodfetchDisabled bool

func verifyAudiobookshelfNotDisabled(op string) {
	if AudiobookshelfDisabled {
		panic(fmt.Sprintf("FATAL: Runtime verification failed: Audiobookshelf operation '%s' blocked because Audiobookshelf is disabled", op))
	}
}

func verifyPodfetchNotDisabled(op string) {
	if PodfetchDisabled {
		panic(fmt.Sprintf("FATAL: Runtime verification failed: Podfetch operation '%s' blocked because Podfetch is disabled", op))
	}
}

func SetAudiobookshelfDisabled(disabled bool) {
	AudiobookshelfDisabled = disabled
}

func SetPodfetchDisabled(disabled bool) {
	PodfetchDisabled = disabled
}
