package cli

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/util"
	"github.com/sarielhp/clihelp"
)

func buildQueueLatestSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "latest",
		Description: "Queue latest published episodes that do not have their ads removed yet",
		UsageLine:   "abs queue latest [N] [podcast-id] [options]",
		Args:        clihelp.MaximumNArgs(2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.Quiet, "-q, --quiet", false, "Suppress progress output"),
			clihelp.Bool(&opts.DryRun, "--dry-run", false, "Show eligible episodes without changing the queue"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs queue latest 5",
				Description: "Queue the 5 latest published uncleaned episodes",
			},
			{
				Line:        "abs queue latest 3 <podcast-id>",
				Description: "Queue the 3 latest published uncleaned episodes for a specific podcast",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "latest"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func parseQueueLatestArgs(args []string) (int, string, error) {
	limit := 5
	target := ""
	for _, arg := range args {
		if n, err := strconv.Atoi(arg); err == nil {
			if n <= 0 {
				return 0, "", fmt.Errorf("invalid episode count %q (must be a positive integer)", arg)
			}
			limit = n
		} else {
			if target != "" {
				return 0, "", fmt.Errorf("unexpected argument %q", arg)
			}
			target = arg
		}
	}
	return limit, target, nil
}

type latestQueueCandidate struct {
	path           string
	podDir         string
	podTitle       string
	podShortID     string
	episodeShortID string
	title          string
	pubTime        time.Time
}

func collectLatestUncleanedEpisodes(podcastsDir, target string) ([]latestQueueCandidate, error) {
	var entries []podcast.PodcastDirEntry
	if target != "" {
		res, err := resolveQueueTarget(podcastsDir, target)
		if err != nil {
			return nil, err
		}
		if res.IsPodcast() {
			entries = []podcast.PodcastDirEntry{{
				Dir:        res.Podcast.Dir,
				FolderName: res.Podcast.FolderName,
				Title:      res.Podcast.Title,
				ShortID:    res.Podcast.ShortID,
			}}
		} else {
			return nil, fmt.Errorf("target %q is an episode, not a podcast", target)
		}
	} else {
		entries = scanQueuePodcasts(podcastsDir)
	}

	var candidates []latestQueueCandidate
	for _, p := range entries {
		for _, path := range util.FindMP3Files(p.Dir) {
			if !pipeline.IsQueueAudioPath(path) || pipeline.IsEpisodeClean(path) {
				continue
			}
			pubTime := podcast.GetEpisodePublicationTime(path)
			if pubTime.IsZero() {
				if fi, err := os.Stat(path); err == nil {
					pubTime = fi.ModTime()
				}
			}
			epShortID := podcast.EpisodeShortIDReadOnly(p.Dir, p.ShortID, path)
			title := podcast.EpisodeTitleFromPath(path)
			candidates = append(candidates, latestQueueCandidate{
				path:           path,
				podDir:         p.Dir,
				podTitle:       p.Title,
				podShortID:     p.ShortID,
				episodeShortID: epShortID,
				title:          title,
				pubTime:        pubTime,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].pubTime.Equal(candidates[j].pubTime) {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].pubTime.After(candidates[j].pubTime)
	})

	return candidates, nil
}

func runQueueLatest(cfg Config, podcastsDir string, limit int, target string, cli CLIOptions) error {
	candidates, err := collectLatestUncleanedEpisodes(podcastsDir, target)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		if !cli.Quiet {
			fmt.Println("No uncleaned episodes found to queue.")
		}
		return nil
	}

	if limit > len(candidates) {
		limit = len(candidates)
	}
	selected := candidates[:limit]

	if cli.DryRun {
		for _, it := range selected {
			fmt.Printf("[dry-run] Would queue for AdR: [%s] %s (%s)\n",
				it.episodeShortID, util.DisplayName(it.title), util.DisplayName(it.podTitle))
		}
		return nil
	}

	addedCount, alreadyCount := 0, 0
	for _, it := range selected {
		qFile := queueFilenameForPath(it.podDir, it.path)
		added, err := pipeline.AddToQueueChecked(it.podDir, qFile)
		if err != nil {
			return fmt.Errorf("queue episode %s: %w", it.title, err)
		}
		if added {
			addedCount++
			if !cli.Quiet {
				fmt.Printf("Added to queue: [%s] %s (%s)\n",
					util.BoldCyan(it.episodeShortID), util.DisplayName(it.title), util.Bold(util.DisplayName(it.podTitle)))
			}
		} else {
			alreadyCount++
			if !cli.Quiet {
				fmt.Printf("Already in queue: [%s] %s (%s)\n",
					util.BoldCyan(it.episodeShortID), util.DisplayName(it.title), util.Bold(util.DisplayName(it.podTitle)))
			}
		}
	}

	if !cli.Quiet {
		if alreadyCount > 0 {
			fmt.Printf("Added %d uncleaned episode(s) to the AdR queue (%d already queued).\n", addedCount, alreadyCount)
		} else {
			fmt.Printf("Added %d uncleaned episode(s) to the AdR queue.\n", addedCount)
		}
	}
	return nil
}
