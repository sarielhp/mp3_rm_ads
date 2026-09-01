package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func (m *tuiModel) exportTranscript() {
	epPath := m.transcriptLoadedFor
	if epPath == "" && m.podIdx < len(m.podcasts) {
		eps := m.filteredEpisodes()
		if m.epIdx < len(eps) {
			epPath = eps[m.epIdx].path
		}
	}
	if epPath == "" {
		m.showToast("No episode transcript available for export", ToastError)
		return
	}

	base := strings.TrimSuffix(epPath, ".mp3")
	srtFile := base + ".srt"
	txtFile := base + ".transcript.txt"
	jsonFile := base + ".transcript.json"

	if raw, err := os.ReadFile(jsonFile); err == nil {
		var td TranscriptionData
		if err := json.Unmarshal(raw, &td); err == nil {
			dur := 0.0
			if m.podIdx < len(m.podcasts) && m.epIdx < len(m.podcasts[m.podIdx].episodes) {
				dur = m.podcasts[m.podIdx].episodes[m.epIdx].duration
			}
			convertJSONToSRT(jsonFile, &td, srtFile, true)
			convertJSONToTXT(jsonFile, &td, dur, txtFile, true)
			m.showToast("Exported transcript to .srt and .txt", ToastSuccess)
			return
		}
	}

	if len(m.transcriptLines) == 0 {
		m.showToast("No transcript content to export", ToastError)
		return
	}

	txtContent := strings.Join(m.transcriptLines, "\n") + "\n"
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		m.showToast("Failed to save .txt: "+err.Error(), ToastError)
		return
	}

	var srtEntries []string
	idx := 1
	for _, line := range m.transcriptLines {
		if sub := timestampLineRegex.FindStringSubmatch(line); len(sub) == 3 {
			ts := strings.Trim(sub[1], "[] ")
			parts := strings.Split(ts, "->")
			if len(parts) == 2 {
				stSec := parseTimestampSeconds(parts[0])
				enSec := parseTimestampSeconds(parts[1])
				st := formatSRTTime(stSec)
				en := formatSRTTime(enSec)
				text := strings.TrimSpace(sub[2])
				srtEntries = append(srtEntries, fmt.Sprintf("%d\n%s --> %s\n%s\n", idx, st, en, text))
				idx++
			}
		}
	}

	if len(srtEntries) > 0 {
		_ = os.WriteFile(srtFile, []byte(strings.Join(srtEntries, "\n")), 0644)
	}

	m.showToast("Exported transcript to .srt and .txt", ToastSuccess)
}
