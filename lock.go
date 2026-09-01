package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

type fileLockWrapper struct {
	fl       *flock.Flock
	lockPath string
}

func acquireFileLock(targetPath string) (*fileLockWrapper, error) {
	lockPath := targetPath + ".lock"
	fl := flock.New(lockPath)

	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock on %s: %w", lockPath, err)
	}
	if !locked {
		return nil, nil
	}

	if f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		fmt.Fprintf(f, "%d\n", os.Getpid())
		f.Close()
	}

	return &fileLockWrapper{fl: fl, lockPath: lockPath}, nil
}

func (w *fileLockWrapper) Release() {
	if w == nil || w.fl == nil {
		return
	}
	fl := w.fl
	w.fl = nil
	_ = fl.Unlock()
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func acquireWorkerLock(resolvedDir string) (func(), error) {
	lockPath := filepath.Join(resolvedDir, ".worker.lock")
	if data, err := os.ReadFile(lockPath); err == nil {
		lines := splitLines(strings.TrimSpace(string(data)))
		var lockPID int
		var lockTime time.Time
		if len(lines) > 0 {
			_, _ = fmt.Sscanf(lines[0], "%d", &lockPID)
		}
		if len(lines) > 1 {
			lockTime, _ = time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
		}

		if lockPID == os.Getpid() && lockPID > 0 {
			return func() {}, nil
		}

		isStale := false
		if lockPID <= 0 || !isProcessAlive(lockPID) {
			isStale = true
		} else if !lockTime.IsZero() && time.Since(lockTime) > 6*time.Hour {
			isStale = true
		}

		if isStale {
			fmt.Fprintf(os.Stderr, "Warning: removing stale or dead worker lock %s (PID: %d)\n", lockPath, lockPID)
			_ = os.Remove(lockPath)
		} else {
			return nil, fmt.Errorf("remote worker is already running (lockfile %s exists with active PID %d)", lockPath, lockPID)
		}
	}

	content := fmt.Sprintf("%d\n%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(lockPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write worker lockfile %s: %w", lockPath, err)
	}

	return func() {
		_ = os.Remove(lockPath)
	}, nil
}

func acquireCollectLock(dir string) (*fileLockWrapper, error) {
	if dir == "" {
		dir = configDir()
	}
	_ = os.MkdirAll(dir, 0755)
	lockPath := filepath.Join(dir, ".collect")
	return acquireFileLock(lockPath)
}
