package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var realHomeBeforeSandbox string

func TestMain(m *testing.M) {
	realHomeBeforeSandbox = os.Getenv("HOME")

	root, err := os.MkdirTemp("", "abs-testroot-")
	if err != nil {
		panic("failed to create test sandbox: " + err.Error())
	}

	os.Setenv("HOME", root)
	os.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	os.Unsetenv("XDG_CONFIG_HOME")
	playerSpawnEnabled = false

	code := m.Run()

	os.RemoveAll(root)
	os.Exit(code)
}

func TestSuiteNeverResolvesPathsUnderTheRealHome(t *testing.T) {
	if realHomeBeforeSandbox == "" {
		t.Skip("HOME was not set before the sandbox was installed")
	}
	prefix := realHomeBeforeSandbox + string(os.PathSeparator)

	for _, tc := range []struct {
		name string
		path string
	}{
		{"play queue", getPlayQueueFilePath()},
		{"podcast cache", cacheBaseDir()},
		{"config", configPath()},
	} {
		if strings.HasPrefix(tc.path, prefix) {
			t.Errorf("%s resolves to %s, under the real home %s; the suite would mutate live user state",
				tc.name, tc.path, realHomeBeforeSandbox)
		}
	}
}

func TestSuiteDoesNotSpawnAnAudioPlayer(t *testing.T) {
	if playerSpawnEnabled {
		t.Fatal("playerSpawnEnabled is true under test; go test would launch ffplay/mpg123")
	}
}
