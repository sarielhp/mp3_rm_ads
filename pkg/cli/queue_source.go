package cli

import (
	"fmt"
	"pod/pkg/backend"
	"pod/pkg/podcast"
	"time"
)

func runQueueToday(cfg Config, root string, cli CLIOptions, now time.Time) error {
	if backend.IsStandalone(&cfg) {
		return handleQueueToday(root, cli, now)
	}
	if _, ready := podcast.SourcePublicationTime(root); ready {
		return handleQueueToday(root, cli, now)
	}
	if cfg.PodfetchDBPath == "" && cfg.PodfetchURL == "" {
		return handleQueueToday(root, cli, now)
	}
	b, err := backend.FromAppConfig(&cfg, nil)
	if err != nil {
		return err
	}
	source, err := queueSourceDates(b, root)
	if err != nil {
		return fmt.Errorf("read source publication dates: %w", err)
	}
	return handleQueueTodaySource(root, cli, now, source)
}

func queueSourceDates(b backend.Backend, root string) (map[string]time.Time, error) {
	return podcast.LoadSourcePublicationDates(b, root)
}

func queueCatalogDates(root string, episodes []backend.CatalogEpisode) (map[string]time.Time, error) {
	return podcast.CatalogPublicationDates(root, episodes)
}
