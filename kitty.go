package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imageorient"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/nfnt/resize"
	"golang.org/x/term"
)

func isKittySupported() bool {
	// Check for true color support (works in any modern terminal)
	p := termenv.ColorProfile()
	return p != termenv.Ascii
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

func renderImageToHalfBlocks(img image.Image, maxCols, maxRows int) string {
	width := uint(maxCols)
	height := uint(maxRows * 2)
	img = resize.Thumbnail(width, height, img, resize.Lanczos3)
	b := img.Bounds()
	w := b.Max.X
	h := b.Max.Y

	p := termenv.ColorProfile()
	var str strings.Builder

	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			c1, _ := colorful.MakeColor(img.At(x, y))
			color1 := p.Color(c1.Hex())

			var color2 termenv.Color
			if y+1 < h {
				c2, _ := colorful.MakeColor(img.At(x, y+1))
				color2 = p.Color(c2.Hex())
			} else {
				color2 = p.Color("#000000")
			}

			str.WriteString(termenv.String("▀").
				Foreground(color1).
				Background(color2).
				String())
		}
		str.WriteByte('\n')
	}

	return str.String()
}

func renderImageFile(filePath string, maxCols, maxRows int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := imageorient.Decode(f)
	if err != nil {
		return "", err
	}

	return renderImageToHalfBlocks(img, maxCols, maxRows), nil
}

func encodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	return renderImageFile(filePath, cols, rows)
}

func encodeKittyGraphics(imgData []byte, cols, rows int, format int) string {
	// Not used with half-block approach, kept for compatibility
	return ""
}

func kittyClearGraphics() string {
	return ""
}

func testKittyImage(args []string) {
	if !isKittySupported() {
		fmt.Println("True color is not supported in this terminal.")
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Println("Usage: mp3_rm_ads test kitty <image-file>")
		fmt.Println()
		fmt.Println("Displays an image using half-block character rendering.")
		fmt.Println("Supported formats: PNG, JPEG, WebP")
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

	output, err := renderImageFile(path, termW, termH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering image: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output)
	fmt.Println("Press Enter to exit.")
	fmt.Scanln()
}

func detectImageFormat(path string) int {
	return 100
}
