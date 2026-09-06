package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

var (
	workerLocksMu syncMutex
	workerLocks   = make(map[string]*workerLockEntry)
)

type workerLockEntry struct {
	fl       *flock.Flock
	refCount int
}

func acquireWorkerLock(resolvedDir string) (func(), error) {
	lockPath := filepath.Join(resolvedDir, ".worker.lock")

	workerLocksMu.Lock()
	defer workerLocksMu.Unlock()

	if entry, exists := workerLocks[lockPath]; exists {
		entry.refCount++
		return releaseWorkerLockFunc(lockPath), nil
	}

	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire worker lock %s: %w", lockPath, err)
	}
	if !locked {
		return nil, fmt.Errorf("remote worker is already running")
	}

	workerLocks[lockPath] = &workerLockEntry{
		fl:       fl,
		refCount: 1,
	}
	return releaseWorkerLockFunc(lockPath), nil
}

func releaseWorkerLockFunc(lockPath string) func() {
	var once syncOnce
	return func() {
		once.Do(func() {
			workerLocksMu.Lock()
			defer workerLocksMu.Unlock()
			if entry, exists := workerLocks[lockPath]; exists {
				entry.refCount--
				if entry.refCount <= 0 {
					delete(workerLocks, lockPath)
					_ = entry.fl.Unlock()
				}
			}
		})
	}
}

func acquireCollectLock(dir string) (*fileLockWrapper, error) {
	if dir == "" {
		dir = configDir()
	}
	_ = os.MkdirAll(dir, 0755)
	lockPath := filepath.Join(dir, ".collect")
	return acquireFileLock(lockPath)
}
