package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

func wakeWhisperServer(whisperURL string, wakeCmd string, quiet bool) {
	if whisperURL == "" {
		return
	}
	if wakeCmd != "" {
		if !quiet {
			fmt.Printf("Running whisper wake command: %s\n", wakeCmd)
		}
		cmd := execCommand("/bin/sh", "-c", wakeCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil && !quiet {
			fmt.Printf("Warning: Whisper wake command failed: %v\n", err)
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

func testWhisperServer(whisperURL string, wakeCmd string, quiet bool) bool {
	return testWhisperServerEx(whisperURL, wakeCmd, 5, 3*time.Second, quiet)
}

func testWhisperServerEx(whisperURL string, wakeCmd string, maxRetries int, retryDelay time.Duration, quiet bool) bool {
	if whisperURL == "" {
		fmt.Println("ERROR: whisper_url is not configured in config file.")
		return false
	}

	if !quiet {
		fmt.Printf("Testing whisper server at: %s\n", whisperURL)
	}

	pcmData := make([]byte, 3200)
	header := buildWavHeader(len(pcmData))
	audioContent := append(header, pcmData...)

	var lastErr error
	var lastStatus int
	var lastResponseBody string

	client := &http.Client{Timeout: 10 * time.Second}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		wakeWhisperServer(whisperURL, wakeCmd, true)

		boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		w.SetBoundary(boundary)
		fw, _ := w.CreateFormFile("file", "test.wav")
		fw.Write(audioContent)
		w.WriteField("response_format", "verbose_json")
		w.WriteField("temperature", "0.0")
		w.Close()

		req, err := http.NewRequest("POST", whisperURL, &buf)
		if err != nil {
			if !quiet {
				fmt.Printf("ERROR: Failed to create request: %v\n", err)
			}
			return false
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				if !quiet {
					fmt.Printf("Attempt %d/%d: Connection error: %v (server may be sleeping, retrying in %v...)\n",
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
			fmt.Println("SUCCESS: Whisper server responded OK (200)")
			return true
		}

		if resp.StatusCode >= 500 && attempt < maxRetries {
			if !quiet {
				fmt.Printf("Attempt %d/%d: Server returned status %d: %s (server may be waking up, retrying in %v...)\n",
					attempt, maxRetries, resp.StatusCode, lastResponseBody, retryDelay)
			}
			time.Sleep(retryDelay)
			continue
		}

		fmt.Printf("FAIL: Server returned status %d: %s\n", resp.StatusCode, lastResponseBody)
		return false
	}

	if lastErr != nil {
		fmt.Printf("FAIL: Could not connect to Whisper server at '%s' after %d attempt(s): %v\n", whisperURL, maxRetries, lastErr)
	} else {
		fmt.Printf("FAIL: Server at '%s' returned status %d after %d attempt(s): %s\n", whisperURL, lastStatus, maxRetries, lastResponseBody)
	}
	return false
}
