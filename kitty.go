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

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
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

func generateGenericCover(title string) image.Image {
	const imgDim = 256
	dst := image.NewRGBA(image.Rect(0, 0, imgDim, imgDim))

	bg := color.RGBA{R: 248, G: 249, B: 250, A: 255}
	border := color.RGBA{R: 220, G: 224, B: 230, A: 255}
	accent := color.RGBA{R: 70, G: 130, B: 180, A: 255}

	for y := 0; y < imgDim; y++ {
		for x := 0; x < imgDim; x++ {
			if x == 0 || x == imgDim-1 || y == 0 || y == imgDim-1 {
				dst.Set(x, y, border)
			} else if y < 4 {
				dst.Set(x, y, accent)
			} else {
				dst.Set(x, y, bg)
			}
		}
	}

	words := strings.Fields(title)
	var lines []string
	var cur string
	maxCharsPerLine := 14
	if len(title) > 28 {
		maxCharsPerLine = 18
	}
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= maxCharsPerLine {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
			if len(lines) >= 4 {
				break
			}
		}
	}
	if cur != "" && len(lines) < 4 {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{"Podcast"}
	}

	lineH := 14
	rawH := len(lines) * lineH
	maxLineW := 0
	for _, l := range lines {
		w := len(l) * 7
		if w > maxLineW {
			maxLineW = w
		}
	}
	if maxLineW <= 0 {
		maxLineW = 7
	}
	if rawH <= 0 {
		rawH = 13
	}

	srcBuf := image.NewRGBA(image.Rect(0, 0, maxLineW, rawH))
	d := &font.Drawer{
		Dst:  srcBuf,
		Src:  image.NewUniform(color.RGBA{R: 30, G: 35, B: 42, A: 255}),
		Face: basicfont.Face7x13,
	}

	for i, l := range lines {
		lineW := len(l) * 7
		startX := (maxLineW - lineW) / 2
		d.Dot = fixed.P(startX, i*lineH+11)
		d.DrawString(l)
	}

	targetW := float64(imgDim) * 0.80
	targetH := float64(imgDim) * 0.80

	scaleX := targetW / float64(maxLineW)
	scaleY := targetH / float64(rawH)
	scale := scaleX
	if scale*float64(rawH) > targetH {
		scale = scaleY
	}

	scaledW := int(float64(maxLineW) * scale)
	scaledH := int(float64(rawH) * scale)
	if scaledW <= 0 {
		scaledW = 1
	}
	if scaledH <= 0 {
		scaledH = 1
	}

	dstX := (imgDim - scaledW) / 2
	dstY := (imgDim - scaledH) / 2
	dstRect := image.Rect(dstX, dstY, dstX+scaledW, dstY+scaledH)

	draw.ApproxBiLinear.Scale(dst, dstRect, srcBuf, srcBuf.Bounds(), draw.Over, nil)

	return dst
}

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

	genPath := filepath.Join(cDir, "cover.png")
	if fi, err := os.Stat(genPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
		return genPath
	}

	title := displayName(filepath.Base(podcastDir))
	if title == "" || title == "." || title == "/" {
		title = "Podcast"
	}
	img := generateGenericCover(title)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err == nil {
		_ = os.WriteFile(genPath, buf.Bytes(), 0644)
		return genPath
	}

	return ""
}

func scaleImageThumbnail(img image.Image, targetW, targetH int) image.Image {
	if targetW <= 0 {
		targetW = 256
	}
	if targetH <= 0 {
		targetH = 256
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return img
	}

	dstRect := image.Rect(0, 0, targetW, targetH)
	dstImg := image.NewRGBA(dstRect)
	draw.ApproxBiLinear.Scale(dstImg, dstRect, img, bounds, draw.Over, nil)
	return dstImg
}

