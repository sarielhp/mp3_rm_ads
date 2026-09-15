package cli

import (
	"fmt"
	"image/color"
	"io"
	"os"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"golang.org/x/term"
)

func testKittyImage(w io.Writer, args []string) {
	if len(args) > 0 && args[0] == "kitty" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(w, "Usage: pod test kitty <image-file>")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Displays an image using ANSI true-color half-block rendering.")
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
	fmt.Fprintln(w, "Press Enter to exit.")
	fmt.Scanln()
}
