package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func saveJSONTranscript(mainFile string, data *TranscriptionData, jsonFile string, quiet bool, id3Tags map[string]string) {
	outputData := make(map[string]interface{})
	raw, _ := json.Marshal(data)
	json.Unmarshal(raw, &outputData)

	for k, v := range id3Tags {
		outputData["id3_"+k] = v
	}

	content, _ := json.MarshalIndent(outputData, "", "  ")
	writeFile(jsonFile, append(content, '\n'))
	if !quiet {
		fmt.Printf("Saved raw Whisper JSON data (.json) to: '%s'\n", jsonFile)
	}
}

func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {
	if data == nil {
		if !fileExists(inputFile) {
			fmt.Fprintf(os.Stderr, "Error: Cannot convert to SRT, JSON file not found: '%s'\n", inputFile)
			return ""
		}
		raw, err := readFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading JSON file: %v\n", err)
			return ""
		}
		var td TranscriptionData
		if err := jsonUnmarshal(raw, &td); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
			return ""
		}
		data = &td
	}

	base := stripExt(inputFile)
	srtFile := customPath
	if srtFile == "" || !strings.HasSuffix(srtFile, ".srt") {
		srtFile = base + ".srt"
	}

	var lines []string
	for idx, seg := range data.Segments {
		st := formatSRTTime(seg.Start)
		en := formatSRTTime(seg.End)
		text := strings.TrimSpace(seg.Text)
		lines = append(lines, fmt.Sprintf("%d", idx+1))
		lines = append(lines, fmt.Sprintf("%s --> %s", st, en))
		lines = append(lines, text)
		lines = append(lines, "")
	}

	writeFile(srtFile, []byte(strings.Join(lines, "\n")+"\n"))
	if !quiet {
		fmt.Printf("Converted and saved SubRip Subtitle file (.srt) to: '%s'\n", srtFile)
	}
	return srtFile
}

func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {
	if data == nil {
		if !fileExists(inputFile) {
			fmt.Fprintf(os.Stderr, "Error: Cannot convert to TXT, JSON file not found: '%s'\n", inputFile)
			return ""
		}
		raw, err := readFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading JSON file: %v\n", err)
			return ""
		}
		var td TranscriptionData
		if err := jsonUnmarshal(raw, &td); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
			return ""
		}
		data = &td
	}

	base := stripExt(inputFile)
	txtFile := customPath
	if txtFile == "" || !strings.HasSuffix(txtFile, ".txt") {
		txtFile = base + ".transcript.txt"
	}

	lang := data.Language
	if lang == "" {
		lang = "auto"
	}

	var lines []string
	lines = append(lines, repeatStr("=", 80))
	lines = append(lines, fmt.Sprintf("PODCAST TRANSCRIPTION: %s", filepathBase(base)))
	lines = append(lines, fmt.Sprintf("Original Duration: %s (%.1fs) | Language: %s", formatTime(totalDuration), totalDuration, strings.ToUpper(lang)))
	lines = append(lines, repeatStr("=", 80))
	lines = append(lines, "")

	if len(data.Segments) == 0 && data.Text != "" {
		lines = append(lines, fmt.Sprintf("[00:00.0 -> %s] %s", formatTime(totalDuration), data.Text))
	} else {
		for _, seg := range data.Segments {
			st := seg.Start
			en := seg.End
			text := strings.TrimSpace(seg.Text)
			lines = append(lines, fmt.Sprintf("[%s -> %s] %s", formatTime(st), formatTime(en), text))

			if len(seg.Words) > 0 {
				var wordStrs []string
				for _, w := range seg.Words {
					wordStrs = append(wordStrs, fmt.Sprintf("%s(%.2f-%.2f)", w.Word, w.Start, w.End))
				}
				lines = append(lines, "   Words: "+strings.Join(wordStrs, " "))
			}
		}
	}

	writeFile(txtFile, []byte(strings.Join(lines, "\n")+"\n"))
	if !quiet {
		fmt.Printf("Converted and saved timestamped text transcript (.txt) to: '%s'\n", txtFile)
	}
	return txtFile
}

func checkPrecutSymlink(precutFile string) {
	info, err := os.Lstat(precutFile)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Pre-cut backup file '%s' is a symlink. Refusing to overwrite.\n", precutFile)
		os.Exit(1)
	}
}

func safeMove(src, dst string) {
	os.Remove(dst)
	os.Rename(src, dst)
}

func copyFile(src, dst string) {
	data, err := readFile(src)
	if err != nil {
		return
	}
	writeFile(dst, data)
}

func findMP3Files(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == ".work" || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			subFiles := findMP3Files(filepath.Join(dir, entry.Name()))
			files = append(files, subFiles...)
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}
