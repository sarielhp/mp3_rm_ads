package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecuteRecutAudio_RollbackOnInstallFailure(t *testing.T) {
	dir := t.TempDir()
	mainMP3 := filepath.Join(dir, "ep.mp3")
	writeRealMP3(t, mainMP3, 10)
	origBytes, err := os.ReadFile(mainMP3)
	if err != nil {
		t.Fatalf("failed to read test mp3: %v", err)
	}

	precut := filepath.Join(dir, "ep.mp3.precut")
	outputFile := mainMP3
	workDir := filepath.Join(dir, ".work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create workDir: %v", err)
	}
	tempOutputFile := filepath.Join(workDir, "ep.mp3.tmp.mp3")

	oldRename := renameFn
	defer func() { renameFn = oldRename }()

	renameFn = func(src, dst string) error {
		if dst == outputFile {
			return errors.New("simulated disk failure")
		}
		return oldRename(src, dst)
	}

	keepSegments := [][2]float64{{0, 8}}
	executeRecutAudio(mainMP3, precut, outputFile, tempOutputFile, mainMP3, workDir, keepSegments, 10.0, Config{}, CLIOptions{Quiet: true}, time.Now())

	currentBytes, err := os.ReadFile(mainMP3)
	if err != nil {
		t.Fatalf("canonical mainMP3 was deleted/lost: %v", err)
	}
	if !bytes.Equal(currentBytes, origBytes) {
		t.Fatalf("canonical mainMP3 content was modified despite install failure")
	}
	if fileExists(precut) {
		t.Fatalf("precut file should have been rolled back/removed")
	}
}

func TestExecuteRecutAudio_SuccessPreservesPrecut(t *testing.T) {
	dir := t.TempDir()
	mainMP3 := filepath.Join(dir, "ep.mp3")
	writeRealMP3(t, mainMP3, 10)
	origBytes, err := os.ReadFile(mainMP3)
	if err != nil {
		t.Fatalf("failed to read test mp3: %v", err)
	}

	precut := filepath.Join(dir, "ep.mp3.precut")
	outputFile := mainMP3
	workDir := filepath.Join(dir, ".work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create workDir: %v", err)
	}
	tempOutputFile := filepath.Join(workDir, "ep.mp3.tmp.mp3")

	keepSegments := [][2]float64{{0, 8}}
	executeRecutAudio(mainMP3, precut, outputFile, tempOutputFile, mainMP3, workDir, keepSegments, 10.0, Config{}, CLIOptions{Quiet: true}, time.Now())

	if !fileExists(mainMP3) {
		t.Fatalf("expected recut audio at mainMP3")
	}
	if !fileExists(precut) {
		t.Fatalf("expected original audio preserved at precut")
	}
	precutBytes, err := os.ReadFile(precut)
	if err != nil {
		t.Fatalf("failed to read precut file: %v", err)
	}
	if !bytes.Equal(precutBytes, origBytes) {
		t.Fatalf("precut content does not match original")
	}
}
