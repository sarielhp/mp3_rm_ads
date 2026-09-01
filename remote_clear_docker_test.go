package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// stubDocker installs a fake `docker` on PATH whose --filter form fails, which is
// what an older daemon or an unsupported filter does.
func stubDocker(t *testing.T) (logPath string) {
	t.Helper()
	bin := t.TempDir()
	logPath = filepath.Join(bin, "restart.log")
	script := `#!/bin/sh
if [ "$1" = "ps" ]; then
  case "$*" in
    *--filter*) exit 1 ;;
    *) echo container_aaa; echo container_bbb; exit 0 ;;
  esac
fi
if [ "$1" = "restart" ]; then
  shift
  echo "$@" >> "` + logPath + `"
  exit 0
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func TestWhisperDockerRestartNeverRestartsUnrelatedContainers(t *testing.T) {
	logPath := stubDocker(t)

	if out, err := exec.Command("sh", "-c", whisperDockerRestartCommand()).CombinedOutput(); err != nil {
		t.Fatalf("command failed: %v: %s", err, out)
	}

	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		t.Errorf("the whisper filter matched nothing, but these containers were restarted: %s", data)
	}
}

func TestWhisperDockerRestartStillRestartsMatches(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(bin, "restart.log")
	script := `#!/bin/sh
if [ "$1" = "ps" ]; then echo whisper_one; exit 0; fi
if [ "$1" = "restart" ]; then shift; echo "$@" >> "` + logPath + `"; exit 0; fi
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if out, err := exec.Command("sh", "-c", whisperDockerRestartCommand()).CombinedOutput(); err != nil {
		t.Fatalf("command failed: %v: %s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil || len(data) == 0 {
		t.Errorf("a matching whisper container was not restarted (log=%q, err=%v)", data, err)
	}
}
