package util

import (
	"fmt"
	"path/filepath"
	"regexp"
)

const WorkDirName = ".work"

func WorkDirFor(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return filepath.Join(filepath.Dir(abs), WorkDirName, filepath.Base(abs))
}

func VerifyTempFile(filePath string) error {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("temp file '%s' path resolution failed: %w", filePath, err)
	}
	matched, _ := regexp.MatchString(`/\.work/`, abs)
	if !matched {
		return fmt.Errorf("temp file '%s' is not in .work/ directory", filePath)
	}
	return nil
}
