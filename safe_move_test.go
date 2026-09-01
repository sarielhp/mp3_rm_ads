package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSafeMoveDoesNotDestroyDestinationWhenTheMoveFails(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "episode.mp3.precut")
	const original = "the only copy of the original audio"
	if err := os.WriteFile(dst, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	err := safeMove(filepath.Join(dir, "does-not-exist.mp3"), dst)
	if err == nil {
		t.Errorf("expected an error when the source does not exist")
	}

	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatalf("the destination was destroyed by a move that could not succeed: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("destination content changed: %q", got)
	}
}

func TestSafeMoveReplacesDestinationOnSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.mp3")
	dst := filepath.Join(dir, "old.mp3")
	if err := os.WriteFile(src, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := safeMove(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "new" {
		t.Errorf("dst = %q, want %q", got, "new")
	}
	if fileExists(src) {
		t.Errorf("source still exists after a successful move")
	}
}

func TestSafeMoveFallsBackToCopyAcrossFilesystems(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.mp3")
	dst := filepath.Join(dir, "out", "old.mp3")
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("payload across a mount point"), 0644); err != nil {
		t.Fatal(err)
	}

	orig := renameFn
	t.Cleanup(func() { renameFn = orig })
	first := true
	renameFn = func(o, n string) error {
		if first {
			first = false
			return &os.LinkError{Op: "rename", Old: o, New: n, Err: syscall.EXDEV}
		}
		return orig(o, n)
	}

	if err := safeMove(src, dst); err != nil {
		t.Fatalf("EXDEV fallback failed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("destination missing after EXDEV fallback: %v", err)
	}
	if string(got) != "payload across a mount point" {
		t.Errorf("content lost across the fallback: %q", got)
	}
	if fileExists(src) {
		t.Errorf("source still exists after a successful cross-device move")
	}
	if fileExists(dst + ".partial") {
		t.Errorf("the .partial staging file was left behind")
	}
}
