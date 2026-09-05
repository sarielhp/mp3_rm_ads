package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			mp3s := findMP3Files(podPath)
			if len(mp3s) == 0 {
				continue
			}
			podcastsCount++
			totalEpisodes += len(mp3s)
			podCfg := loadPodcastConfig(podPath)
			if podCfg.AdRemoval == AdRemovalNone {
				continue
			}
			filtered := filterMP3FilesByPodcastConfig(mp3s, podPath, podCfg)
			for _, mp3 := range filtered {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
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
			fmt.Printf("  - Needs Ad Removal:  %s\n", boldYellow(fmt.Sprintf("%d episode(s)", totalNeedsAd)))
		} else {
			fmt.Printf("  - Needs Ad Removal:  %s\n", boldGreen("0 (All clean)"))
		}
	}
	return podcastsCount, totalEpisodes, totalNeedsAd
}

func renderRemoteStatusSection(cfg *Config, targetHost string, transport RemoteTransport, quiet bool) {
	_ = runRemoteStatus(cfg, targetHost, transport, quiet, false)
}

func renderLocalLibraryStatus(cfg Config, quiet bool) {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	renderedABS := false
	if isAudiobookshelfActive(cfg) && cfg.AudiobookshelfURL != "" {
		if err := renderABSPodcastStatus(cfg, cfg.AudiobookshelfURL, "", podcastsDir, quiet); err == nil {
			renderedABS = true
		}
	}

	if !renderedABS {
		renderLocalDiskPodcastStatus(podcastsDir, quiet)
	}
}

func renderABSPodcastStatus(cfg Config, baseURL, token, podcastsDir string, quiet bool) error {
	b, err := getBackend(cfg, quiet)
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
		fmt.Printf("  %-3s  %-6s  %-48s │ %-8s │ %-16s\n", "#", "ID", "Podcast Name", "Episodes", "Needs Ad Removal")
		fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 16))
	}

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for idx, item := range allItems {
		title := item.Media.Metadata.Title
		dName := displayName(title)
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

func buildPodcastMapsForStatus(podcastsDir string) (map[string]tuiPodcast, map[string]string) {
	localPodcasts, _ := loadTUIPodcasts(podcastsDir)
	localByName := make(map[string]tuiPodcast)
	for _, lp := range localPodcasts {
		localByName[strings.ToLower(lp.name)] = lp
		localByName[strings.ToLower(filepath.Base(lp.dir))] = lp
	}

	podEntries := scanPodcastDirs(podcastsDir)
	podIDByDir := make(map[string]string)
	for _, p := range podEntries {
		podIDByDir[p.dir] = p.shortID
		podIDByDir[strings.ToLower(p.title)] = p.shortID
		podIDByDir[strings.ToLower(p.folderName)] = p.shortID
	}
	return localByName, podIDByDir
}

func calculateItemAdRemovalCount(item Podcast, localByName map[string]tuiPodcast, podIDByDir map[string]string) (string, int) {
	title := item.Media.Metadata.Title
	relBase := filepath.Base(item.RelPath)
	lp, ok := localByName[strings.ToLower(title)]
	if !ok {
		lp, ok = localByName[strings.ToLower(relBase)]
	}

	needsAdRemoval := 0
	shortID := ""
	if ok {
		shortID = podIDByDir[lp.dir]
		if shortID == "" {
			shortID = getOrSetPodcastShortID(lp.dir, title)
		}
		mp3Files, _ := filepath.Glob(filepath.Join(lp.dir, "*.mp3"))
		podCfg := loadPodcastConfig(lp.dir)
		if podCfg.AdRemoval != AdRemovalNone {
			filtered := filterMP3FilesByPodcastConfig(mp3Files, lp.dir, podCfg)
			for _, mp3 := range filtered {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
					needsAdRemoval++
				}
			}
		}
	} else {
		shortID = podIDByDir[strings.ToLower(title)]
		if shortID == "" {
			shortID = generatePodcastShortID(title)
		}
	}
	return shortID, needsAdRemoval
}

func renderLocalDiskPodcastStatus(podcastsDir string, quiet bool) {
	podEntries := scanPodcastDirs(podcastsDir)
	var entries []podcastStatusEntry
	for _, pe := range podEntries {
		mp3s := findMP3Files(pe.dir)
		if len(mp3s) == 0 {
			continue
		}
		needsAd := 0
		podCfg := loadPodcastConfig(pe.dir)
		if podCfg.AdRemoval != AdRemovalNone {
			filtered := filterMP3FilesByPodcastConfig(mp3s, pe.dir, podCfg)
			for _, mp3 := range filtered {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
					needsAd++
				}
			}
		}
		entries = append(entries, podcastStatusEntry{
			id:             pe.shortID,
			name:           pe.title,
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
	fmt.Printf("  %-3s  %-6s  %-48s │ %-8s │ %-16s\n", "#", "ID", "Podcast Name", "Episodes", "Needs Ad Removal")
	fmt.Printf("  %-3s  %-6s  %-48s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 3), strings.Repeat("─", 6), strings.Repeat("─", 48), strings.Repeat("─", 8), strings.Repeat("─", 16))

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for idx, e := range entries {
		dName := displayName(e.name)
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
