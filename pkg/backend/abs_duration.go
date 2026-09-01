package backend

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tcolgate/mp3"
)

func GetMP3DiskDurationNative(path string) float64 {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	d := mp3.NewDecoder(file)
	var duration float64
	var frame mp3.Frame
	var skipped int

	for {
		if err := d.Decode(&frame, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		duration += frame.Duration().Seconds()
	}

	return duration
}

func GetMP3DiskDuration(path string) float64 {
	if path == "" {
		return 0
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return 0
	}

	if dur := GetMP3DiskDurationNative(path); dur > 0 {
		return dur
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		s := strings.TrimSpace(out.String())
		if dur, err := strconv.ParseFloat(s, 64); err == nil && dur > 0 {
			return dur
		}
	}

	ctxMI, cancelMI := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelMI()

	cmdMI := exec.CommandContext(ctxMI, "mediainfo", "--Inform=General;%Duration%", path)
	var outMI bytes.Buffer
	cmdMI.Stdout = &outMI
	if err := cmdMI.Run(); err == nil {
		s := strings.TrimSpace(outMI.String())
		if durMS, err := strconv.ParseFloat(s, 64); err == nil && durMS > 0 {
			return durMS / 1000.0
		}
	}

	return 0
}

func resolveHostPathLocal(path string, podcastsDir string) string {
	if path == "" {
		return ""
	}
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return path
	}

	if podcastsDir != "" {
		relPath := path
		if strings.HasPrefix(relPath, "/podcasts/") {
			relPath = strings.TrimPrefix(relPath, "/podcasts/")
		} else if strings.HasPrefix(relPath, "/") {
			relPath = strings.TrimPrefix(relPath, "/")
		}

		mapped := filepath.Join(podcastsDir, relPath)
		if fi, err := os.Stat(mapped); err == nil && !fi.IsDir() {
			return mapped
		}

		baseDir := filepath.Base(filepath.Dir(path))
		baseFile := filepath.Base(path)
		mappedSub := filepath.Join(podcastsDir, baseDir, baseFile)
		if fi, err := os.Stat(mappedSub); err == nil && !fi.IsDir() {
			return mappedSub
		}
	}

	return ""
}
