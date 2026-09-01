package main

import (
	"encoding/base64"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	"os"
	"strings"
)

func isKittyTerminal() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("KITTY_PID") != "" {
		return true
	}
	term := os.Getenv("TERM")
	if strings.Contains(term, "kitty") || strings.Contains(term, "wezterm") || strings.Contains(term, "ghostty") {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" || os.Getenv("WEZTERM_PANE") != "" {
		return true
	}
	return false
}

func isKittySupported() bool {
	return true
}

var (
	podcastCoverPathCache   = make(map[string]string)
	podcastCoverPathCacheMu syncMutex
	coverGraphicsCache      = make(map[string]string)
	coverGraphicsCacheMu    syncMutex
	coverPngMemoryCache     = make(map[string][]byte)
	coverPngMemoryCacheMu   syncMutex
)

func clearImageMemoryCache() {
	podcastCoverPathCacheMu.Lock()
	podcastCoverPathCache = make(map[string]string)
	podcastCoverPathCacheMu.Unlock()

	coverGraphicsCacheMu.Lock()
	coverGraphicsCache = make(map[string]string)
	coverGraphicsCacheMu.Unlock()

	coverPngMemoryCacheMu.Lock()
	coverPngMemoryCache = make(map[string][]byte)
	coverPngMemoryCacheMu.Unlock()
}

func encodeNativeKittyGraphics(filePath string, cols, rows int) (string, error) {
	pngData, err := getOrCacheCoverPNG(filePath)
	if err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(pngData)

	var sb strings.Builder
	chunkSize := 4096
	totalChunks := (len(b64) + chunkSize - 1) / chunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(b64) {
			end = len(b64)
		}
		chunk := b64[start:end]
		m := 0
		if i < totalChunks-1 {
			m = 1
		}

		if i == 0 {
			if cols > 0 && rows > 0 {
				sb.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d,C=1,q=2,m=%d;%s\x1b\\", cols, rows, m, chunk))
			} else if cols > 0 {
				sb.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,C=1,q=2,m=%d;%s\x1b\\", cols, m, chunk))
			} else {
				sb.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,C=1,q=2,m=%d;%s\x1b\\", m, chunk))
			}
		} else {
			sb.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", m, chunk))
		}
	}

	return sb.String(), nil
}
