package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pod/pkg/config"
	"pod/pkg/pipeline"
	"pod/pkg/podcast"
	"pod/pkg/remote"
	"pod/pkg/util"
)

type podcastStatusEntry struct {
	id             string
	name           string
	episodes       int
	needsAdRemoval int
}

func absStatus(cfg Config, showDetailed bool, quiet bool) {
	if showDetailed {
		targetHost := cfg.RemoteHost
		if targetHost == "" {
			targetHost = cfg.RemoteFFmpegHost
		}
		if targetHost != "" && !strings.EqualFold(targetHost, "local") {
			renderRemoteStatusSection(&cfg, targetHost, nil, quiet)
		}
		renderLocalLibraryStatus(cfg, quiet)
		return
	}

	renderLocalSummary(cfg, quiet)

	targetHost := cfg.RemoteHost
	if targetHost == "" {
		targetHost = cfg.RemoteFFmpegHost
	}
	if targetHost != "" && !strings.EqualFold(targetHost, "local") {
		renderRemoteStatusSection(&cfg, targetHost, nil, quiet)
	}
}

func renderLocalSummary(cfg Config, quiet bool) (int, int, int) {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	podcastsCount := 0
	totalEpisodes := 0
	totalNeedsAd := 0

	dirEntries, err := os.ReadDir(podcastsDir)
	if err == nil {
		for _, de := range dirEntries {
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") || de.Name() == ".work" || strings.HasSuffix(de.Name(), "-1") {
				continue
			}
			podPath := filepath.Join(podcastsDir, de.Name())
			mp3s := util.FindMP3Files(podPath)
			if len(mp3s) == 0 {
				continue
			}
			podcastsCount++
			totalEpisodes += len(mp3s)
			podCfg := config.LoadPodcastConfig(podPath, config.PodcastConfig{})
			if podCfg.AdRemoval == AdRemovalNone {
				continue
			}
			filtered := podcast.FilterByAdRemovalPolicy(mp3s, podPath, podCfg)
			for _, mp3 := range filtered {
				_ = pipeline.GetOrCreateEpisodeStatus(mp3)
				if !pipeline.IsEpisodeCompleted(mp3) {
					totalNeedsAd++
				}
			}
		}
	}

	if !quiet {
		fmt.Println()
		fmt.Println("=== Local Library Status ===")
		fmt.Printf("  - Version:           %s\n", getVersion())
		fmt.Printf("  - Podcasts:          %d\n", podcastsCount)
		fmt.Printf("  - Total Episodes:    %d\n", totalEpisodes)
		if totalNeedsAd > 0 {
			fmt.Printf("  - AdR Status:        %s\n", util.BoldYellow(fmt.Sprintf("%d episode(s) need AdR", totalNeedsAd)))
		} else {
			fmt.Printf("  - AdR Status:        %s\n", util.BoldGreen("0 (All clean)"))
		}
	}
	return podcastsCount, totalEpisodes, totalNeedsAd
}

func renderRemoteStatusSection(cfg *Config, targetHost string, transport RemoteTransport, quiet bool) {
	_ = remote.RunRemoteStatus(cfg, targetHost, transport, quiet, false)
}

func renderLocalLibraryStatus(cfg Config, quiet bool) {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}
	renderLocalDiskPodcastStatus(podcastsDir, quiet)
}

func renderLocalDiskPodcastStatus(podcastsDir string, quiet bool) {
	podEntries := podcast.ScanPodcastDirs(podcastsDir)
	var entries []podcastStatusEntry
	for _, pe := range podEntries {
		mp3s := util.FindMP3Files(pe.Dir)
		if len(mp3s) == 0 {
			continue
		}
		needsAd := 0
		podCfg := config.LoadPodcastConfig(pe.Dir, config.PodcastConfig{})
		if podCfg.AdRemoval != config.AdRemovalNone {
			filtered := podcast.FilterByAdRemovalPolicy(mp3s, pe.Dir, podCfg)
			for _, mp3 := range filtered {
				_ = pipeline.GetOrCreateEpisodeStatus(mp3)
				if !pipeline.IsEpisodeCompleted(mp3) {
					needsAd++
				}
			}
		}
		entries = append(entries, podcastStatusEntry{
			id:             pe.ShortID,
			name:           pe.Title,
			episodes:       len(mp3s),
			needsAdRemoval: needsAd,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	if quiet {
		return
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 90))
	fmt.Println("LOCAL LIBRARY PODCAST STATUS REPORT")
	fmt.Printf("%s\n", strings.Repeat("=", 90))
	fmt.Printf("  %-3s  %-6s  %-48s │ %-8s │ %-10s\n", "#", "ID", "Title", "Episodes", "NeedAdR")
	fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-10s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 10))

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for idx, e := range entries {
		dName := util.TruncateDisplayName(e.name, 48)
		totalEpisodes += e.episodes
		totalNeedsAdRemoval += e.needsAdRemoval
		fmt.Printf("  %-3d  %-6s  %s │ %-8d │ %-16d\n", idx+1, e.id, util.PadRight(dName, 48), e.episodes, e.needsAdRemoval)
	}

	fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 16))
	fmt.Printf("  %-3s  %-6s  %-48s │ %-8d │ %-16d\n", "", "", "TOTAL", totalEpisodes, totalNeedsAdRemoval)
	fmt.Printf("%s\n\n", strings.Repeat("=", 90))
}

func runStatusCommand(config *Config, cli CLIOptions) error {
	if cli.StatusSubcmd == "check" {
		return runCheckCommand(*config, cli)
	}
	showDetailedPodcasts := false
	targetDir := config.PodcastsDir
	if len(cli.Args) > 0 {
		arg := strings.ToLower(cli.Args[0])
		if arg == "podcasts" || arg == "podcast" || arg == "all" {
			showDetailedPodcasts = true
		} else if fi, err := os.Stat(cli.Args[0]); err == nil && fi.IsDir() {
			targetDir = cli.Args[0]
			showDetailedPodcasts = true
		}
	}
	if cli.Verbose {
		showDetailedPodcasts = true
	}
	if targetDir != "" {
		config.PodcastsDir = targetDir
	}
	absStatus(*config, showDetailedPodcasts, cli.Quiet)
	return nil
}
