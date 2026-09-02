package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

func formatTime(seconds float64) string {
	sec := seconds
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := math.Mod(sec, 60)
	if hrs > 0 {
		return fmt.Sprintf("%02d:%02d:%04.1f", hrs, mins, secs)
	}
	return fmt.Sprintf("%02d:%04.1f", mins, secs)
}

func formatClock(seconds float64) string {
	sec := math.Max(math.Round(seconds), 0)
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := int(math.Mod(sec, 60))
	if hrs > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func formatSRTTime(seconds float64) string {
	sec := seconds
	hrs := int(sec / 3600)
	mins := int(math.Mod(sec, 3600) / 60)
	secs := int(math.Mod(sec, 60))
	millis := int(math.Mod(sec, 1) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hrs, mins, secs, millis)
}

func detectScriptLanguage(text string) string {
	if text == "" {
		return ""
	}

	hebrew := 0
	arabic := 0
	cyrillic := 0
	greek := 0
	totalLetters := 0

	for _, r := range text {
		if isLetter(r) {
			totalLetters++
			switch {
			case r >= 0x0590 && r <= 0x05FF:
				hebrew++
			case r >= 0x0600 && r <= 0x06FF || r >= 0x0750 && r <= 0x077F:
				arabic++
			case r >= 0x0400 && r <= 0x04FF:
				cyrillic++
			case r >= 0x0370 && r <= 0x03FF:
				greek++
			}
		}
	}

	if totalLetters == 0 {
		return ""
	}

	type langRatio struct {
		code  string
		ratio float64
	}
	ratios := []langRatio{
		{"he", float64(hebrew) / float64(totalLetters)},
		{"ar", float64(arabic) / float64(totalLetters)},
		{"ru", float64(cyrillic) / float64(totalLetters)},
		{"el", float64(greek) / float64(totalLetters)},
	}

	best := ""
	bestRatio := 0.3
	for _, r := range ratios {
		if r.ratio > bestRatio {
			bestRatio = r.ratio
			best = r.code
		}
	}
	return best
}

func isLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= 0x00C0 && r <= 0x024F) ||
		(r >= 0x0400 && r <= 0x04FF) ||
		(r >= 0x0590 && r <= 0x05FF) ||
		(r >= 0x0600 && r <= 0x06FF) ||
		(r >= 0x0750 && r <= 0x077F) ||
		(r >= 0x0370 && r <= 0x03FF) ||
		(r >= 0x1E00 && r <= 0x1EFF) ||
		(r >= 0x2C00 && r <= 0x2C5F) ||
		(r >= 0x3040 && r <= 0x309F) ||
		(r >= 0x30A0 && r <= 0x30FF) ||
		(r >= 0x4E00 && r <= 0x9FFF)
}

func validateTranscriptSanity(data *TranscriptionData, totalDuration float64, quiet bool) bool {
	if totalDuration <= 0 {
		return true
	}

	segments := data.Segments
	fullText := data.Text
	if fullText == "" && len(segments) > 0 {
		for _, seg := range segments {
			fullText += seg.Text + " "
		}
	}

	wordCount := countWords(fullText)
	minExpectedWords := int(totalDuration / 60.0 * 15.0)
	if minExpectedWords < 20 {
		minExpectedWords = 20
	}

	lastSegmentEnd := 0.0
	for _, seg := range segments {
		if seg.End > lastSegmentEnd {
			lastSegmentEnd = seg.End
		}
	}
	minRequiredCoverage := totalDuration * 0.85

	var failedReasons []string

	if wordCount < minExpectedWords {
		failedReasons = append(failedReasons,
			fmt.Sprintf("Word count too low (%d words found, expected at least %d words for %s audio",
				wordCount, minExpectedWords, formatClock(totalDuration)))
	}

	if totalDuration >= 30.0 && lastSegmentEnd < minRequiredCoverage {
		failedReasons = append(failedReasons,
			fmt.Sprintf("Transcript ended prematurely at %s (expected coverage up to at least %s)",
				formatClock(lastSegmentEnd), formatClock(minRequiredCoverage)))
	}

	if len(failedReasons) > 0 {
		if !quiet {
			fmt.Println("\n" + repeatStr("WARNING ", 5))
			fmt.Println("TRANSCRIPT SANITY CHECK FAILED!")
			fmt.Println(repeatStr("WARNING ", 5))
			for _, reason := range failedReasons {
				fmt.Printf("  - %s\n", reason)
			}
			fmt.Println("  - The Whisper transcription appears incomplete or corrupted.")
			fmt.Println("  - Aborting ad detection and audio cutting for safety.")
			fmt.Println(repeatStr("WARNING ", 5) + "\n")
		}
		return false
	}

	return true
}

