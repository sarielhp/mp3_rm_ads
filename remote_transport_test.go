package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultSSHTransport_GetTimeout(t *testing.T) {
	trDefault := &DefaultSSHTransport{}
	if trDefault.getTimeout() != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", trDefault.getTimeout())
	}

	trCustom := &DefaultSSHTransport{Timeout: 15 * time.Second}
	if trCustom.getTimeout() != 15*time.Second {
		t.Errorf("expected custom timeout 15s, got %v", trCustom.getTimeout())
	}
}

func setupFakeHangingBinaries(t *testing.T) string {
	t.Helper()
	tempBinDir := t.TempDir()

	// Create fake ssh and rsync that sleep for 5 seconds to simulate hung remote calls
	fakeScript := "#!/usr/bin/env ruby\nsleep 5\n"

	sshPath := filepath.Join(tempBinDir, "ssh")
	if err := os.WriteFile(sshPath, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed to create fake ssh: %v", err)
	}

	rsyncPath := filepath.Join(tempBinDir, "rsync")
	if err := os.WriteFile(rsyncPath, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed to create fake rsync: %v", err)
	}

	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", fmt.Sprintf("%s:%s", tempBinDir, origPath))
	t.Cleanup(func() {
		_ = os.Setenv("PATH", origPath)
	})

	return tempBinDir
}

func TestDefaultSSHTransport_ContextTimeoutExec(t *testing.T) {
	setupFakeHangingBinaries(t)

	tr := &DefaultSSHTransport{Timeout: 50 * time.Millisecond}
	start := time.Now()
	_, err := tr.Exec("example-remote-host", "echo test")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Errorf("expected error message to mention 'timed out after', got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("command took too long to abort (%v), context timeout did not trigger promptly", elapsed)
	}
}

func TestDefaultSSHTransport_ContextTimeoutRsync(t *testing.T) {
	setupFakeHangingBinaries(t)

	tr := &DefaultSSHTransport{Timeout: 50 * time.Millisecond}

	start := time.Now()
	errTo := tr.RsyncTo("example-remote-host", "/tmp", "/tmp")
	elapsedTo := time.Since(start)
	if errTo == nil {
		t.Fatalf("expected rsync to timeout, got nil")
	}
	if !strings.Contains(errTo.Error(), "timed out after") {
		t.Errorf("expected error message to mention 'timed out after', got: %v", errTo)
	}
	if elapsedTo > 2*time.Second {
		t.Errorf("rsync took too long to abort (%v)", elapsedTo)
	}

	startFrom := time.Now()
	errFrom := tr.RsyncFrom("example-remote-host", "/tmp", "/tmp")
	elapsedFrom := time.Since(startFrom)
	if errFrom == nil {
		t.Fatalf("expected rsync from timeout, got nil")
	}
	if !strings.Contains(errFrom.Error(), "timed out after") {
		t.Errorf("expected error message to mention 'timed out after', got: %v", errFrom)
	}
	if elapsedFrom > 2*time.Second {
		t.Errorf("rsync from took too long to abort (%v)", elapsedFrom)
	}
}
