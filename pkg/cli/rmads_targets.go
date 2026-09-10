package cli

// Turning what was typed on the command line into a list of things to process
// is the command line's job, not the engine's. These resolvers read the flags
// and positional arguments; pkg/adremoval receives only the resolved paths.

import (
	"abs/pkg/podcast"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveTargetAudioArgs(cli CLIOptions, config Config) ([]string, bool) {
	podcastsDir := config.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if cli.Podcast != "" {
		targetDir, _, found := podcast.ResolvePodcastDirByIDOrName(podcastsDir, cli.Podcast)
		if !found {
			if !cli.Quiet {
				fmt.Printf("Podcast matching '%s' not found.\n", cli.Podcast)
			}
			return nil, false
		}
		return []string{targetDir}, true
	}

	if len(cli.Args) == 1 {
		arg := cli.Args[0]
		if !strings.HasSuffix(strings.ToLower(arg), ".mp3") && !strings.HasSuffix(strings.ToLower(arg), ".json") {
			if targetDir, _, found := podcast.ResolvePodcastDirByIDOrName(podcastsDir, arg); found {
				return []string{targetDir}, true
			}
			if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
				return []string{arg}, true
			}
			if !cli.Quiet {
				fmt.Printf("Podcast matching '%s' not found.\n", arg)
			}
			return nil, false
		}
		return cli.Args, true
	}

	if len(cli.Args) == 0 {
		if config.PodcastsDir != "" {
			return []string{config.PodcastsDir}, true
		}
		fmt.Println("ERROR: No files or directories specified, and podcasts_dir is not configured.")
		return nil, false
	}
	return cli.Args, true
}

func resolvePodcastTarget(podcastsDir string, cli CLIOptions) (*ResolvedPodcast, bool) {
	candidate := ""
	if cli.Podcast != "" {
		candidate = cli.Podcast
	} else if len(cli.Args) == 1 {
		candidate = cli.Args[0]
	} else {
		return nil, false
	}

	cleanCand := strings.TrimSpace(candidate)
	if cleanCand == "" {
		return nil, false
	}

	lower := strings.ToLower(cleanCand)
	if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".m4a") ||
		strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".aac") {
		return nil, false
	}

	if podcastsDir == "" {
		podcastsDir = "."
	}

	if isExcludedRootPodcastsDir(podcastsDir, cleanCand) {
		return nil, false
	}

	if res, err := podcast.ResolveAnyID(podcastsDir, cleanCand); err == nil && res.IsPodcast() {
		if !isExcludedRootPodcastsDir(podcastsDir, res.Podcast.Dir) {
			return res.Podcast, true
		}
	}

	if dir, title, found := podcast.ResolvePodcastDirByIDOrName(podcastsDir, cleanCand); found {
		if !isExcludedRootPodcastsDir(podcastsDir, dir) {
			shortID := podcast.GetOrSetPodcastShortID(dir, title)
			cfg := loadPodcastConfig(dir)
			return &ResolvedPodcast{
				Dir:        dir,
				Title:      title,
				ShortID:    shortID,
				FolderName: filepath.Base(dir),
				UUID:       cfg.ID,
				Config:     cfg,
			}, true
		}
	}

	return nil, false
}

func isExcludedRootPodcastsDir(podcastsDir, cand string) bool {
	absCand, err1 := filepath.Abs(cand)
	absRoot, err2 := filepath.Abs(podcastsDir)
	if err1 != nil || err2 != nil {
		return false
	}
	return absCand == absRoot
}
