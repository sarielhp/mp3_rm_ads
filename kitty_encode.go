package main

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

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"golang.org/x/term"
)

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
