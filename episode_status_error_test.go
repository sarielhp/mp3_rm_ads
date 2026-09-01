package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateEpisodeStatusReportsAWriteFailure(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "Show")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	ep := filepath.Join(sub, "ep.mp3")
	if err := os.WriteFile(ep, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	// Make the directory unwritable so the status sidecar cannot be created.
	if err := os.Chmod(sub, 0500); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0755) })
	if os.Geteuid() == 0 {
		t.Skip("running as root; permissions are not enforced")
	}

	err := updateEpisodeStatus(ep, func(st *EpisodeStatusFile) { st.Status = StateDone })
	if err == nil {
		t.Errorf("a status write that could not succeed reported success; the pipeline's " +
			"only idempotency marker is written blind")
	}
}

func TestUpdateEpisodeStatusSucceedsNormally(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "ep.mp3")
	if err := os.WriteFile(ep, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := updateEpisodeStatus(ep, func(st *EpisodeStatusFile) { st.Status = StateDone }); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !isEpisodeCompleted(ep) {
		t.Errorf("status was not persisted")
	}
}
