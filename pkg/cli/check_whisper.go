package cli

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"pod/pkg/transcribe"
	"time"
)

func testWhisperServer(w io.Writer, whisperURL string, wakeCmd string, quiet bool) bool {
	return testWhisperServerEx(w, whisperURL, wakeCmd, 5, 3*time.Second, quiet)
}

func testWhisperServerEx(w io.Writer, whisperURL string, wakeCmd string, maxRetries int, retryDelay time.Duration, quiet bool) bool {
	if whisperURL == "" {
		fmt.Fprintln(w, "ERROR: whisper_url is not configured in config file.")
		return false
	}
	if !quiet {
		fmt.Fprintf(w, "Testing whisper server at: %s\n", whisperURL)
	}

	var lastErr error
	var lastStatus int
	var lastResponseBody string
	client := &http.Client{Timeout: 10 * time.Second}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		wakeWhisperServer(whisperURL, wakeCmd, true)
		bodyData, contentType := buildTestWavPayload()

		req, err := http.NewRequest("POST", whisperURL, bytes.NewReader(bodyData))
		if err != nil {
			if !quiet {
				fmt.Fprintf(w, "ERROR: Failed to create request: %v\n", err)
			}
			return false
		}
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				if !quiet {
					fmt.Fprintf(w, "Attempt %d/%d: Connection error: %v (server may be sleeping, retrying in %v...)\n",
						attempt, maxRetries, err, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		lastResponseBody = string(body)

		if resp.StatusCode == http.StatusOK {
			fmt.Fprintln(w, "SUCCESS: Whisper server responded OK (200)")
			return true
		}

		if resp.StatusCode >= 500 && attempt < maxRetries {
			if !quiet {
				fmt.Fprintf(w, "Attempt %d/%d: Server returned status %d: %s (server may be waking up, retrying in %v...)\n",
					attempt, maxRetries, resp.StatusCode, lastResponseBody, retryDelay)
			}
			time.Sleep(retryDelay)
			continue
		}

		fmt.Fprintf(w, "FAIL: Server returned status %d: %s\n", resp.StatusCode, lastResponseBody)
		return false
	}

	if lastErr != nil {
		fmt.Fprintf(w, "FAIL: Could not connect to Whisper server at '%s' after %d attempt(s): %v\n", whisperURL, maxRetries, lastErr)
	} else {
		fmt.Fprintf(w, "FAIL: Server at '%s' returned status %d after %d attempt(s): %s\n", whisperURL, lastStatus, maxRetries, lastResponseBody)
	}
	return false
}

func buildTestWavPayload() ([]byte, string) {
	pcmData := make([]byte, 3200)
	header := transcribe.BuildWavHeader(len(pcmData))
	audioContent := append(header, pcmData...)

	boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.SetBoundary(boundary)
	fw, _ := w.CreateFormFile("file", "test.wav")
	fw.Write(audioContent)
	w.WriteField("response_format", "verbose_json")
	w.WriteField("temperature", "0.0")
	w.Close()

	return buf.Bytes(), w.FormDataContentType()
}

func wakeWhisperServer(whisperURL string, wakeCmd string, quiet bool) {
	transcribe.WakeServer(whisperURL, wakeCmd, quiet)
}
