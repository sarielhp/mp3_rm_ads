package kitty

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eliukblau/pixterm/pkg/ansimage"
)

var (
	podcastCoverPathCache   = make(map[string]string)
	podcastCoverPathCacheMu sync.Mutex
	coverGraphicsCache      = make(map[string]string)
	coverGraphicsCacheMu    sync.Mutex
	coverPngMemoryCache     = make(map[string][]byte)
	coverPngMemoryCacheMu   sync.Mutex
)

func ClearImageMemoryCache() {
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

func EncodeNativeKittyGraphics(filePath string, cols, rows int) (string, error) {
	pngData, err := GetOrCacheCoverPNG(filePath)
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

func EncodeKittyGraphics(imgData []byte, cols, rows int, format int) string {
	if len(imgData) == 0 {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)
	return fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d;%s\x1b\\", cols, rows, b64)
}

func KittyClearGraphics() string {
	return "\x1b_Ga=d,d=A,q=2\x1b\\"
}

func EncodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	if cols <= 0 {
		cols = 30
	}
	if rows <= 0 {
		rows = cols
	}

	fi, statErr := os.Stat(filePath)
	var mtime, size int64
	if statErr == nil {
		mtime = fi.ModTime().UnixNano()
		size = fi.Size()
	}

	isKitty := IsKittyTerminal()
	key := fmt.Sprintf("%s:%d:%d:%d:%d:%v", filePath, mtime, size, cols, rows, isKitty)

	coverGraphicsCacheMu.Lock()
	if cached, ok := coverGraphicsCache[key]; ok {
		coverGraphicsCacheMu.Unlock()
		return cached, nil
	}
	coverGraphicsCacheMu.Unlock()

	cDir := PodcastCacheDirForImage(filePath)
	modeStr := "ansi"
	if isKitty {
		modeStr = "kitty"
	}
	diskCachePath := filepath.Join(cDir, fmt.Sprintf("cover_%dx%d_%s.esc", cols, rows, modeStr))

	if diskFi, err := os.Stat(diskCachePath); err == nil && diskFi.Size() > 0 {
		if statErr != nil || !diskFi.ModTime().Before(fi.ModTime()) {
			if diskData, err := os.ReadFile(diskCachePath); err == nil && len(diskData) > 0 {
				str := string(diskData)
				coverGraphicsCacheMu.Lock()
				coverGraphicsCache[key] = str
				coverGraphicsCacheMu.Unlock()
				return str, nil
			}
		}
	}

	result, err := renderGraphicsContent(filePath, cols, rows, isKitty)
	if err == nil && result != "" {
		coverGraphicsCacheMu.Lock()
		coverGraphicsCache[key] = result
		coverGraphicsCacheMu.Unlock()

		_ = os.WriteFile(diskCachePath, []byte(result), 0644)
	}

	return result, err
}

func renderGraphicsContent(filePath string, cols, rows int, isKitty bool) (string, error) {
	if isKitty {
		return EncodeNativeKittyGraphics(filePath, cols, rows)
	}

	pngBytes, pErr := GetOrCacheCoverPNG(filePath)
	if pErr == nil {
		if img, _, dErr := image.Decode(bytes.NewReader(pngBytes)); dErr == nil {
			ai, err := ansimage.NewScaledFromImage(img, rows, cols, color.Black, ansimage.ScaleModeResize, ansimage.NoDithering)
			if err == nil {
				return ai.Render(), nil
			}
		}
	}

	ai, err := ansimage.NewScaledFromFile(filePath, rows, cols, color.Black, ansimage.ScaleModeResize, ansimage.NoDithering)
	if err != nil {
		return "", err
	}
	return ai.Render(), nil
}

func DetectImageFormat(path string) int {
	return 100
}
