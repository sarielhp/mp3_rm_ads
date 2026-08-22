package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
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
		(r >= 0x00C0 && r <= 0x024F) || // Latin Extended
		(r >= 0x0400 && r <= 0x04FF) || // Cyrillic
		(r >= 0x0590 && r <= 0x05FF) || // Hebrew
		(r >= 0x0600 && r <= 0x06FF) || // Arabic
		(r >= 0x0750 && r <= 0x077F) || // Arabic Supplement
		(r >= 0x0370 && r <= 0x03FF) || // Greek
		(r >= 0x1E00 && r <= 0x1EFF) || // Latin Extended Additional
		(r >= 0x2C00 && r <= 0x2C5F) || // Glagolitic
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0x4E00 && r <= 0x9FFF) // CJK
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

func mergeIntervals(ads []AdSegment) []AdSegment {
	if len(ads) == 0 {
		return ads
	}

	sorted := make([]AdSegment, len(ads))
	copy(sorted, ads)
	sortAds(sorted)

	merged := []AdSegment{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		last := &merged[len(merged)-1]
		ad := sorted[i]
		lastSt := last.Start
		lastEn := last.End
		adSt := ad.Start
		adEn := ad.End

		dur1 := lastEn - lastSt
		dur2 := adEn - adSt
		minDur := dur1
		if dur2 < minDur {
			minDur = dur2
		}

		allowedGap := 0.0
		switch {
		case minDur >= 30.0:
			allowedGap = 5.0
		case minDur >= 20.0:
			allowedGap = 4.0
		case minDur >= 10.0:
			allowedGap = 3.0
		}

		if adSt <= lastEn+allowedGap {
			if adEn > lastEn {
				last.End = adEn
			}
			if ad.Reason != "" && last.Reason == "" {
				last.Reason = ad.Reason
			}
		} else {
			merged = append(merged, ad)
		}
	}

	return merged
}

func sortAds(ads []AdSegment) {
	for i := 0; i < len(ads); i++ {
		for j := i + 1; j < len(ads); j++ {
			if ads[j].Start < ads[i].Start || (ads[j].Start == ads[i].Start && ads[j].End < ads[i].End) {
				ads[i], ads[j] = ads[j], ads[i]
			}
		}
	}
}

func calculateKeepSegments(totalDuration float64, ads []AdSegment) [][2]float64 {
	sorted := make([]AdSegment, len(ads))
	copy(sorted, ads)
	sortAds(sorted)

	var keep [][2]float64
	currentStart := 0.0

	for _, ad := range sorted {
		adStart := ad.Start
		adEnd := ad.End

		if adStart > currentStart {
			keep = append(keep, [2]float64{currentStart, adStart})
		}
		if adEnd > currentStart {
			currentStart = adEnd
		}
	}

	if currentStart < totalDuration {
		keep = append(keep, [2]float64{currentStart, totalDuration})
	}

	return keep
}

func saveCutsJSON(mainFile string, totalDuration float64, adSegments []AdSegment, profile *LLMProfile, quiet bool) CutsResult {
	base := stripExt(mainFile)
	cutsFile := base + ".cuts.json"

	existingRaw := []AdSegment{}
	existingMerged := []MergedCutInterval{}
	var existingCutsData *CutsData

	if fileExists(cutsFile) {
		data, err := readFile(cutsFile)
		if err == nil {
			var existing CutsData
			if jsonUnmarshal(data, &existing) == nil {
				existingCutsData = &existing
				existingMerged = append(existingMerged, existing.MergedCutIntervals...)
				for _, c := range existing.CutIntervals {
					st := c.StartSec
					en := c.EndSec
					existingRaw = append(existingRaw, AdSegment{Start: st, End: en, Reason: c.Reason})
				}
			}
		}
	}

	combined := existingRaw
	combined = append(combined, adSegments...)

	formattedRaw := make([]CutEntry, 0, len(combined))
	for _, ad := range combined {
		dur := roundFloat(ad.End-ad.Start, 2)
		entry := CutEntry{
			StartSec:       roundFloat(ad.Start, 2),
			EndSec:         roundFloat(ad.End, 2),
			DurationSec:    dur,
			StartFormatted: formatClock(ad.Start),
			EndFormatted:   formatClock(ad.End),
		}
		if ad.Reason != "" {
			entry.Reason = ad.Reason
		}
		formattedRaw = append(formattedRaw, entry)
	}

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

	generatorName := "abs"

	cutsData := CutsData{
		Version:             1,
		Generator:           generatorName,
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
	return os.WriteFile(path, data, 0644)
}

func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func sortBounds(bounds [][2]float64) {
	for i := 0; i < len(bounds); i++ {
		for j := i + 1; j < len(bounds); j++ {
			if bounds[j][0] < bounds[i][0] || (bounds[j][0] == bounds[i][0] && bounds[j][1] < bounds[i][1]) {
				bounds[i], bounds[j] = bounds[j], bounds[i]
			}
		}
	}
}

func mergeBounds(bounds [][2]float64) [][2]float64 {
	if len(bounds) == 0 {
		return bounds
	}

	merged := [][2]float64{bounds[0]}

	for i := 1; i < len(bounds); i++ {
		last := &merged[len(merged)-1]
		st := bounds[i][0]
		en := bounds[i][1]

		dur1 := (*last)[1] - (*last)[0]
		dur2 := en - st
		minDur := dur1
		if dur2 < minDur {
			minDur = dur2
		}

		allowedGap := 0.0
		switch {
		case minDur >= 30.0:
			allowedGap = 5.0
		case minDur >= 20.0:
			allowedGap = 4.0
		case minDur >= 10.0:
			allowedGap = 3.0
		}

		if st <= (*last)[1]+allowedGap {
			if en > (*last)[1] {
				(*last)[1] = en
			}
		} else {
			merged = append(merged, bounds[i])
		}
	}

	return merged
}

func equalMergedIntervals(a, b []MergedCutInterval) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Start != b[i].Start || a[i].End != b[i].End {
			return false
		}
	}
	return true
}
