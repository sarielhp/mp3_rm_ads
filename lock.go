package main

import (
	"context"
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

	return &fileLockWrapper{fl: fl, lockPath: lockPath}, nil
}

func acquireFileLockWithTimeout(targetPath string, timeout time.Duration) (*fileLockWrapper, error) {
	lockPath := targetPath + ".lock"
	fl := flock.New(lockPath)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	locked, err := fl.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to acquire lock on %s: %w", lockPath, err)
	}
	if !locked {
		return nil, nil
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

func checkStaleWorkerLock(lockPath string) (bool, int, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, 0, nil
		}
		return false, 0, err
	}
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
		return false, lockPID, nil
	}
	if lockPID <= 0 || !isProcessAlive(lockPID) || (!lockTime.IsZero() && time.Since(lockTime) > 6*time.Hour) {
		return true, lockPID, nil
	}
	return false, lockPID, fmt.Errorf("remote worker is already running (lockfile %s exists with active PID %d)", lockPath, lockPID)
}

func acquireWorkerLock(resolvedDir string) (func(), error) {
	lockPath := filepath.Join(resolvedDir, ".worker.lock")
	content := fmt.Sprintf("%d\n%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))

	for attempts := 0; attempts < 2; attempts++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, writeErr := f.WriteString(content)
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(lockPath)
				if writeErr != nil {
					return nil, writeErr
				}
				return nil, closeErr
			}
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to create worker lockfile %s: %w", lockPath, err)
		}

		stale, pid, checkErr := checkStaleWorkerLock(lockPath)
		if !stale {
			if pid == os.Getpid() && pid > 0 {
				return func() {}, nil
			}
			return nil, checkErr
		}

		fmt.Fprintf(os.Stderr, "Warning: removing stale or dead worker lock %s (PID: %d)\n", lockPath, pid)
		_ = os.Remove(lockPath)
	}
	return nil, fmt.Errorf("failed to acquire worker lock after clearing stale lock %s", lockPath)
}

func acquireCollectLock(dir string) (*fileLockWrapper, error) {
	if dir == "" {
		dir = configDir()
	}
	_ = os.MkdirAll(dir, 0755)
	lockPath := filepath.Join(dir, ".collect")
	return acquireFileLock(lockPath)
}
