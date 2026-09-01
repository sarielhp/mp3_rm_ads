package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkDirIsPerEpisodeNotPerFolder(t *testing.T) {
	a := workDirFor("/lib/Show/ep-a.mp3")
	b := workDirFor("/lib/Show/ep-b.mp3")
	if a == b {
		t.Fatalf("two episodes in one folder share a work dir (%s); finishing one "+
			"deletes the other's in-flight output", a)
	}
	for _, d := range []string{a, b} {
		if !strings.Contains(d, string(filepath.Separator)+workDirName+string(filepath.Separator)) {
			t.Errorf("%s is not inside a %s/ directory, so verifyTempFile would reject it", d, workDirName)
		}
	}
}

func TestFinishingOneEpisodeKeepsAnotherInFlightOutput(t *testing.T) {
	dir := t.TempDir()
	epA := filepath.Join(dir, "ep-a.mp3")
	epB := filepath.Join(dir, "ep-b.mp3")

	workA := workDirFor(epA)
	if err := os.MkdirAll(workA, 0755); err != nil {
		t.Fatal(err)
	}
	inflight := filepath.Join(workA, "ep-a.mp3.tmp.mp3")
	if err := os.WriteFile(inflight, []byte("A's freshly cut audio"), 0644); err != nil {
		t.Fatal(err)
	}

	// Instance B finishes and cleans up after itself.
	workB := workDirFor(epB)
	if err := os.MkdirAll(workB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(workB); err != nil {
		t.Fatal(err)
	}

	if !fileExists(inflight) {
		t.Errorf("instance B's cleanup destroyed instance A's in-flight cut output")
	}
}

func TestVerifyTempFileStillAcceptsPerEpisodeWorkPaths(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "ep.mp3")
	tmp := filepath.Join(workDirFor(ep), "ep.mp3.tmp.mp3")
	// verifyTempFile aborts the process on rejection, so assert the predicate it uses.
	if !strings.Contains(tmp, string(filepath.Separator)+workDirName+string(filepath.Separator)) {
		t.Errorf("%s would be rejected by verifyTempFile", tmp)
	}
}
