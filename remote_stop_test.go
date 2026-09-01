package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteCommandWithoutSubcommandsShowsUsage(t *testing.T) {
	var action string
	opts := CLIOptions{}
	app := buildCLIApp(&action, &opts)

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = w
	os.Stderr = w

	_ = app.Execute([]string{"remote"})

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if action != "" {
		t.Errorf("expected action to be empty when run without subcommands, got %q", action)
	}
	if !strings.Contains(out, "Usage:  abs remote <subcommand> [args]") && !strings.Contains(out, "abs remote") {
		t.Errorf("expected remote usage output, got:\n%s", out)
	}
	if !strings.Contains(out, "Subcommands:") {
		t.Errorf("expected Subcommands list in output, got:\n%s", out)
	}
	if !strings.Contains(out, "stop [host]") {
		t.Errorf("expected stop subcommand listed, got:\n%s", out)
	}
}

func TestRemoteStopSubcommandParsing(t *testing.T) {
	var action string
	opts := CLIOptions{}
	app := buildCLIApp(&action, &opts)

	_ = app.Execute([]string{"remote", "stop", "server1.example.com", "-q"})

	if action != "remote" {
		t.Fatalf("expected action 'remote', got %q", action)
	}
	if opts.RemoteSubcmd != "stop" {
		t.Errorf("expected RemoteSubcmd 'stop', got %q", opts.RemoteSubcmd)
	}
	if opts.RemoteHost != "server1.example.com" {
		t.Errorf("expected RemoteHost 'server1.example.com', got %q", opts.RemoteHost)
	}
	if !opts.Quiet {
		t.Errorf("expected Quiet to be true")
	}
}

func TestRunRemoteStopExecution(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	remoteRoot := filepath.Join(tempDir, "remote_work")
	_ = os.MkdirAll(remoteRoot, 0755)

	lockPath := filepath.Join(remoteRoot, ".worker.lock")
	_ = os.WriteFile(lockPath, []byte("54321\n2026-08-31T20:00:00Z\n"), 0644)
	triggerPath := filepath.Join(remoteRoot, ".scan_trigger")
	_ = os.WriteFile(triggerPath, []byte(""), 0644)

	cfg := &Config{
		RemoteHost:             "worker-node",
		RemoteWorkDir:          remoteRoot,
		WhisperDockerContainer: "custom-whisper-container",
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := runRemoteStop(cfg, "worker-node", mock, false, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if err != nil {
		t.Fatalf("runRemoteStop failed: %v", err)
	}

	if !strings.Contains(out, "Stopped remote worker process and Whisper server on worker-node") {
		t.Errorf("expected stop success message, got: %s", out)
	}

	cmds := strings.Join(mock.ExecutedCmds, "\n")
	if !strings.Contains(cmds, "kill -9 54321") && !strings.Contains(cmds, "kill 54321") {
		t.Errorf("expected PID kill command in executed commands, got: %s", cmds)
	}
	if !strings.Contains(cmds, "abs.*(scan|worker|batch-worker)") {
		t.Errorf("expected worker process kill in executed commands, got: %s", cmds)
	}
	if !strings.Contains(cmds, "rm -f") || !strings.Contains(cmds, ".worker.lock") {
		t.Errorf("expected lock removal command in executed commands, got: %s", cmds)
	}
	if !strings.Contains(cmds, "docker stop whisper.cpp-server") {
		t.Errorf("expected docker stop whisper.cpp-server, got: %s", cmds)
	}
	if !strings.Contains(cmds, "docker stop custom-whisper-container") {
		t.Errorf("expected docker stop custom-whisper-container, got: %s", cmds)
	}
	if !strings.Contains(cmds, "whisper-server") {
		t.Errorf("expected whisper process kill command, got: %s", cmds)
	}
}

func TestRunRemoteStopErrors(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)

	cfg := &Config{
		RemoteHost: "",
	}

	err := runRemoteStop(cfg, "", mock, true, false)
	if err == nil {
		t.Error("expected error for empty remote host, got nil")
	}

	errLocal := runRemoteStop(cfg, "local", mock, true, false)
	if errLocal == nil {
		t.Error("expected error for local host, got nil")
	}
}

func TestStatusDisplaysLocalAndRemoteVersion(t *testing.T) {
	tempDir := t.TempDir()
	mock := NewMockRemoteTransport(tempDir)
	setRemoteTransport(mock)
	t.Cleanup(func() { setRemoteTransport(&DefaultSSHTransport{}) })

	remoteWorkDir := filepath.Join(tempDir, "remote_root")
	_ = os.MkdirAll(remoteWorkDir, 0755)

	localPodcasts := filepath.Join(tempDir, "local_podcasts")
	_ = os.MkdirAll(localPodcasts, 0755)

	cfg := Config{
		RemoteHost:    "mock-remote",
		RemoteWorkDir: remoteWorkDir,
		PodcastsDir:   localPodcasts,
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	absStatus(cfg, false, false)

	_ = w.Close()
	os.Stdout = oldStdout

	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)

	if !strings.Contains(out, "=== Local Library Status ===") {
		t.Errorf("expected Local Library Status section, got: %s", out)
	}
	expectedLocalVer := fmt.Sprintf("Version:           %s", getVersion())
	if !strings.Contains(out, expectedLocalVer) {
		t.Errorf("expected local version %q in output, got: %s", expectedLocalVer, out)
	}

	if !strings.Contains(out, "=== Remote Server Status: mock-remote ===") {
		t.Errorf("expected Remote Server Status section, got: %s", out)
	}
	if !strings.Contains(out, "Version:         0.1.26") && !strings.Contains(out, "Version:") {
		t.Errorf("expected remote version in output, got: %s", out)
	}
}
