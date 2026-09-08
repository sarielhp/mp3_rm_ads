package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

func buildCutFilterComplex(keepSegments [][2]float64) string {
	var filter strings.Builder
	filter.Grow(len(keepSegments) * 80)
	for idx, seg := range keepSegments {
		fmt.Fprintf(&filter, "[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[a%d];", seg[0], seg[1], idx)
	}
	for idx := range keepSegments {
		fmt.Fprintf(&filter, "[a%d]", idx)
	}
	fmt.Fprintf(&filter, "concat=n=%d:v=0:a=1[aout]", len(keepSegments))
	return filter.String()
}

func hasMp3splt() bool {
	_, err := exec.LookPath("mp3splt")
	return err == nil
}

func formatCUETime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalSecs := int(seconds)
	mins := totalSecs / 60
	secs := totalSecs % 60
	frames := int((seconds - float64(totalSecs)) * 75.0)
	if frames > 74 {
		frames = 74
	}
	return fmt.Sprintf("%02d:%02d:%02d", mins, secs, frames)
}

func buildCUEContent(inputFile string, splitPoints []float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "FILE %q MP3\n", inputFile)
	for idx, pt := range splitPoints {
		fmt.Fprintf(&sb, "  TRACK %d AUDIO\n", idx+1)
		fmt.Fprintf(&sb, "    INDEX 01 %s\n", formatCUETime(pt))
	}
	return sb.String()
}

func computeSplitPoints(keepSegments [][2]float64, duration float64) []float64 {
	pts := make([]float64, 0, len(keepSegments)*2+1)
	pts = append(pts, 0.0)
	for _, seg := range keepSegments {
		if seg[0] > 0 {
			pts = append(pts, seg[0])
		}
		if seg[1] > 0 && (duration <= 0 || seg[1] < duration) {
			pts = append(pts, seg[1])
		}
	}
	sort.Float64s(pts)
	dedup := make([]float64, 0, len(pts))
	for _, p := range pts {
		if len(dedup) == 0 || p-dedup[len(dedup)-1] >= 0.05 {
			dedup = append(dedup, p)
		}
	}
	return dedup
}

func isMidpointInKeep(mid float64, keepSegments [][2]float64) bool {
	for _, seg := range keepSegments {
		if mid >= seg[0] && mid <= seg[1] {
			return true
		}
	}
	return false
}

func determineKeepChunks(splitPoints []float64, keepSegments [][2]float64, workDir string, duration float64, pfx string) []string {
	var keep []string
	for i := 0; i < len(splitPoints); i++ {
		pStart := splitPoints[i]
		pEnd := duration
		if i+1 < len(splitPoints) {
			pEnd = splitPoints[i+1]
		}
		mid := (pStart + pEnd) / 2.0
		if isMidpointInKeep(mid, keepSegments) {
			chunkName := filepath.Join(workDir, fmt.Sprintf("%s_chunk_%02d.mp3", pfx, i+1))
			keep = append(keep, chunkName)
		}
	}
	return keep
}

func concatFiles(srcFiles []string, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, src := range srcFiles {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		_, cpErr := io.Copy(out, in)
		_ = in.Close()
		if cpErr != nil {
			return cpErr
		}
	}
	return nil
}

func copyTagsAndArt(cutFile, srcFile, dstFile string) bool {
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", cutFile,
		"-i", srcFile,
		"-map", "0:a",
		"-map", "1:v?",
		"-c", "copy",
		"-map_metadata", "1",
		"-id3v2_version", "3",
		dstFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tags copy failed: %v, output: %s\n", err, string(out))
		return copyFileErr(cutFile, dstFile) == nil
	}
	return true
}

func cleanUpFiles(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

func cleanUpPrefixedChunks(workDir, pfx string, count int) {
	for i := 1; i <= count+2; i++ {
		_ = os.Remove(filepath.Join(workDir, fmt.Sprintf("%s_chunk_%02d.mp3", pfx, i)))
	}
}

func writeConcatManifest(manifestPath string, paths []string) error {
	var sb strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&sb, "file '%s'\n", strings.ReplaceAll(p, "'", "'\\''"))
	}
	return os.WriteFile(manifestPath, []byte(sb.String()), 0644)
}

func cutAudioMp3splt(absInput string, keepSegments [][2]float64, absOutput, workDir string, duration float64) bool {
	splitPoints := computeSplitPoints(keepSegments, duration)
	if len(splitPoints) <= 1 {
		return false
	}
	cueFile := filepath.Join(workDir, "split.cue")
	verifyTempFile(cueFile)
	cueContent := buildCUEContent(absInput, splitPoints)
	if err := os.WriteFile(cueFile, []byte(cueContent), 0644); err != nil {
		return false
	}
	defer os.Remove(cueFile)

	pfx := fmt.Sprintf("mp3splt_%d", time.Now().UnixNano()%1000000)
	pattern := pfx + "_chunk_@n2"
	cmd := exec.Command("mp3splt", "-q", "-n", "-x", "-c", cueFile, "-d", workDir, "-o", pattern, absInput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mp3splt failed: %v, output: %s\n", err, string(out))
		return false
	}

	keepChunks := determineKeepChunks(splitPoints, keepSegments, workDir, duration, pfx)
	existing := make([]string, 0, len(keepChunks))
	for _, p := range keepChunks {
		if fileExists(p) {
			existing = append(existing, p)
		}
	}
	defer cleanUpPrefixedChunks(workDir, pfx, len(splitPoints))

	if len(existing) == 0 {
		return false
	}
	mergedPath := filepath.Join(workDir, pfx+"_merged.mp3")
	verifyTempFile(mergedPath)
	defer os.Remove(mergedPath)

	if err := concatFiles(existing, mergedPath); err != nil {
		return false
	}
	return copyTagsAndArt(mergedPath, absInput, absOutput)
}

