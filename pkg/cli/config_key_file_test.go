package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pod/pkg/config"
)

func TestHandleConfigSetAndGetGeminiAPIKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	config.SetTestConfigPath(filepath.Join(tmpDir, "config.json"))
	defer config.SetTestConfigPath("")

	var cfg Config

	if err := handleConfigSet(&cfg, "gemini-api-key-file", "/custom/path/key.txt"); err != nil {
		t.Fatalf("handleConfigSet failed: %v", err)
	}
	if cfg.GeminiAPIKeyFile != "/custom/path/key.txt" {
		t.Errorf("expected /custom/path/key.txt, got %q", cfg.GeminiAPIKeyFile)
	}

	if err := handleConfigSet(&cfg, "gemini_key_file", "/another/path/key.txt"); err != nil {
		t.Fatalf("handleConfigSet failed with underscore alias: %v", err)
	}
	if cfg.GeminiAPIKeyFile != "/another/path/key.txt" {
		t.Errorf("expected /another/path/key.txt, got %q", cfg.GeminiAPIKeyFile)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := handleConfigGet(cfg, "gemini-api-key-file")
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("handleConfigGet failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if strings.TrimSpace(buf.String()) != "/another/path/key.txt" {
		t.Errorf("expected /another/path/key.txt, got %q", strings.TrimSpace(buf.String()))
	}
}
