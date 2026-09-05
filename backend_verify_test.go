package main

import (
	"strings"
	"testing"

	"github.com/sariel/abs/pkg/backend"
)

func TestAudiobookshelfRuntimeVerificationBlocked(t *testing.T) {
	backend.SetAudiobookshelfDisabled(true)
	defer backend.SetAudiobookshelfDisabled(false)

	assertPanics(t, "NewAudiobookshelf", func() {
		_ = backend.NewAudiobookshelf(backend.Config{Host: "http://localhost:8087"})
	})

	be := &backend.AudiobookshelfBackend{Host: "http://localhost:8087"}

	assertPanics(t, "Login", func() {
		_, _ = be.Login()
	})

	assertPanics(t, "Request", func() {
		_, _ = be.Request("/api/test", "GET", nil)
	})

	assertPanics(t, "GetTokenFromDB", func() {
		_ = backend.GetTokenFromDB("/tmp/fake.db")
	})

	assertPanics(t, "ResetPodcastDateCheckInDB", func() {
		_ = backend.ResetPodcastDateCheckInDB("/tmp/fake.db", "item-1", "title")
	})
}

func TestPodfetchRuntimeVerificationBlocked(t *testing.T) {
	backend.SetPodfetchDisabled(true)
	defer backend.SetPodfetchDisabled(false)

	assertPanics(t, "NewPodFetch", func() {
		_ = backend.NewPodFetch(backend.Config{Host: "http://localhost:8094"})
	})

	be := &backend.PodFetchBackend{Host: "http://localhost:8094", DBPath: "/tmp/fake.db"}

	assertPanics(t, "Login", func() {
		_, _ = be.Login()
	})

	assertPanics(t, "Request", func() {
		_, _ = be.Request("/api/v1/podcasts", "GET", nil)
	})

	assertPanics(t, "ResetPodcastDateCheck", func() {
		_ = be.ResetPodcastDateCheck("1", "title")
	})

	assertPanics(t, "DeletePodcast", func() {
		_ = be.DeletePodcast("1")
	})
}

func TestFactoryRuntimeVerificationBlocked(t *testing.T) {
	backend.SetAudiobookshelfDisabled(true)
	backend.SetPodfetchDisabled(true)
	defer func() {
		backend.SetAudiobookshelfDisabled(false)
		backend.SetPodfetchDisabled(false)
	}()

	assertPanics(t, "Factory ABS", func() {
		_, _ = backend.New("audiobookshelf", backend.Config{})
	})

	assertPanics(t, "Factory ABS alias", func() {
		_, _ = backend.New("abs", backend.Config{})
	})

	assertPanics(t, "Factory Podfetch", func() {
		_, _ = backend.New("podfetch", backend.Config{})
	})

	assertPanics(t, "Factory Podfetch alias", func() {
		_, _ = backend.New("pod_fetch", backend.Config{})
	})
}

func TestBackendAllowedAndBlockedByConfig(t *testing.T) {
	podCfg := Config{BackendType: "podfetch", PodfetchURL: "http://localhost:8094"}
	absCfg := Config{BackendType: "audiobookshelf", AudiobookshelfURL: "http://localhost:8087"}

	applyBackendVerification(podCfg)
	if !backend.AudiobookshelfDisabled || backend.PodfetchDisabled {
		t.Fatalf("expected Audiobookshelf disabled and Podfetch enabled for podfetch config")
	}

	assertPanics(t, "verifyAudiobookshelfAllowedWithConfig when podfetch active", func() {
		verifyAudiobookshelfAllowedWithConfig(podCfg, "absOp")
	})

	applyBackendVerification(absCfg)
	if backend.AudiobookshelfDisabled || !backend.PodfetchDisabled {
		t.Fatalf("expected Audiobookshelf enabled and Podfetch disabled for audiobookshelf config")
	}

	assertPanics(t, "verifyPodfetchAllowedWithConfig when abs active", func() {
		verifyPodfetchAllowedWithConfig(absCfg, "podfetchOp")
	})

	backend.SetAudiobookshelfDisabled(false)
	backend.SetPodfetchDisabled(false)
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s was expected to panic with runtime verification failure, but did not panic", name)
		} else if msg, ok := r.(string); ok {
			if !strings.Contains(msg, "Runtime verification failed") {
				t.Errorf("%s panic message %q does not contain expected verification failure text", name, msg)
			}
		}
	}()
	fn()
}
