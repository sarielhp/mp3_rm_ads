package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd
}

func getAudioDuration(filePath string) float64 {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get absolute path: %v\n", err)
		return 0.0
	}
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		absPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffprobe failed: %v, output: %s\n", err, string(output))
		return 0.0
	}
	var dur float64
	fmt.Sscanf(string(output), "%f", &dur)
	return dur
}

func extractID3Tags(filePath string) map[string]string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get absolute path: %v\n", err)
		return nil
	}
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format_tags",
		"-of", "default=noprint_wrappers=1",
		absPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffprobe failed: %v, output: %s\n", err, string(output))
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

const minKeepFraction = 0.25

func keepFractionIsPlausible(inputFile string, keepSegments [][2]float64) bool {
	sourceDuration := getAudioDuration(inputFile)
	if sourceDuration <= 0 {
		return true
	}
	kept := 0.0
	for _, seg := range keepSegments {
		if seg[1] > seg[0] {
			kept += seg[1] - seg[0]
		}
	}
	if kept >= sourceDuration*minKeepFraction {
		return true
	}
	fmt.Fprintf(os.Stderr,
		"Refusing to cut '%s': the requested cut would keep only %.1fs of %.1fs (%.1f%%, floor %.0f%%).\n"+
			"This usually means the ad detector returned implausible timestamps. The file was left unchanged.\n",
		inputFile, kept, sourceDuration, kept/sourceDuration*100, minKeepFraction*100)
	return false
}

func cutAudioFFmpeg(inputFile string, keepSegments [][2]float64, outputFile string) bool {
	return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")
}

func cutAudioFFmpegWithHost(inputFile string, keepSegments [][2]float64, outputFile, remoteHost string) bool {
	if len(keepSegments) == 0 {
		return false
	}

	if !keepFractionIsPlausible(inputFile, keepSegments) {
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

	if remoteHost != "" {
		tempID := fmt.Sprintf("abs_%d_%d", time.Now().UnixNano(), os.Getpid())
		ext := filepath.Ext(absInput)
		if ext == "" {
			ext = ".mp3"
		}
		remIn := fmt.Sprintf(".work/%s_in%s", tempID, ext)
		remOut := fmt.Sprintf(".work/%s_out%s", tempID, filepath.Ext(absOutput))

		scpInCmd := exec.Command("scp", "-B", "-q", absInput, fmt.Sprintf("%s:%s", remoteHost, remIn))
		if err := scpInCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "scp in failed: %v\n", err)
			return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")
		}
		defer func() {
			delCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost, fmt.Sprintf("rm -f %s %s", remIn, remOut))
			_ = delCmd.Run()
		}()

		remFFmpegCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost,
			fmt.Sprintf("ffmpeg -y -loglevel error -i %s -filter_complex %q -map '[aout]' -c:a libmp3lame -b:a 192k %s",
				remIn, filterComplex, remOut))
		if err := remFFmpegCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "remote ffmpeg failed: %v\n", err)
			return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")
		}

		scpOutCmd := exec.Command("scp", "-B", "-q", fmt.Sprintf("%s:%s", remoteHost, remOut), absOutput)
		if err := scpOutCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "scp out failed: %v\n", err)
			return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")
		}

		return true
	}

	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", absInput,
		"-filter_complex", filterComplex,
		"-map", "[aout]",
		"-c:a", "libmp3lame",
		"-b:a", "192k",
		absOutput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg failed: %v, output: %s\n", err, string(out))
		return false
	}
	return true
}

func convertToWAV(inputPath, wavPath string) bool {
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", inputPath,
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg convert failed: %v, output: %s\n", err, string(out))
		return false
	}
	return true
}

func truncateAudio(inputPath, outputPath string, durationSec float64) bool {
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-ss", "0", "-i", inputPath,
		"-to", fmt.Sprintf("%.3f", durationSec),
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg truncate failed: %v, output: %s\n", err, string(out))
		return false
	}
	return true
}
