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

	lock2, err := acquireFileLock(targetFile)
	if err != nil {
		t.Fatalf("second acquire errored: %v", err)
	}
	if lock2 != nil {
		lock2.Release()
		t.Fatal("expected second lock acquire to fail (return nil)")
	}

	lock1.Release()

	if !fileExists(lockPath) {
		t.Errorf("lock file %s was unlinked on release; unlinking a lock path lets a "+
			"second process create a fresh inode there and hold the lock simultaneously", lockPath)
	}

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
	l.Release()
}

func TestDoubleReleaseDoesNotDropAnotherHoldersLock(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "episode.mp3")
	if err := os.WriteFile(target, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	lockPath := target + ".lock"

	first, err := acquireFileLock(target)
	if err != nil || first == nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	first.Release()

	second, err := acquireFileLock(target)
	if err != nil || second == nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	t.Cleanup(second.Release)

	// The six stray Release() calls in the old processSingleAudioFile made a
	// second release of an already-released wrapper an ordinary occurrence.
	first.Release()

	if !fileExists(lockPath) {
		t.Errorf("a stale wrapper's second Release() deleted the lock file that %q now holds", lockPath)
	}
	third, err := acquireFileLock(target)
	if err != nil {
		t.Fatalf("third acquire errored: %v", err)
	}
	if third != nil {
		third.Release()
		t.Errorf("mutual exclusion was lost: a third holder acquired a lock the second still holds")
	}
}
