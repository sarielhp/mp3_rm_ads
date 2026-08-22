package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func isKittySupported() bool {
	term := os.Getenv("TERM")
	if strings.Contains(term, "kitty") {
		return true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("KITTY_PID") != "" {
		return true
	}
	if strings.Contains(os.Getenv("TERMINAL_EMULATOR"), "kitty") {
		return true
	}
	return false
}

func findCoverImage(podcastDir string) string {
	candidates := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "cover.webp",
		"folder.jpg", "folder.jpeg", "folder.png",
		"podcast.jpg", "podcast.jpeg", "podcast.png",
		"album.jpg", "album.jpeg", "album.png",
	}

	for _, cand := range candidates {
		p := filepath.Join(podcastDir, cand)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return p
		}
	}

	matches, err := filepath.Glob(filepath.Join(podcastDir, "*.jpg"))
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	matchesPng, err := filepath.Glob(filepath.Join(podcastDir, "*.png"))
	if err == nil && len(matchesPng) > 0 {
		return matchesPng[0]
	}

	return ""
}

func encodeKittyGraphics(imgData []byte, cols, rows int) string {
	if len(imgData) == 0 {
		return ""
	}

	b64 := base64.StdEncoding.EncodeToString(imgData)
	chunkSize := 4096
	var b strings.Builder

	for i := 0; i < len(b64); i += chunkSize {
		end := i + chunkSize
		m := 1
		if end >= len(b64) {
			end = len(b64)
			m = 0
		}
		chunk := b64[i:end]

		if i == 0 {
			ctrl := fmt.Sprintf("a=T,f=100,t=d,m=%d", m)
			if cols > 0 {
				ctrl += fmt.Sprintf(",c=%d", cols)
			}
			if rows > 0 {
				ctrl += fmt.Sprintf(",r=%d", rows)
			}
			b.WriteString(fmt.Sprintf("\x1b_G%s;%s\x1b\\", ctrl, chunk))
		} else {
			b.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", m, chunk))
		}
	}

	return b.String()
}

func encodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return encodeKittyGraphics(data, cols, rows), nil
}

func kittyClearGraphics() string {
	return "\x1b_Ga=d,d=a\x1b\\"
}
