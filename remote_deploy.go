package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runRemoteDeploy(cfg *Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost := host
	if targetHost == "" && cfg != nil {
		targetHost = cfg.RemoteHost
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("no remote host specified and remote_host not configured in settings")
	}

	if transport == nil {
		transport = getRemoteTransport()
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current abs executable: %w", err)
	}
	if realExe, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realExe
	}

	remoteWorkDir := "~/.abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	if !quiet {
		fmt.Printf("Preparing deployment directories on %s...\n", targetHost)
	}

	setupCmd := fmt.Sprintf("mkdir -p ~/.local/bin ~/.config/abs %s/staging %s/bin", remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, setupCmd); err != nil {
		return fmt.Errorf("failed to create directories on %s: %w", targetHost, err)
	}

	if !quiet {
		fmt.Printf("Uploading abs binary to %s:~/.local/bin/abs...\n", targetHost)
	}

	if err := transport.Upload(targetHost, exePath, "~/.local/bin/abs"); err != nil {
		return fmt.Errorf("failed to upload binary to %s: %w", targetHost, err)
	}

	chmodCmd := fmt.Sprintf("chmod +x ~/.local/bin/abs && cp ~/.local/bin/abs %s/bin/abs 2>/dev/null || true", remoteWorkDir)
	if _, err := transport.Exec(targetHost, chmodCmd); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: chmod error: %v\n", err)
		}
	}

	cfgPath := configPath()
	if fileExists(cfgPath) {
		if !quiet {
			fmt.Printf("Syncing configuration to %s:~/.config/abs/config.json...\n", targetHost)
		}
		if err := transport.Upload(targetHost, cfgPath, "~/.config/abs/config.json"); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to sync config: %v\n", err)
			}
		}
	}

	if !quiet {
		fmt.Printf("Verifying remote deployment on %s...\n", targetHost)
	}

	verifyCmd := "~/.local/bin/abs help 2>/dev/null || ~/.abs_remote/bin/abs help 2>/dev/null || abs help"
	out, err := transport.Exec(targetHost, verifyCmd)
	if err != nil {
		return fmt.Errorf("remote verification failed on %s: %w", targetHost, err)
	}

	if !quiet {
		fmt.Printf("Successfully deployed abs to %s\n", targetHost)
		if verbose && out != "" {
			firstLine := strings.Split(strings.TrimSpace(out), "\n")[0]
			fmt.Printf("Remote response: %s\n", firstLine)
		}
	}

	return nil
}
