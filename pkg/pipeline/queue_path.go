package pipeline

import (
	"abs/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func IsQueueAudioPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if !strings.HasSuffix(name, ".mp3") || strings.HasSuffix(name, "precut.mp3") {
		return false
	}
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == ".work" {
			return false
		}
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func ResolveQueueAudioPath(dir, entry string) (string, error) {
	if entry == "" || filepath.IsAbs(entry) {
		return "", fmt.Errorf("invalid queue path %q", entry)
	}
	clean := filepath.Clean(entry)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("queue path escapes podcast: %q", entry)
	}
	path := filepath.Join(dir, clean)
	current := dir
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if info != nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("queue path contains symlink: %s", current)
		}
	}
	if IsQueueAudioPath(path) {
		return path, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("queue entry is not episode audio: %s", path)
	}
	if filepath.Base(clean) != clean {
		return "", fmt.Errorf("queued audio is missing: %s", path)
	}
	files, err := util.FindMP3FilesErr(dir)
	if err != nil {
		return "", err
	}
	var match string
	for _, candidate := range files {
		if IsQueueAudioPath(candidate) && filepath.Base(candidate) == clean {
			if match != "" {
				return "", fmt.Errorf("ambiguous queue entry %q; use an episode-relative path", entry)
			}
			match = candidate
		}
	}
	if match == "" {
		return "", fmt.Errorf("queued audio is missing: %s", path)
	}
	return match, nil
}

func RemoveQueuedAudio(dir, path string) (bool, error) {
	removed := false
	err := UpdateQueue(dir, func(entries []string) []string {
		result := make([]string, 0, len(entries))
		for _, entry := range entries {
			resolved, err := ResolveQueueAudioPath(dir, entry)
			if err == nil && filepath.Clean(resolved) == filepath.Clean(path) {
				removed = true
			} else {
				result = append(result, entry)
			}
		}
		return result
	})
	if removed && err == nil {
		err = ClearQueuePriority(path)
	}
	return removed && err == nil, err
}

func ClearQueuePriority(path string) error {
	st, err := LoadEpisodeStatus(StatusPathFor(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Priority == 0 {
		return nil
	}
	st.Priority = 0
	return SaveEpisodeStatus(StatusPathFor(path), st)
}
