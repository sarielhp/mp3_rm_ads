package scan

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// ClassifierResult represents the result of a single classifier
type ClassifierResult struct {
	Name        string
	Score       float64
	Description string
	IsSpam      bool
}

// classifyWithBogofilter runs Bogofilter on an email
func classifyWithBogofilter(emailBytes []byte) ClassifierResult {
	cmd := exec.Command("bogofilter", "-v")
	cmd.Stdin = bytes.NewReader(emailBytes)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	_ = cmd.Run()

	output := outBuf.String()
	classification := "Ham"
	spamicity := 0.0

	if contains(output, "Spam") {
		classification = "Spam"
	} else if contains(output, "Unsure") {
		classification = "Unsure"
	}

	if idx := indexOf(output, "spamicity="); idx != -1 {
		sub := output[idx+10:]
		if endIdx := indexOfAny(sub, ", \r\n"); endIdx != -1 {
			sub = sub[:endIdx]
		}
		if val, err := strconv.ParseFloat(strings.TrimSpace(sub), 64); err == nil {
			spamicity = val
		}
	}

	isSpam := spamicity >= 0.5 || classification == "Spam"

	return ClassifierResult{
		Name:        "BOGOFILTER",
		Score:       spamicity,
		Description: "Bogofilter classification: " + classification + " (spamicity: " + strconv.FormatFloat(spamicity, 'f', -1, 64) + ")",
		IsSpam:      isSpam,
	}
}

func contains(s, substr string) bool {
	return indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func indexOfAny(s string, chars string) int {
	for i, r := range s {
		if strings.ContainsRune(chars, r) {
			return i
		}
	}
	return -1
}
