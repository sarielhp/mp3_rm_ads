package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pod/pkg/config"
	"pod/pkg/types"
	"pod/pkg/util"
)

func RunRemoteDeploy(cfg *types.Config, host string, transport RemoteTransport, quiet, verbose bool) error {
	targetHost := host
	if targetHost == "" && cfg != nil {
		targetHost = cfg.RemoteHost
	}
	if targetHost == "" || strings.EqualFold(targetHost, "local") {
		return fmt.Errorf("no remote host specified and remote_host not configured in settings")
	}

	if transport == nil {
		transport = GetRemoteTransport()
	}

	exePath, err := FindDeployableBinary()
	if err != nil {
		return err
	}

	remoteWorkDir := "~/abs_remote"
	if cfg != nil && cfg.RemoteWorkDir != "" {
		remoteWorkDir = cfg.RemoteWorkDir
	}

	if err := UploadBinaryAndConfig(targetHost, exePath, remoteWorkDir, transport, quiet, verbose); err != nil {
		return err
	}

	return VerifyRemoteDeployment(targetHost, transport, quiet, verbose)
}

func FindDeployableBinary() (string, error) {
	home, _ := os.UserHomeDir()
	exePath := filepath.Join(home, "bin", "pod")
	if !util.FileExists(exePath) {
		exePath = filepath.Join(home, "bin", "abs")
	}
	if !util.FileExists(exePath) {
		if curExe, err := os.Executable(); err == nil {
			if realExe, err := filepath.EvalSymlinks(curExe); err == nil {
				exePath = realExe
			} else {
				exePath = curExe
			}
		}
	}
	if !util.FileExists(exePath) {
		return "", fmt.Errorf("failed to locate pod executable at %s", exePath)
	}
	return exePath, nil
}

func UploadBinaryAndConfig(targetHost, exePath, remoteWorkDir string, transport RemoteTransport, quiet, verbose bool) error {
	if !quiet {
		fmt.Printf("Preparing deployment directories on %s...\n", targetHost)
	}

	setupCmd := fmt.Sprintf("mkdir -p ~/.local/bin ~/.config/pod ~/.config/abs %s/staging %s/bin", remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, setupCmd); err != nil {
		return fmt.Errorf("failed to create directories on %s: %w", targetHost, err)
	}

	if !quiet {
		fmt.Printf("Uploading pod binary to %s:~/.local/bin/pod...\n", targetHost)
	}

	if err := transport.Upload(targetHost, exePath, "~/.local/bin/pod.new"); err != nil {
		return fmt.Errorf("failed to upload binary to %s: %w", targetHost, err)
	}

	chmodCmd := fmt.Sprintf("mv -f ~/.local/bin/pod.new ~/.local/bin/pod && chmod +x ~/.local/bin/pod && ln -sf ~/.local/bin/pod ~/.local/bin/abs && cp -f ~/.local/bin/pod %s/bin/pod 2>/dev/null && ln -sf %s/bin/pod %s/bin/abs 2>/dev/null || true", remoteWorkDir, remoteWorkDir, remoteWorkDir)
	if _, err := transport.Exec(targetHost, chmodCmd); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: chmod error: %v\n", err)
		}
	}

	cfgPath := config.ConfigPath()
	if util.FileExists(cfgPath) {
		if !quiet {
			fmt.Printf("Syncing configuration to %s:~/.config/pod/config.json...\n", targetHost)
		}
		if err := transport.Upload(targetHost, cfgPath, "~/.config/pod/config.json"); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to sync config: %v\n", err)
			}
		}
		_ = transport.Upload(targetHost, cfgPath, "~/.config/abs/config.json")
	}
	return nil
}

func VerifyRemoteDeployment(targetHost string, transport RemoteTransport, quiet, verbose bool) error {
	if !quiet {
		fmt.Printf("Verifying remote deployment on %s...\n", targetHost)
	}

	verifyCmd := "~/.local/bin/pod help 2>/dev/null || ~/.local/bin/abs help 2>/dev/null || pod help 2>/dev/null || abs help"
	out, err := transport.Exec(targetHost, verifyCmd)
	if err != nil {
		return fmt.Errorf("remote verification failed on %s: %w", targetHost, err)
	}

	if !quiet {
		fmt.Printf("Successfully deployed pod to %s\n", targetHost)
		if verbose && out != "" {
			firstLine := strings.Split(strings.TrimSpace(out), "\n")[0]
			fmt.Printf("Remote response: %s\n", firstLine)
		}
	}
	return nil
}
