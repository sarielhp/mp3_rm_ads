package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func TranscriptText(path string) (string, error) {
	items, _, err := loadEpisodeTranscriptData(path)
	if err != nil {
		return transcriptPlainJSON(path)
	}
	var out strings.Builder
	for _, item := range items {
		if strings.TrimSpace(item.text) == "" {
			continue
		}
		if item.isAd {
			out.WriteString("[AD] ")
		}
		if item.timeFull != "" {
			out.WriteString(item.timeFull + " ")
		}
		out.WriteString(item.text + "\n")
	}
	if out.Len() == 0 {
		return "", fmt.Errorf("transcript is empty")
	}
	return out.String(), nil
}

func transcriptPlainJSON(path string) (string, error) {
	data, err := os.ReadFile(strings.TrimSuffix(path, ".mp3") + ".transcript.json")
	if err != nil {
		return "", fmt.Errorf("no readable transcript found: %w", err)
	}
	var td TranscriptionData
	if err := json.Unmarshal(data, &td); err != nil {
		return "", fmt.Errorf("invalid transcript JSON: %w", err)
	}
	if strings.TrimSpace(td.Text) == "" {
		return "", fmt.Errorf("transcript is empty")
	}
	return strings.TrimSpace(td.Text) + "\n", nil
}
