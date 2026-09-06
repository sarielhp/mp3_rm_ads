package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMP3DiskDurationNative_NonExistent(t *testing.T) {
	dur := GetMP3DiskDurationNative("/non/existent/path/audio.mp3")
	if dur != 0 {
		t.Fatalf("expected 0 for non-existent file, got %v", dur)
	}
}

func TestGetMP3DiskDurationNative_CorruptStreamReturnsZero(t *testing.T) {
	dir := t.TempDir()
	corruptFile := filepath.Join(dir, "corrupt.mp3")
	// Invalid MP3 bytes that fail decoding before reaching EOF cleanly
	corruptData := []byte{0xFF, 0xFB, 0x90, 0x64, 0x00, 0x00, 0x00, 0x00, 0x12, 0x34, 0x56}
	if err := os.WriteFile(corruptFile, corruptData, 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	dur := GetMP3DiskDurationNative(corruptFile)
	if dur != 0 {
		t.Fatalf("expected 0 on decode failure, got %v", dur)
	}
}

func TestGetMP3DiskDuration_EmptyOrCorrupt(t *testing.T) {
	if dur := GetMP3DiskDuration(""); dur != 0 {
		t.Fatalf("expected 0 for empty path, got %v", dur)
	}

	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.mp3")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}
	if dur := GetMP3DiskDuration(emptyFile); dur != 0 {
		t.Fatalf("expected 0 for empty file, got %v", dur)
	}
}
