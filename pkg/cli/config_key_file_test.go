package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"pod/pkg/config"
)

func TestHandleConfigSetAndGetGeminiAPIKeyFile(t *testing.T) {
	var buf bytes.Buffer
	tmpDir := t.TempDir()
	config.SetTestConfigPath(filepath.Join(tmpDir, "config.json"))
	defer config.SetTestConfigPath("")

	var cfg Config

	if err := handleConfigSet(&buf, &cfg, "gemini-api-key-file", "/custom/path/key.txt"); err != nil {
		t.Fatalf("handleConfigSet failed: %v", err)
	}
	if cfg.GeminiAPIKeyFile != "/custom/path/key.txt" {
		t.Errorf("expected /custom/path/key.txt, got %q", cfg.GeminiAPIKeyFile)
	}

	if err := handleConfigSet(&buf, &cfg, "gemini_key_file", "/another/path/key.txt"); err != nil {
		t.Fatalf("handleConfigSet failed with underscore alias: %v", err)
	}
	if cfg.GeminiAPIKeyFile != "/another/path/key.txt" {
		t.Errorf("expected /another/path/key.txt, got %q", cfg.GeminiAPIKeyFile)
	}

	buf.Reset()
	if err := handleConfigGet(&buf, cfg, "gemini-api-key-file"); err != nil {
		t.Fatalf("handleConfigGet failed: %v", err)
	}

	if strings.TrimSpace(buf.String()) != "/another/path/key.txt" {
		t.Errorf("expected /another/path/key.txt, got %q", strings.TrimSpace(buf.String()))
	}
}
