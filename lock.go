package main

import (
	"fmt"
	"os"

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
	_ = w.fl.Unlock()
	_ = os.Remove(w.lockPath)
}
