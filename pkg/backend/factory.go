package backend

import (
	"fmt"
	"strings"

	"abs/pkg/types"
)

func IsAudiobookshelfActive(cfg *types.Config) bool {
	if cfg == nil {
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
	if cfg == nil {
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
