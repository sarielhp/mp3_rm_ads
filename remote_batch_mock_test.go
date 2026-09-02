package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func NewMockRemoteTransport(root string) *MockRemoteTransport {
	return &MockRemoteTransport{
		RemoteRoot: root,
		Reachable:  true,
	}
}

func shellUnquote(tok string) string {
	tok = strings.ReplaceAll(tok, `'\''`, "'")
	tok = strings.NewReplacer("'", "", `"`, "").Replace(tok)
	return strings.TrimPrefix(tok, "$HOME/")
}

func (m *MockRemoteTransport) resolveRemotePath(remPath string) string {
	remPath = shellUnquote(remPath)
	if strings.HasPrefix(remPath, m.RemoteRoot) {
		return remPath
	}
	clean := strings.TrimPrefix(remPath, "~/")
	clean = strings.TrimPrefix(clean, ".abs_remote/")
	clean = strings.TrimPrefix(clean, "abs_remote/")
	clean = strings.TrimPrefix(clean, ".config/")
	clean = strings.TrimPrefix(clean, ".local/")
	return filepath.Join(m.RemoteRoot, clean)
}

func (m *MockRemoteTransport) Exec(host string, cmd string) (string, error) {
	m.ExecutedCmds = append(m.ExecutedCmds, cmd)
	if !m.Reachable {
		return "", fmt.Errorf("host %s unreachable", host)
	}
	if m.ExecHandler != nil {
		return m.ExecHandler(host, cmd)
	}

	if strings.Contains(cmd, "echo 1") || strings.Contains(cmd, "echo ping") {
		return "1", nil
	}
	if strings.Contains(cmd, "abs help") || strings.Contains(cmd, "abs version") || strings.Contains(cmd, "abs --version") {
		return "abs version 0.1.26", nil
	}
	if strings.HasPrefix(cmd, "pgrep") {
		return "12345\n", nil
	}
	if strings.Contains(cmd, "remote ack") {
		handleMockAckCmd(m, cmd)
		return "", nil
	}
	if out, handled := handleMockFilesystemCmd(m, cmd); handled {
		return out, nil
	}
	return "", nil
}

func handleMockAckCmd(m *MockRemoteTransport, cmd string) {
	var rels []string
	quote := "\""
	if strings.Count(cmd, "'") >= 2 {
		quote = "'"
	}
	parts := strings.Split(cmd, quote)
	for i := 1; i < len(parts); i += 2 {
		if strings.TrimSpace(parts[i]) != "" {
			rels = append(rels, strings.TrimSpace(parts[i]))
		}
	}
	if len(rels) == 0 {
		fields := strings.Fields(cmd)
		for i, f := range fields {
			if f == "ack" && i+1 < len(fields) {
				rels = append(rels, fields[i+1:]...)
				break
			}
		}
	}
	dir := m.RemoteRoot
	if len(rels) > 0 && fileExists(filepath.Join(m.RemoteRoot, "remote_root", rels[0])) {
		dir = filepath.Join(m.RemoteRoot, "remote_root")
	}
	_ = runRemoteAck(dir, rels)
}

func handleMockFilesystemCmd(m *MockRemoteTransport, cmd string) (string, bool) {
	if strings.HasPrefix(cmd, "mkdir -p") {
		raw := strings.Trim(strings.TrimSpace(strings.TrimPrefix(cmd, "mkdir -p")), "\"")
		_ = os.MkdirAll(m.resolveRemotePath(raw), 0755)
		return "", true
	}
	if strings.HasPrefix(cmd, "ls -1") {
		parts := strings.Fields(cmd)
		if len(parts) >= 3 {
			entries, err := os.ReadDir(m.resolveRemotePath(parts[2]))
			if err != nil {
				return "", true
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return strings.Join(names, "\n"), true
		}
		return "", true
	}
	if strings.HasPrefix(cmd, "rm -rf") {
		parts := strings.Fields(cmd)
		if len(parts) >= 3 {
			_ = os.RemoveAll(m.resolveRemotePath(parts[2]))
		}
		return "", true
	}
	if strings.HasPrefix(cmd, "touch ") {
		for _, p := range strings.Fields(cmd)[1:] {
			_ = os.WriteFile(m.resolveRemotePath(p), []byte{}, 0644)
		}
		return "", true
	}
	if strings.HasPrefix(cmd, "cat ") {
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			if data, err := os.ReadFile(m.resolveRemotePath(strings.Trim(parts[1], "\""))); err == nil {
				return string(data), true
			}
		}
		return "", true
	}
	if strings.Contains(cmd, "rm -f ") {
		for _, f := range strings.Fields(cmd) {
			if f != "rm" && f != "-f" && !strings.HasPrefix(f, "-") {
				_ = os.Remove(m.resolveRemotePath(f))
			}
		}
		return "", true
	}
	return "", false
}

func (m *MockRemoteTransport) Upload(host string, localSrc, remoteDst string) error {
	if !m.Reachable {
		return fmt.Errorf("host %s unreachable", host)
	}
	dst := m.resolveRemotePath(remoteDst)
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return copyFileOrDir(localSrc, dst)
}

func (m *MockRemoteTransport) Download(host string, remoteSrc, localDst string) error {
	if !m.Reachable {
		return fmt.Errorf("host %s unreachable", host)
	}
	src := m.resolveRemotePath(remoteSrc)
	_ = os.MkdirAll(filepath.Dir(localDst), 0755)
	return copyFileOrDir(src, localDst)
}

func (m *MockRemoteTransport) RsyncTo(host string, localSrc, remoteDst string) error {
	return m.Upload(host, localSrc, remoteDst)
}

func (m *MockRemoteTransport) RsyncFrom(host string, remoteSrc, localDst string) error {
	return m.Download(host, remoteSrc, localDst)
}

func copyFileOrDir(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		_ = os.MkdirAll(dst, 0755)
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			srcChild := filepath.Join(src, e.Name())
			dstChild := filepath.Join(dst, e.Name())
			if err := copyFileOrDir(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return os.WriteFile(dst, data, fi.Mode())
}
