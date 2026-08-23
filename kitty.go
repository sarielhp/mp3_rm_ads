package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"golang.org/x/term"
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
	podcastCoverPathCacheMu sync.Mutex
	coverGraphicsCache      = make(map[string]string)
	coverGraphicsCacheMu    sync.Mutex
)

func findCoverImage(podcastDir string) string {
	podcastCoverPathCacheMu.Lock()
	if cached, ok := podcastCoverPathCache[podcastDir]; ok {
		podcastCoverPathCacheMu.Unlock()
		return cached
	}
	podcastCoverPathCacheMu.Unlock()

	res := findCoverImageUncached(podcastDir)

	podcastCoverPathCacheMu.Lock()
	podcastCoverPathCache[podcastDir] = res
	podcastCoverPathCacheMu.Unlock()
	return res
}

func findCoverImageUncached(podcastDir string) string {
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

	cDir := cacheDirForPodcast(podcastDir)
	for _, cand := range candidates {
		p := filepath.Join(cDir, cand)
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

func encodeNativeKittyGraphics(filePath string, cols, rows int) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	pngData := data
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err == nil {
			pngData = buf.Bytes()
		}
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

func podcastCacheDirForImage(imagePath string) string {
	dir := filepath.Dir(imagePath)
	base := cacheBaseDir()
	if strings.HasPrefix(filepath.Clean(dir), filepath.Clean(base)) {
		return dir
	}
	return cacheDirForPodcast(dir)
}

func encodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	if cols <= 0 {
		cols = 30
	}
	if rows <= 0 {
		rows = cols
	}

	fi, statErr := os.Stat(filePath)
	var mtime int64
	var size int64
	if statErr == nil {
		mtime = fi.ModTime().UnixNano()
		size = fi.Size()
	}

	isKitty := isKittyTerminal()
	key := fmt.Sprintf("%s:%d:%d:%d:%d:%v", filePath, mtime, size, cols, rows, isKitty)

	// 1. Check L1 in-memory cache
	coverGraphicsCacheMu.Lock()
	if cached, ok := coverGraphicsCache[key]; ok {
		coverGraphicsCacheMu.Unlock()
		return cached, nil
	}
	coverGraphicsCacheMu.Unlock()

	// 2. Check L2 on-disk cache in the podcast's cache directory
	cDir := podcastCacheDirForImage(filePath)
	modeStr := "ansi"
	if isKitty {
		modeStr = "kitty"
	}
	diskCachePath := filepath.Join(cDir, fmt.Sprintf("cover_%dx%d_%s.esc", cols, rows, modeStr))

	if diskFi, err := os.Stat(diskCachePath); err == nil && diskFi.Size() > 0 {
		if statErr != nil || diskFi.ModTime().After(fi.ModTime()) || diskFi.ModTime().Equal(fi.ModTime()) {
			if diskData, err := os.ReadFile(diskCachePath); err == nil && len(diskData) > 0 {
				str := string(diskData)
				coverGraphicsCacheMu.Lock()
				coverGraphicsCache[key] = str
				coverGraphicsCacheMu.Unlock()
				return str, nil
			}
		}
	}

	// 3. Process image
	var result string
	var err error

	if isKitty {
		result, err = encodeNativeKittyGraphics(filePath, cols, rows)
	} else {
		var ai *ansimage.ANSImage
		ai, err = ansimage.NewScaledFromFile(filePath, rows, cols, color.Black, ansimage.ScaleModeFit, ansimage.NoDithering)
		if err == nil {
			result = ai.Render()
		}
	}

	if err == nil && result != "" {
		// Save to L1 memory cache
		coverGraphicsCacheMu.Lock()
		coverGraphicsCache[key] = result
		coverGraphicsCacheMu.Unlock()

		// Save to L2 disk cache inside the podcast's cache directory
		_ = os.WriteFile(diskCachePath, []byte(result), 0644)
	}

	return result, err
}

func encodeKittyGraphics(imgData []byte, cols, rows int, format int) string {
	if len(imgData) == 0 {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)
	return fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d;%s\x1b\\", cols, rows, b64)
}

func kittyClearGraphics() string {
	return "\x1b_Ga=d,d=A,q=2\x1b\\"
}

func testKittyImage(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: abs test kitty <image-file>")
		fmt.Println()
		fmt.Println("Displays an image using ANSI true-color half-block rendering.")
		fmt.Println("Supported formats: PNG, JPEG, GIF, BMP, TIFF, WebP")
		os.Exit(1)
	}

	path := args[0]
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", path)
		os.Exit(1)
	}

	termW, termH, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termW <= 0 || termH <= 0 {
		termW, termH = 80, 24
	}

	ai, err := ansimage.NewScaledFromFile(path, termH, termW, color.Black, ansimage.ScaleModeFit, ansimage.NoDithering)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering image: %v\n", err)
		os.Exit(1)
	}

	ai.Draw()
	fmt.Println("Press Enter to exit.")
	fmt.Scanln()
}

func detectImageFormat(path string) int {
	return 100
}
