package cli

import (
	"abs/pkg/backend"
	"abs/pkg/podcast"
	"fmt"
	"time"
)

func preparePublicationSource(action string, cfg Config, cli CLIOptions) error {
	podcast.SetPublicationSource("", nil)
	if action == "info" && cli.InfoSubcmd == "transcript" {
		return nil
	}
	if action == "offload" && cli.RemoteSubcmd != "push" {
		return nil
	}
	if action == "queue" && (cli.QueueSubcmd == "clear" || cli.QueueSubcmd == "remove" || cli.QueueSubcmd == "priority") {
		return nil
	}
	switch action {
	case "info", "queue", "rm_ads", "tui", "offload":
	default:
		return nil
	}
	if cfg.PodfetchDBPath == "" && cfg.PodfetchURL == "" && cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" {
		return nil
	}
	root := cfg.PodcastsDir
	if cli.PodcastsDir != "" {
		root = cli.PodcastsDir
	}
	b, err := backend.FromAppConfig(&cfg, true)
	if err != nil {
		return err
	}
	dates, err := podcast.LoadSourcePublicationDates(b, root)
	if err != nil {
		return fmt.Errorf("load source publication dates: %w", err)
	}
	podcast.SetPublicationSource(root, dates)
	return nil
}

func publicationDateTime(date time.Time) string {
	if date.IsZero() {
		return "—"
	}
	return queuePublicationTime(date, time.Now())
}
