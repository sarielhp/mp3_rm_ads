package cli

// Turning what was typed on the command line into a list of things to process
// is the command line's job, not the engine's. These resolvers read the flags
// and positional arguments; pkg/adremoval receives only the resolved paths.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pod/pkg/podcast"
)

func resolveTargetAudioArgs(cli CLIOptions, config Config) ([]string, bool) {
	podcastsDir := config.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if cli.Podcast != "" {
		podcasts := podcast.ScanPodcastDirs(podcastsDir)
		matched, err := podcast.MatchLocalPodcasts(podcasts, cli.Podcast)
		if err != nil {
			if !errors.Is(err, podcast.ErrAmbiguousPodcast) && !cli.Quiet {
				fmt.Fprintf(outFor(cli), "Podcast matching '%s' not found.\n", cli.Podcast)
			}
			return nil, false
		}
		return []string{matched.Dir}, true
	}

	if len(cli.Args) == 1 {
		arg := cli.Args[0]
		if !strings.HasSuffix(strings.ToLower(arg), ".mp3") && !strings.HasSuffix(strings.ToLower(arg), ".json") {
			podcasts := podcast.ScanPodcastDirs(podcastsDir)
			matched, err := podcast.MatchLocalPodcasts(podcasts, arg)
			if err == nil {
				return []string{matched.Dir}, true
			}
			if errors.Is(err, podcast.ErrAmbiguousPodcast) {
				return nil, false
			}
			if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
				return []string{arg}, true
			}
			fmt.Fprintf(progressFor(cli), "Podcast matching '%s' not found.\n", arg)
			return nil, false
		}
		return cli.Args, true
	}

	if len(cli.Args) == 0 {
		if config.PodcastsDir != "" {
			return []string{config.PodcastsDir}, true
		}
		fmt.Fprintln(outFor(cli), "ERROR: No files or directories specified, and podcasts_dir is not configured.")
		return nil, false
	}
	return cli.Args, true
}

func resolvePodcastTarget(podcastsDir string, cli CLIOptions) (*ResolvedPodcast, error) {
	candidate := ""
	explicitPodcast := false
	if cli.Podcast != "" {
		candidate = cli.Podcast
		explicitPodcast = true
	} else if len(cli.Args) == 1 {
		candidate = cli.Args[0]
	} else {
		return nil, nil
	}

	cleanCand := strings.TrimSpace(candidate)
	if cleanCand == "" {
		return nil, nil
	}

	lower := strings.ToLower(cleanCand)
	if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".m4a") ||
		strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".aac") {
		return nil, nil
	}

	if podcastsDir == "" {
		podcastsDir = "."
	}

	if isExcludedRootPodcastsDir(podcastsDir, cleanCand) {
		return nil, nil
	}

	res, err := podcast.ResolveAnyID(podcastsDir, cleanCand)
	if err != nil {
		if errors.Is(err, podcast.ErrAmbiguousPodcast) {
			return nil, err
		}
		if explicitPodcast {
			return nil, fmt.Errorf("podcast matching %q not found", cleanCand)
		}
		return nil, nil
	}

	if res.IsPodcast() && !isExcludedRootPodcastsDir(podcastsDir, res.Podcast.Dir) {
		return res.Podcast, nil
	}

	if explicitPodcast {
		return nil, fmt.Errorf("identifier %q is not a podcast", cleanCand)
	}

	return nil, nil
}

func isExcludedRootPodcastsDir(podcastsDir, cand string) bool {
	absCand, err1 := filepath.Abs(cand)
	absRoot, err2 := filepath.Abs(podcastsDir)
	if err1 != nil || err2 != nil {
		return false
	}
	return absCand == absRoot
}
