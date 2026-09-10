package cli

import (
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"fmt"
	"time"
)

func runQueueToday(cfg Config, root string, cli CLIOptions, now time.Time) error {
	if _, ready := podcast.SourcePublicationTime(root); ready {
		return handleQueueToday(root, cli, now)
	}
	if cfg.PodfetchDBPath == "" && cfg.PodfetchURL == "" && cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" {
		return handleQueueToday(root, cli, now)
	}
	b, err := backend.FromAppConfig(&cfg, true)
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
