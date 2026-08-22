package app

import (
	"os"

	"golang.org/x/term"
)

func GetTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 70
	}
	return width
}
