package kitty

import (
	"os"
	"strings"
)

func IsKittyTerminal() bool {
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

func IsKittySupported() bool {
	return true
}
