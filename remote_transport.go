package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RemoteTransport interface {
	Exec(host string, cmd string) (string, error)
	Upload(host string, localSrc, remoteDst string) error
	Download(host string, remoteSrc, localDst string) error
	RsyncTo(host string, localSrc, remoteDst string) error
	RsyncFrom(host string, remoteSrc, localDst string) error
}

type DefaultSSHTransport struct {
	Timeout time.Duration
}

func (t *DefaultSSHTransport) getTimeout() time.Duration {
	if t.Timeout > 0 {
		return t.Timeout
	}
	return 30 * time.Second
}

func (t *DefaultSSHTransport) Exec(host string, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.getTimeout())
	defer cancel()
	c := exec.CommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
		host, cmd)
	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	err := c.Run()
	outStr := strings.TrimSpace(outBuf.String())
	if ctx.Err() == context.DeadlineExceeded {
		return outStr, fmt.Errorf("ssh command on %s timed out after %s", host, t.getTimeout())
	}
	if err != nil {
		errStr := strings.TrimSpace(errBuf.String())
		if errStr != "" {
			return outStr, fmt.Errorf("ssh error on %s: %s (%w)", host, errStr, err)
		}
		return outStr, fmt.Errorf("ssh error on %s: %w", host, err)
	}
	return outStr, nil
}

func (t *DefaultSSHTransport) Upload(host string, localSrc, remoteDst string) error {
	return t.RsyncTo(host, localSrc, remoteDst)
}

func (t *DefaultSSHTransport) Download(host string, remoteSrc, localDst string) error {
	return t.RsyncFrom(host, remoteSrc, localDst)
}

func (t *DefaultSSHTransport) RsyncTo(host string, localSrc, remoteDst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.getTimeout())
	defer cancel()
	sshOpt := "ssh -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=5 -o ServerAliveCountMax=3"
	c := exec.CommandContext(ctx, "rsync", "-avz", "-e", sshOpt, localSrc, fmt.Sprintf("%s:%s", host, remoteDst))
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("rsync to %s timed out after %s", host, t.getTimeout())
		}
		errStr := strings.TrimSpace(errBuf.String())
		if errStr != "" {
			return fmt.Errorf("rsync to %s failed: %s (%w)", host, errStr, err)
		}
		return fmt.Errorf("rsync to %s failed: %w", host, err)
	}
	return nil
}

func (t *DefaultSSHTransport) RsyncFrom(host string, remoteSrc, localDst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.getTimeout())
	defer cancel()
	sshOpt := "ssh -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=5 -o ServerAliveCountMax=3"
	c := exec.CommandContext(ctx, "rsync", "-avz", "-e", sshOpt, fmt.Sprintf("%s:%s", host, remoteSrc), localDst)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("rsync from %s timed out after %s", host, t.getTimeout())
		}
		errStr := strings.TrimSpace(errBuf.String())
		if errStr != "" {
			return fmt.Errorf("rsync from %s failed: %s (%w)", host, errStr, err)
		}
		return fmt.Errorf("rsync from %s failed: %w", host, err)
	}
	return nil
}

var currentTransport RemoteTransport = &DefaultSSHTransport{}

func getRemoteTransport() RemoteTransport {
	return currentTransport
}

func setRemoteTransport(t RemoteTransport) {
	currentTransport = t
}

func isRemoteHostReachable(host string, transport RemoteTransport) bool {
	if host == "" || host == "local" {
		return true
	}
	if transport == nil {
		transport = getRemoteTransport()
	}
	_, err := transport.Exec(host, "echo 1")
	return err == nil
}

func ResolveProcessingHost(cfg *Config, requestedHost string, transport RemoteTransport) (string, bool, error) {
	if requestedHost != "" {
		if strings.EqualFold(requestedHost, "local") {
			return "local", false, nil
		}
		return requestedHost, true, nil
	}

	targetHost := ""
	if cfg != nil {
		targetHost = cfg.RemoteHost
		if targetHost == "" {
			targetHost = cfg.RemoteFFmpegHost
		}
	}

	if cfg != nil && strings.EqualFold(cfg.DefaultProcessing, "remote") && targetHost != "" {
		if transport == nil {
			transport = getRemoteTransport()
		}
		if isRemoteHostReachable(targetHost, transport) {
			return targetHost, true, nil
		}
		fmt.Fprintf(os.Stderr, "Warning: Remote host '%s' unreachable. Falling back to local processing.\n", targetHost)
		return "local", false, nil
	}

	return "local", false, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellQuoteHomePath(s string) string {
	if s == "~" {
		return "$HOME"
	}
	if strings.HasPrefix(s, "~/") {
		return "$HOME/" + shellQuote(strings.TrimPrefix(s, "~/"))
	}
	return shellQuote(s)
}

func validateBatchID(batchID string) bool {
	if batchID == "" || strings.Contains(batchID, "/") || strings.Contains(batchID, "\\") || strings.Contains(batchID, "..") {
		return false
	}
	if filepath.Base(batchID) != batchID {
		return false
	}
	for _, r := range batchID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
