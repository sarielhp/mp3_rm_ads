package backend

import (
	"fmt"
	"strings"

	"abs/pkg/types"
)

func IsStandalone(cfg *types.Config) bool {
	if cfg == nil {
		return false
	}
	bt := strings.ToLower(strings.TrimSpace(cfg.BackendType))
	return bt == "standalone" || bt == "local" || bt == "none"
}

func IsAudiobookshelfActive(cfg *types.Config) bool {
	if cfg == nil || IsStandalone(cfg) {
		return false
	}
	if strings.EqualFold(cfg.BackendType, "podfetch") || strings.EqualFold(cfg.BackendType, "pod_fetch") {
		return false
	}
	if strings.EqualFold(cfg.BackendType, "audiobookshelf") || strings.EqualFold(cfg.BackendType, "abs") {
		return true
	}
	if cfg.PodfetchURL != "" || cfg.PodfetchDBPath != "" {
		return false
	}
	return true
}

func IsPodfetchActive(cfg *types.Config) bool {
	if cfg == nil || IsStandalone(cfg) {
		return false
	}
	if strings.EqualFold(cfg.BackendType, "audiobookshelf") || strings.EqualFold(cfg.BackendType, "abs") {
		return false
	}
	if strings.EqualFold(cfg.BackendType, "podfetch") || strings.EqualFold(cfg.BackendType, "pod_fetch") {
		return true
	}
	return cfg.PodfetchURL != "" || cfg.PodfetchDBPath != ""
}

func FromAppConfig(cfg *types.Config, quiet bool) (Backend, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if IsStandalone(cfg) {
		return nil, fmt.Errorf("backend is standalone")
	}
	if IsPodfetchActive(cfg) {
		SetAudiobookshelfDisabled(true)
		SetPodfetchDisabled(false)
		bCfg := Config{
			Host:        cfg.PodfetchURL,
			User:        cfg.PodfetchUser,
			Pass:        cfg.PodfetchPass,
			Token:       cfg.PodfetchAPIKey,
			APIKey:      cfg.PodfetchAPIKey,
			DBPath:      cfg.PodfetchDBPath,
			PodcastsDir: cfg.PodcastsDir,
			Quiet:       quiet,
		}
		return New("podfetch", bCfg)
	}

	SetAudiobookshelfDisabled(false)
	SetPodfetchDisabled(true)
	bCfg := Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       cfg.AudiobookshelfToken,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	}
	return New("audiobookshelf", bCfg)
}

// ReaderFromAppConfig returns a PodcastReader from configured backend settings, even in standalone mode.
func ReaderFromAppConfig(cfg *types.Config, quiet bool) (PodcastReader, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.PodfetchDBPath != "" || cfg.PodfetchURL != "" {
		SetAudiobookshelfDisabled(true)
		SetPodfetchDisabled(false)
		return New("podfetch", Config{
			Host:        cfg.PodfetchURL,
			User:        cfg.PodfetchUser,
			Pass:        cfg.PodfetchPass,
			Token:       cfg.PodfetchAPIKey,
			APIKey:      cfg.PodfetchAPIKey,
			DBPath:      cfg.PodfetchDBPath,
			PodcastsDir: cfg.PodcastsDir,
			Quiet:       quiet,
		})
	}
	if cfg.AudiobookshelfDBPath != "" || cfg.AudiobookshelfURL != "" {
		SetAudiobookshelfDisabled(false)
		SetPodfetchDisabled(true)
		return New("audiobookshelf", Config{
			Host:        cfg.AudiobookshelfURL,
			User:        cfg.AudiobookshelfUser,
			Pass:        cfg.AudiobookshelfPass,
			Token:       cfg.AudiobookshelfToken,
			DBPath:      cfg.AudiobookshelfDBPath,
			PodcastsDir: cfg.PodcastsDir,
			Quiet:       quiet,
		})
	}
	return nil, fmt.Errorf("no podcast backend configured")
}

// SyncEpisodeDuration synchronizes the duration of a cleaned episode back to the configured backend.
func SyncEpisodeDuration(cfg *types.Config, filePath string, duration float64) error {
	if cfg == nil {
		return nil
	}
	if cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" && cfg.PodfetchURL == "" && cfg.PodfetchDBPath == "" {
		return nil
	}
	b, err := FromAppConfig(cfg, true)
	if err != nil || b == nil {
		return err
	}
	return b.SyncDuration(filePath, duration)
}
