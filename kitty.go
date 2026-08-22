package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"golang.org/x/term"
)

func isKittySupported() bool {
	return true
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

func encodeKittyGraphicsFile(filePath string, cols, rows int) (string, error) {
	if cols <= 0 {
		cols = 30
	}
	if rows <= 0 {
		rows = cols
	}

	ai, err := ansimage.NewScaledFromFile(filePath, rows, cols, color.Black, ansimage.ScaleModeFit, ansimage.NoDithering)
	if err != nil {
		return "", err
	}
	return ai.Render(), nil
}

func encodeKittyGraphics(imgData []byte, cols, rows int, format int) string {
	return ""
}

func kittyClearGraphics() string {
	return ""
}

func testKittyImage(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mp3_rm_ads test kitty <image-file>")
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
