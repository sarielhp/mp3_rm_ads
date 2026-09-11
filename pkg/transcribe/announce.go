package transcribe

import (
	"abs/pkg/types"
	"abs/pkg/util"
	"fmt"
	"math"
	"net/url"
)

func AnnounceStart(duration float64, quiet bool) {
	if quiet {
		return
	}
	length := "unknown"
	if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
		minutes := int(duration / 60)
		length = fmt.Sprintf("%2d:%02d", minutes/60, minutes%60)
	}
	fmt.Println("\n" + util.BoldYellow("--transcribing---"))
	fmt.Println(util.BoldCyan("Episode length: " + length))
}

func AnnounceWhisperServer(endpoint string, engine types.WhisperEngine, container string, quiet bool) {
	if quiet {
		return
	}
	label := "Transcription: Whisper HTTP server"
	if engine == types.WhisperEngineDocker {
		label = "Transcription: Whisper in DOCKER (HTTP server)"
		if container != "" {
			label += " [container: " + container + "]"
		}
	} else if engine == types.WhisperEngineRemote {
		label = "Transcription: REMOTE Whisper HTTP server"
	}
	label += " (model selected by server)"
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		label += " on " + u.Hostname()
	}
	fmt.Println("\n" + util.BoldCyan(label))
}
