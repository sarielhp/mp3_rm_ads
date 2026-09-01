#!/usr/bin/env ruby
require 'fileutils'

docker_go = File.read("docker.go")

File.write("openrouter.go", <<~GO)
package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type OpenRouterModel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context int    `json:"context_length"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

type OpenRouterResponse struct {
	Data []OpenRouterModel `json:"data"`
}

var openRouterModelsCache []OpenRouterModel

func fetchOpenRouterModels() []OpenRouterModel {
	openRouterCacheMu.Lock()
	defer openRouterCacheMu.Unlock()
	if openRouterModelsCache != nil {
		return openRouterModelsCache
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		openRouterModelsCache = []OpenRouterModel{}
		return openRouterModelsCache
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var orResp OpenRouterResponse
		if err := json.NewDecoder(resp.Body).Decode(&orResp); err == nil {
			openRouterModelsCache = orResp.Data
			return openRouterModelsCache
		}
	}

	openRouterModelsCache = []OpenRouterModel{}
	return openRouterModelsCache
}
GO

File.write("whisper_docker.go", <<~GO)
package main

import (
	"fmt"
)

func pollWhisperDockerProgress(containerName string) interface{} {
	if containerName == "" {
		return nil
	}

	logText := fetchDockerLogs(containerName, 20)
	if logText == "" {
		return nil
	}

	hasFailed := false
	for _, line := range splitLines(logText) {
		if matchFailedDecode(line) {
			hasFailed = true
		} else if h, m, s, ok := matchProgressHMS(line); ok {
			return float64(h*3600 + m*60 + s)
		} else if m, s, ok := matchProgressMS(line); ok {
			return float64(m*60 + s)
		} else if pct, ok := matchProgressPercent(line); ok {
			return pct / 100.0
		}
	}
	if hasFailed {
		return failedToDecodeSentinel
	}
	return nil
}

type failedToDecodeType struct{}

var failedToDecodeSentinel = &failedToDecodeType{}

func detectWhisperDockerContainer(whisperURL string) string {
	host := extractHost(whisperURL)
	if !isLocalHost(host) {
		return ""
	}

	dockerMu.Lock()
	cmd := execCommand("docker", "ps", "--format", "{{.ID}}\\t{{.Image}}\\t{{.Names}}")
	output, err := cmd.Output()
	dockerMu.Unlock()
	if err != nil {
		return ""
	}

	type containerInfo struct {
		id    string
		image string
		name  string
	}
	var containers []containerInfo
	for _, line := range splitLines(string(output)) {
		parts := splitTab(line)
		if len(parts) >= 3 {
			containers = append(containers, containerInfo{id: parts[0], image: parts[1], name: parts[2]})
		}
	}

	for _, c := range containers {
		if contains(toLower(c.image), "whisper") || contains(toLower(c.name), "whisper") {
			return c.name
		}
	}
	return ""
}

func matchFailedDecode(line string) bool {
	lower := toLower(line)
	return contains(lower, "failed to decode") || contains(lower, "failed to encode")
}

func matchProgressHMS(line string) (h, m, s int, ok bool) {
	_, err := fmt.Sscanf(line, "processing audio (%d:%d:%d", &h, &m, &s)
	if err == nil {
		return h, m, s, true
	}
	for i := 0; i < len(line)-10; i++ {
		if line[i:i+10] == "processing" {
			_, err := fmt.Sscanf(line[i:], "processing audio (%d:%d:%d", &h, &m, &s)
			if err == nil {
				return h, m, s, true
			}
			break
		}
	}
	return 0, 0, 0, false
}

func matchProgressMS(line string) (m, s int, ok bool) {
	for i := 0; i < len(line)-10; i++ {
		if line[i:i+10] == "processing" {
			_, err := fmt.Sscanf(line[i:], "processing audio (%d:%d", &m, &s)
			if err == nil {
				return m, s, true
			}
			break
		}
	}
	return 0, 0, false
}

func matchProgressPercent(line string) (float64, bool) {
	var pct float64
	_, err := fmt.Sscanf(line, "progress_common: %f%%", &pct)
	if err == nil {
		return pct, true
	}
	for i := 0; i < len(line)-10; i++ {
		if line[i:i+10] == "progress_c" {
			_, err := fmt.Sscanf(line[i:], "progress_common: %f%%", &pct)
			if err == nil {
				return pct, true
			}
			break
		}
	}
	return 0, false
}
GO

File.write("string_utils.go", <<~GO)
package main

import (
	"net"
)

func extractHost(url string) string {
	protoEnd := -1
	for i := 0; i < len(url)-2; i++ {
		if url[i:i+3] == "://" {
			protoEnd = i + 3
			break
		}
	}
	if protoEnd < 0 {
		protoEnd = 0
	}
	hostEnd := protoEnd
	for hostEnd < len(url) && url[hostEnd] != '/' && url[hostEnd] != ':' {
		hostEnd++
	}
	return url[protoEnd:hostEnd]
}

func extractPort(url string) string {
	protoEnd := -1
	for i := 0; i < len(url)-2; i++ {
		if url[i:i+3] == "://" {
			protoEnd = i + 3
			break
		}
	}
	if protoEnd < 0 {
		protoEnd = 0
	}
	hostEnd := protoEnd
	for hostEnd < len(url) && url[hostEnd] != '/' && url[hostEnd] != ':' {
		hostEnd++
	}
	if hostEnd < len(url) && url[hostEnd] == ':' {
		portEnd := hostEnd + 1
		for portEnd < len(url) && url[portEnd] >= '0' && url[portEnd] <= '9' {
			portEnd++
		}
		return url[hostEnd+1 : portEnd]
	}
	return ""
}

func isLocalHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	}
	addrs, err := netInterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.String() == host {
				return true
			}
		}
	}
	return false
}

var netInterfaceAddrs = net.InterfaceAddrs

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitTab(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\t' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
GO

File.write("docker.go", <<~GO)
package main

import (
	"fmt"
	"os/exec"
)

func fetchDockerLogs(containerName string, tail int) string {
	if containerName == "" {
		return ""
	}
	dockerMu.Lock()
	defer dockerMu.Unlock()

	cmd := execCommand("docker", "logs", "--tail", fmt.Sprintf("%d", tail), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}

var execCommand = exec.Command
GO

