package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd
}

func getAudioDuration(filePath string) float64 {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return 0.0
	}
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		absPath)
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}
	var dur float64
	fmt.Sscanf(string(output), "%f", &dur)
	return dur
}

func extractID3Tags(filePath string) map[string]string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format_tags",
		"-of", "default=noprint_wrappers=1",
		absPath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	tags := make(map[string]string)
	for _, line := range splitLines(string(output)) {
		if len(line) > 4 && line[:4] == "TAG:" {
			eqIdx := -1
			for i := 4; i < len(line); i++ {
				if line[i] == '=' {
					eqIdx = i
					break
				}
			}
			if eqIdx > 4 {
				key := toLower(line[4:eqIdx])
				val := line[eqIdx+1:]
				if val != "" {
					tags[key] = val
				}
			}
		}
	}
	return tags
}

func validateWavFile(filePath string) bool {
	dur := getAudioDuration(filePath)
	return dur > 0
}

func cutAudioFFmpeg(inputFile string, keepSegments [][2]float64, outputFile string) bool {
	if len(keepSegments) == 0 {
		return false
	}

	absInput, _ := filepath.Abs(inputFile)
	absOutput, _ := filepath.Abs(outputFile)

	var filterParts string
	var concatInputs string

	for idx, seg := range keepSegments {
		st := seg[0]
		en := seg[1]
		filterParts += fmt.Sprintf("[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[a%d];", st, en, idx)
		concatInputs += fmt.Sprintf("[a%d]", idx)
	}

	filterComplex := filterParts + concatInputs + fmt.Sprintf("concat=n=%d:v=0:a=1[aout]", len(keepSegments))

	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", absInput,
		"-filter_complex", filterComplex,
		"-map", "[aout]",
		"-c:a", "libmp3lame",
		"-b:a", "192k",
		absOutput)
	return cmd.Run() == nil
}

func convertToWAV(inputPath, wavPath string) bool {
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", inputPath,
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	return cmd.Run() == nil
}

func truncateAudio(inputPath, outputPath string, durationSec float64) bool {
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-ss", "0", "-i", inputPath,
		"-to", fmt.Sprintf("%.3f", durationSec),
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", outputPath)
	return cmd.Run() == nil
}
