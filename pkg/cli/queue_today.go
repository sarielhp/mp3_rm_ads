package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
)

func buildQueueTodaySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "today",
		Description: "Queue downloaded, uncleaned episodes published today (local calendar date)",
		UsageLine:   "abs queue today [options]",
		Args:        clihelp.MaximumNArgs(0),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress output"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show eligible episodes without changing the queue"),
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "today"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func knownQueuePublicationTime(path string, cached map[string]time.Time) time.Time {
	if st, err := loadEpisodeStatus(statusPathFor(path)); err == nil && st != nil {
		if published, err := time.Parse(time.RFC3339, st.PublishedAt); err == nil && !published.IsZero() {
			return published
		}
	}
	return cached[filepath.Base(path)]
}

func todayQueueCandidates(dir string, now time.Time) []string {
	cached := make(map[string]time.Time)
	if index, _ := loadPodcastCache(dir); index != nil {
		for _, ep := range index.Episodes {
			if ep.PublishedAt > 0 {
				filename := ep.Filename
				if filename == "" && filepath.Dir(ep.Path) == dir {
					filename = filepath.Base(ep.Path)
				}
				cached[filename] = time.UnixMilli(ep.PublishedAt)
			}
		}
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var candidates []string
	for _, path := range findMP3Files(dir) {
		published := knownQueuePublicationTime(path, cached)
		if !published.IsZero() && !published.Before(start) && published.Before(end) && !isEpisodeClean(path) {
			candidates = append(candidates, filepath.Base(path))
		}
	}
	return candidates
}

func handleQueueToday(root string, cli CLIOptions, now time.Time) error {
	total := 0
	for _, pod := range scanPodcastDirs(root) {
		candidates := todayQueueCandidates(pod.dir, now)
		if cli.DryRun {
			for _, filename := range candidates {
				fmt.Printf("[dry-run] Would queue for AdR: %s\n", filepath.Join(pod.dir, filename))
			}
			continue
		}
		if len(candidates) == 0 {
			continue
		}
		err := updateQueue(pod.dir, func(entries []string) []string {
			existing := make(map[string]bool)
			for _, entry := range entries {
				existing[strings.ToLower(entry)] = true
			}
			for _, filename := range candidates {
				key := strings.ToLower(filename)
				if !existing[key] {
					entries = append(entries, filename)
					existing[key] = true
					total++
				}
			}
			return entries
		})
		if err != nil {
			return fmt.Errorf("queue today's episodes for %s: %w", pod.title, err)
		}
	}
	if !cli.Quiet && !cli.DryRun {
		fmt.Printf("Added %d uncleaned episode(s) published today (%s, local time) to the AdR queue.\n", total, now.Format("2006-01-02"))
	}
	return nil
}
