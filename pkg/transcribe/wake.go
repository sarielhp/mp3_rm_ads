package transcribe

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// WakeServer runs the configured wake command and waits for the Whisper
// server to answer, so the first transcription does not race a cold start.
func WakeServer(whisperURL string, wakeCmd string, quiet bool) {
	if whisperURL == "" {
		return
	}
	if wakeCmd != "" {
		if !quiet {
			fmt.Printf("Running whisper wake command: %s\n", wakeCmd)
		}
		cmd := exec.Command("/bin/sh", "-c", wakeCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil && !quiet {
			fmt.Printf("\nWarning: Whisper wake command failed: %v\n\n", err)
		}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", whisperURL, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