func getOrCacheCoverPNG(filePath string) ([]byte, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%d:%d", filePath, fi.ModTime().UnixNano(), fi.Size())

	coverPngMemoryCacheMu.Lock()
	if cached, ok := coverPngMemoryCache[key]; ok {
		coverPngMemoryCacheMu.Unlock()
		return cached, nil
	}
	coverPngMemoryCacheMu.Unlock()

	cDir := podcastCacheDirForImage(filePath)
	diskPngPath := filepath.Join(cDir, "cover.png")

	if diskFi, err := os.Stat(diskPngPath); err == nil && diskFi.Size() > 0 {
		if !diskFi.ModTime().Before(fi.ModTime()) {
			if diskData, err := os.ReadFile(diskPngPath); err == nil && len(diskData) >= 8 && string(diskData[:8]) == "\x89PNG\r\n\x1a\n" {
				coverPngMemoryCacheMu.Lock()
				coverPngMemoryCache[key] = diskData
				coverPngMemoryCacheMu.Unlock()
				return diskData, nil
			}
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err == nil {
			scaled := scaleImageThumbnail(img, 256, 256)
			var buf bytes.Buffer
			if err := png.Encode(&buf, scaled); err == nil {
				pngBytes := buf.Bytes()
				coverPngMemoryCacheMu.Lock()
				coverPngMemoryCache[key] = pngBytes
				coverPngMemoryCacheMu.Unlock()
				_ = os.WriteFile(diskPngPath, pngBytes, 0644)
				return pngBytes, nil
			}
		}
		coverPngMemoryCacheMu.Lock()
		coverPngMemoryCache[key] = data
		coverPngMemoryCacheMu.Unlock()
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	scaled := scaleImageThumbnail(img, 256, 256)

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	pngBytes := buf.Bytes()

	coverPngMemoryCacheMu.Lock()
	coverPngMemoryCache[key] = pngBytes
	coverPngMemoryCacheMu.Unlock()

	_ = os.WriteFile(diskPngPath, pngBytes, 0644)

	return pngBytes, nil
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

func podcastCacheDirForImage(imagePath string) string {
	dir := filepath.Dir(imagePath)
	base := cacheBaseDir()
	if strings.HasPrefix(filepath.Clean(dir), filepath.Clean(base)) {
		return dir
	}
	return cacheDirForPodcast(dir)
}

func prewarmPodcastCovers(podcasts []tuiPodcast, cols, rows int) {
	go func() {
		for _, pod := range podcasts {
			cp := pod.coverPath
			if cp == "" {
				cp = findCoverImage(pod.dir)
			}
			if cp != "" {
				_, _ = encodeKittyGraphicsFile(cp, cols, rows)
			}
		}
	}()
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

	coverGraphicsCacheMu.Lock()
	if cached, ok := coverGraphicsCache[key]; ok {
		coverGraphicsCacheMu.Unlock()
		return cached, nil
	}
	coverGraphicsCacheMu.Unlock()

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

	var result string
	var err error

	if isKitty {
		result, err = encodeNativeKittyGraphics(filePath, cols, rows)
	} else {
		pngBytes, pErr := getOrCacheCoverPNG(filePath)
		if pErr == nil {
			if img, _, dErr := image.Decode(bytes.NewReader(pngBytes)); dErr == nil {
				var ai *ansimage.ANSImage
				ai, err = ansimage.NewScaledFromImage(img, rows, cols, color.Black, ansimage.ScaleModeResize, ansimage.NoDithering)
				if err == nil {
					result = ai.Render()
				}
			}
		}
		if result == "" {
			var ai *ansimage.ANSImage
			ai, err = ansimage.NewScaledFromFile(filePath, rows, cols, color.Black, ansimage.ScaleModeResize, ansimage.NoDithering)
			if err == nil {
				result = ai.Render()
			}
		}
	}

	if err == nil && result != "" {
		coverGraphicsCacheMu.Lock()
		coverGraphicsCache[key] = result
		coverGraphicsCacheMu.Unlock()

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
		fatalError("%s\n", "Supported formats: PNG, JPEG, GIF, BMP, TIFF, WebP")
	}

	path := args[0]
	if _, err := os.Stat(path); err != nil {
		fatalError("Error: file not found: %s\n", path)
	}

	termW, termH, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termW <= 0 || termH <= 0 {
		termW, termH = 80, 24
	}

	ai, err := ansimage.NewScaledFromFile(path, termH, termW, color.Black, ansimage.ScaleModeFit, ansimage.NoDithering)
	if err != nil {
		fatalError("Error rendering image: %v\n", err)
	}

	ai.Draw()
	fmt.Println("Press Enter to exit.")
	fmt.Scanln()
}

func detectImageFormat(path string) int {
	return 100
}
