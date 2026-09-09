package util

import (
	"path/filepath"
	"strings"
)

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func ShellQuoteHomePath(s string) string {
	if s == "~" {
		return "$HOME"
	}
	if strings.HasPrefix(s, "~/") {
		return "$HOME/" + ShellQuote(strings.TrimPrefix(s, "~/"))
	}
	return ShellQuote(s)
}

func ValidateBatchID(batchID string) bool {
	if batchID == "" || strings.Contains(batchID, "/") || strings.Contains(batchID, "\\") || strings.Contains(batchID, "..") {
		return false
	}
	if filepath.Base(batchID) != batchID {
		return false
	}
	for _, r := range batchID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
