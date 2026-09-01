package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedWriteLeavesTheExistingFileIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ep.cuts.json")
	const original = `{"version":1,"cut_intervals":[{"start_sec":10,"end_sec":20}]}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	orig := renameFn
	t.Cleanup(func() { renameFn = orig })
	renameFn = func(o, n string) error {
		return &os.LinkError{Op: "rename", Old: o, New: n, Err: os.ErrPermission}
	}

	if err := writeFile(path, []byte("REPLACEMENT")); err == nil {
		t.Errorf("expected an error when the write cannot complete")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the existing file was destroyed by a write that could not complete: %v", err)
	}
	if string(got) != original {
		t.Errorf("the existing file was replaced by a partial write: %q", got)
	}
}

func TestAtomicWriteLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ep.cuts.json")
	if err := writeFile(path, []byte("content")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("a staging file was left behind: %s", e.Name())
		}
	}
}

func TestPlayQueueIsWrittenPrivately(t *testing.T) {
	p := &AudioPlayer{Volume: 70}
	p.Queue = []PlayerTrack{{Title: "Ep", Path: "/x/ep.mp3", Duration: 60}}
	p.SaveQueueToFile()

	fi, err := os.Stat(getPlayQueueFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Errorf("play_queue.json is mode %04o; it records listening history", fi.Mode().Perm())
	}
}
