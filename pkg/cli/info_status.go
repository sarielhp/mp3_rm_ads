package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/remote"
	"abs/pkg/util"
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

	renderedABS := false
	if backend.IsAudiobookshelfActive(&cfg) && cfg.AudiobookshelfURL != "" {
		if err := renderABSPodcastStatus(cfg, cfg.AudiobookshelfURL, "", podcastsDir, quiet); err == nil {
			renderedABS = true
		}
	}

	if !renderedABS {
		renderLocalDiskPodcastStatus(podcastsDir, quiet)
	}
}

func renderABSPodcastStatus(cfg Config, baseURL, token, podcastsDir string, quiet bool) error {
	b, err := backend.FromAppConfig(&cfg, quiet)
	if err != nil {
		return err
	}
	libs, err := b.PodcastLibraries()
	if err != nil {
		return err
	}
	if len(libs) == 0 {
		return fmt.Errorf("no podcast libraries found in ABS")
	}
	allItems, err := b.Podcasts()
	if err != nil {
		return err
	}
	if len(allItems) == 0 {
		return fmt.Errorf("no podcasts found in ABS")
	}

	localByName, podIDByDir := buildPodcastMapsForStatus(podcastsDir)
	if !quiet {
		fmt.Printf("\n%s\n", strings.Repeat("=", 90))
		fmt.Println("AUDIOBOOKSHELF DATABASE STATUS REPORT (DRY RUN)")
		fmt.Printf("%s\n", strings.Repeat("=", 90))
		fmt.Printf("  %-3s  %-6s  %-48s │ %-8s │ %-10s\n", "#", "ID", "Title", "Episodes", "NeedAdR")
		fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-10s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 10))
	}

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for idx, item := range allItems {
		title := item.Media.Metadata.Title
		dName := util.DisplayName(title)
		if len(dName) > 48 {
			dName = dName[:45] + "..."
		}
		absEpisodeCount := len(item.Media.Episodes)
		shortID, needsAdRemoval := calculateItemAdRemovalCount(item, localByName, podIDByDir)

		totalEpisodes += absEpisodeCount
		totalNeedsAdRemoval += needsAdRemoval

		if !quiet {
			fmt.Printf("  %-3d  %-6s  %-48s │ %-8d │ %-16d\n", idx+1, shortID, dName, absEpisodeCount, needsAdRemoval)
		}
	}

	if !quiet {
		fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 16))
		fmt.Printf("  %-3s  %-6s  %-48s │ %-8d │ %-16d\n", "", "", "TOTAL", totalEpisodes, totalNeedsAdRemoval)
		fmt.Printf("%s\n\n", strings.Repeat("=", 90))
	}
	return nil
}

func buildPodcastMapsForStatus(podcastsDir string) (map[string]podcast.PodcastDirEntry, map[string]string) {
	podEntries := podcast.ScanPodcastDirs(podcastsDir)
	localByName := make(map[string]podcast.PodcastDirEntry)
	podIDByDir := make(map[string]string)
	for _, p := range podEntries {
		localByName[strings.ToLower(p.Title)] = p
		localByName[strings.ToLower(p.FolderName)] = p
		localByName[strings.ToLower(filepath.Base(p.Dir))] = p

		podIDByDir[p.Dir] = p.ShortID
		podIDByDir[strings.ToLower(p.Title)] = p.ShortID
		podIDByDir[strings.ToLower(p.FolderName)] = p.ShortID
	}
	return localByName, podIDByDir
}

func calculateItemAdRemovalCount(item Podcast, localByName map[string]podcast.PodcastDirEntry, podIDByDir map[string]string) (string, int) {
	title := item.Media.Metadata.Title
	relBase := filepath.Base(item.RelPath)
	lp, ok := localByName[strings.ToLower(title)]
	if !ok {
		lp, ok = localByName[strings.ToLower(relBase)]
	}

	needsAdRemoval := 0
	shortID := ""
	if ok {
		shortID = podIDByDir[lp.Dir]
		if shortID == "" {
			shortID = podcast.GetOrSetPodcastShortID(lp.Dir, title)
		}
		mp3Files, _ := filepath.Glob(filepath.Join(lp.Dir, "*.mp3"))
		podCfg := config.LoadPodcastConfig(lp.Dir, config.PodcastConfig{})
		if podCfg.AdRemoval != config.AdRemovalNone {
			filtered := podcast.FilterByAdRemovalPolicy(mp3Files, lp.Dir, podCfg)
			for _, mp3 := range filtered {
				_ = pipeline.GetOrCreateEpisodeStatus(mp3)
				if !pipeline.IsEpisodeCompleted(mp3) {
					needsAdRemoval++
				}
			}
		}
	} else {
		shortID = podIDByDir[strings.ToLower(title)]
		if shortID == "" {
			shortID = podcast.GeneratePodcastShortID(title)
		}
	}
	return shortID, needsAdRemoval
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
		dName := util.DisplayName(e.name)
		if len(dName) > 48 {
			dName = dName[:45] + "..."
		}
		totalEpisodes += e.episodes
		totalNeedsAdRemoval += e.needsAdRemoval
		fmt.Printf("  %-3d  %-6s  %-48s │ %-8d │ %-16d\n", idx+1, e.id, dName, e.episodes, e.needsAdRemoval)
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