func countWords(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func saveCutsJSON(mainFile string, totalDuration float64, adSegments []AdSegment, profile *LLMProfile, quiet bool) CutsResult {
	base := stripExt(mainFile)
	cutsFile := base + ".cuts.json"

	existingRaw, existingMerged, existingCutsData := loadExistingCuts(cutsFile)
	combined := append(existingRaw, adSegments...)

	formattedRaw := buildCutEntries(combined)
	formattedMerged, keep, keepIntervals := buildMergedAndKeepIntervals(totalDuration, combined)

	if existingCutsData != nil && equalMergedIntervals(existingMerged, formattedMerged) {
		if !quiet {
			fmt.Println("No new ad cuts were discovered (cut set remains unchanged).")
		}
		return CutsResult{
			CutsFile:     cutsFile,
			KeepSegments: keep,
			Changed:      false,
		}
	}

	totalCutSec := 0.0
	for _, mc := range formattedMerged {
		totalCutSec += mc.End - mc.Start
	}
	totalCutSec = roundFloat(totalCutSec, 2)

	llmInfo := "Unknown"
	if profile != nil {
		llmInfo = fmt.Sprintf("%s (%s)", profile.Name, profile.Model)
	}

	cutsData := CutsData{
		Version:             1,
		Generator:           "abs",
		LLMUsed:             llmInfo,
		TargetFile:          filepathBase(mainFile),
		OriginalDurationSec: roundFloat(totalDuration, 2),
		TotalCutDurationSec: totalCutSec,
		CutIntervals:        formattedRaw,
		MergedCutIntervals:  formattedMerged,
		KeepIntervals:       keepIntervals,
	}

	data, _ := jsonMarshalIndent(cutsData)
	writeFile(cutsFile, append(data, '\n'))

	if !quiet {
		fmt.Printf("Saved updated cut metadata (.json) to: '%s'\n", cutsFile)
	}

	return CutsResult{
		CutsFile:     cutsFile,
		KeepSegments: keep,
		Changed:      true,
	}
}

func loadExistingCuts(cutsFile string) ([]AdSegment, []MergedCutInterval, *CutsData) {
	var existingRaw []AdSegment
	var existingMerged []MergedCutInterval
	var existingCutsData *CutsData

	if fileExists(cutsFile) {
		data, err := readFile(cutsFile)
		if err == nil {
			var existing CutsData
			if jsonUnmarshal(data, &existing) == nil {
				existingCutsData = &existing
				existingMerged = append(existingMerged, existing.MergedCutIntervals...)
				for _, c := range existing.CutIntervals {
					existingRaw = append(existingRaw, AdSegment{Start: c.StartSec, End: c.EndSec, Reason: c.Reason})
				}
			}
		}
	}
	return existingRaw, existingMerged, existingCutsData
}

func buildCutEntries(combined []AdSegment) []CutEntry {
	formattedRaw := make([]CutEntry, 0, len(combined))
	for _, ad := range combined {
		entry := CutEntry{
			StartSec:       roundFloat(ad.Start, 2),
			EndSec:         roundFloat(ad.End, 2),
			DurationSec:    roundFloat(ad.End-ad.Start, 2),
			StartFormatted: formatClock(ad.Start),
			EndFormatted:   formatClock(ad.End),
		}
		if ad.Reason != "" {
			entry.Reason = ad.Reason
		}
		formattedRaw = append(formattedRaw, entry)
	}
	return formattedRaw
}

func buildMergedAndKeepIntervals(totalDuration float64, combined []AdSegment) ([]MergedCutInterval, [][2]float64, []KeepSegment) {
	allBounds := make([][2]float64, 0, len(combined))
	for _, ad := range combined {
		allBounds = append(allBounds, [2]float64{ad.Start, ad.End})
	}
	sortBounds(allBounds)

	newMerged := mergeBounds(allBounds)
	formattedMerged := make([]MergedCutInterval, 0, len(newMerged))
	for _, b := range newMerged {
		formattedMerged = append(formattedMerged, MergedCutInterval{Start: roundFloat(b[0], 2), End: roundFloat(b[1], 2)})
	}

	keep := calculateKeepSegments(totalDuration, combined)
	keepIntervals := make([]KeepSegment, 0, len(keep))
	for _, k := range keep {
		keepIntervals = append(keepIntervals, KeepSegment{Start: roundFloat(k[0], 2), End: roundFloat(k[1], 2)})
	}
	return formattedMerged, keep, keepIntervals
}

func stripExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return path
}

func filepathBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFile(path string, data []byte) error {
	return writeFileAtomic(path, data, 0644)
}

func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := renameFn(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
