package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptTextFormatsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, suffix, data, want string
		fail                     bool
	}{
		{"segments", ".json", `{"segments":[{"start":1,"end":2,"text":"` + strings.Repeat("complete text ", 100) + `"}]}`, strings.TrimSpace(strings.Repeat("complete text ", 100)), false},
		{"plain JSON", ".json", `{"text":"Full plain transcript"}`, "Full plain transcript", false},
		{"plain TXT", ".txt", "Full text transcript\n", "Full text transcript", false},
		{"empty JSON", ".json", `{}`, "", true},
		{"empty TXT", ".txt", "  \n", "", true},
		{"invalid JSON", ".json", `{broken`, "", true},
		{"missing", "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "episode")
			if tc.suffix != "" {
				if err := os.WriteFile(base+".transcript"+tc.suffix, []byte(tc.data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			text, err := TranscriptText(base + ".mp3")
			if (err != nil) != tc.fail || !strings.Contains(text, tc.want) {
				t.Fatalf("text=%q, err=%v", text, err)
			}
			if tc.name == "segments" && !strings.Contains(text, " -> ") {
				t.Fatal("missing timestamps")
			}
		})
	}
}