func cutAudioFFmpegStreamCopy(absInput string, keepSegments [][2]float64, absOutput, workDir string) bool {
	ext := filepath.Ext(absInput)
	if ext == "" {
		ext = ".mp3"
	}
	pfx := fmt.Sprintf("stream_%d", time.Now().UnixNano()%1000000)
	var partPaths []string
	for idx, seg := range keepSegments {
		partPath := filepath.Join(workDir, fmt.Sprintf("%s_part_%03d%s", pfx, idx, ext))
		verifyTempFile(partPath)
		partPaths = append(partPaths, partPath)
		length := seg[1] - seg[0]
		cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
			"-ss", fmt.Sprintf("%.3f", seg[0]),
			"-i", absInput,
			"-t", fmt.Sprintf("%.3f", length),
			"-c", "copy",
			partPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "stream segment failed: %v, output: %s\n", err, string(out))
			cleanUpFiles(partPaths)
			return false
		}
	}
	defer cleanUpFiles(partPaths)

	concatTxt := filepath.Join(workDir, pfx+"_concat.txt")
	verifyTempFile(concatTxt)
	defer os.Remove(concatTxt)
	if err := writeConcatManifest(concatTxt, partPaths); err != nil {
		return false
	}

	mergedPath := filepath.Join(workDir, pfx+"_merged"+ext)
	verifyTempFile(mergedPath)
	defer os.Remove(mergedPath)

	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "concat", "-safe", "0", "-i", concatTxt,
		"-c", "copy", mergedPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "stream concat failed: %v, output: %s\n", err, string(out))
		return false
	}
	return copyTagsAndArt(mergedPath, absInput, absOutput)
}

func cutAudioFilterComplex(absInput string, keepSegments [][2]float64, absOutput string) bool {
	filterComplex := buildCutFilterComplex(keepSegments)
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-i", absInput,
		"-filter_complex", filterComplex,
		"-map", "[aout]",
		"-c:a", "libmp3lame",
		"-b:a", "192k",
		absOutput)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg filter complex failed: %v, output: %s\n", err, string(out))
		return false
	}
	return true
}

func getCutWorkDir(absInput, absOutput string) string {
	if strings.Contains(absOutput, "/"+workDirName+"/") || filepath.Base(filepath.Dir(absOutput)) == workDirName {
		return filepath.Dir(absOutput)
	}
	if strings.Contains(absInput, "/"+workDirName+"/") || filepath.Base(filepath.Dir(absInput)) == workDirName {
		return filepath.Dir(absInput)
	}
	return filepath.Join(filepath.Dir(absOutput), workDirName, "cut_"+filepath.Base(absOutput))
}

func cutAudioLocal(absInput string, keepSegments [][2]float64, absOutput, workDir string, duration float64) bool {
	isMP3 := strings.EqualFold(filepath.Ext(absInput), ".mp3")
	if isMP3 && hasMp3splt() {
		if cutAudioMp3splt(absInput, keepSegments, absOutput, workDir, duration) {
			return true
		}
	}
	if cutAudioFFmpegStreamCopy(absInput, keepSegments, absOutput, workDir) {
		return true
	}
	return cutAudioFilterComplex(absInput, keepSegments, absOutput)
}

func cutAudioRemote(absInput, absOutput, filterComplex, remoteHost string) bool {
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
		return false
	}
	defer func() {
		delCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost, buildRemoteCutCleanupCmd(remIn, remOut))
		_ = delCmd.Run()
	}()

	remFFmpegCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost,
		buildRemoteFFmpegCmd(remIn, filterComplex, remOut))
	if err := remFFmpegCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "remote ffmpeg failed: %v\n", err)
		return false
	}

	scpOutCmd := exec.Command("scp", "-B", "-q", fmt.Sprintf("%s:%s", remoteHost, remOut), absOutput)
	if err := scpOutCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "scp out failed: %v\n", err)
		return false
	}
	return true
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
	workDir := getCutWorkDir(absInput, absOutput)
	_ = os.MkdirAll(workDir, 0755)

	duration := getAudioDuration(absInput)
	if len(keepSegments) == 1 && keepSegments[0][0] <= 0.001 && duration > 0 && (duration-keepSegments[0][1]) <= 0.001 {
		return copyFileErr(absInput, absOutput) == nil
	}

	if remoteHost != "" {
		filterComplex := buildCutFilterComplex(keepSegments)
		if cutAudioRemote(absInput, absOutput, filterComplex, remoteHost) {
			return true
		}
		return cutAudioLocal(absInput, keepSegments, absOutput, workDir, duration)
	}

	return cutAudioLocal(absInput, keepSegments, absOutput, workDir, duration)
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

func buildRemoteCutCleanupCmd(remIn, remOut string) string {
	return fmt.Sprintf("rm -f %s %s", shellQuote(remIn), shellQuote(remOut))
}

func buildRemoteFFmpegCmd(remIn, filterComplex, remOut string) string {
	return fmt.Sprintf("ffmpeg -y -loglevel error -i %s -filter_complex %s -map '[aout]' -c:a libmp3lame -b:a 192k %s",
		shellQuote(remIn), shellQuote(filterComplex), shellQuote(remOut))
}
