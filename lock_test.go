package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireAndReleaseFileLock(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "episode.mp3")
	if err := os.WriteFile(targetFile, []byte("audio"), 0644); err != nil {
		t.Fatalf("failed to create dummy target: %v", err)
	}

	lock1, err := acquireFileLock(targetFile)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if lock1 == nil {
		t.Fatal("expected lock1 to be acquired")
	}

	lockPath := targetFile + ".lock"
	if !fileExists(lockPath) {
		t.Errorf("expected lock file %s to exist", lockPath)
	}

	pidData, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil || pid != os.Getpid() {
		t.Errorf("expected lock file to contain current PID %d, got %s", os.Getpid(), string(pidData))
	}

	// Second acquire on the same target must return (nil, nil)
	lock2, err := acquireFileLock(targetFile)
	if err != nil {
		t.Fatalf("second acquire errored: %v", err)
	}
	if lock2 != nil {
		lock2.Release()
		t.Fatal("expected second lock acquire to fail (return nil)")
	}

	// Release lock1
	lock1.Release()

	if fileExists(lockPath) {
		t.Errorf("expected lock file %s to be removed on release", lockPath)
	}

	// Now third acquire should succeed
	lock3, err := acquireFileLock(targetFile)
	if err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	if lock3 == nil {
		t.Fatal("expected lock3 to be acquired after release")
	}
	lock3.Release()
}

func TestNilFileLockRelease(t *testing.T) {
	var l *fileLockWrapper
	l.Release() // Should not panic
}
