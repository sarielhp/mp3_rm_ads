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
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") || de.Name() == ".work" {
				continue
			}
			podPath := filepath.Join(podcastsDir, de.Name())
			mp3s := findMP3Files(podPath)
			if len(mp3s) == 0 {
				continue
			}
			podcastsCount++
			totalEpisodes += len(mp3s)
			for _, mp3 := range mp3s {
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
	if cfg.AudiobookshelfURL != "" {
		token, err := absLogin(cfg)
		if err == nil {
			baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")
			if err := renderABSPodcastStatus(cfg, baseURL, token, podcastsDir, quiet); err == nil {
				renderedABS = true
			}
		}
	}

	if !renderedABS {
		renderLocalDiskPodcastStatus(podcastsDir, quiet)
	}
}

func renderABSPodcastStatus(cfg Config, baseURL, token, podcastsDir string, quiet bool) error {
	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		return err
	}

	var allItems []absItem
	for _, lib := range libsResp.Libraries {
		if lib.MediaType != "podcast" {
			continue
		}
		var itemsResp absItemsResp
		endpoint := fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID)
		if err := absGet(baseURL, token, endpoint, &itemsResp); err == nil {
			allItems = append(allItems, itemsResp.Results...)
		}
	}

	if len(allItems) == 0 {
		return fmt.Errorf("no podcast libraries found in ABS")
	}

	localPodcasts, _ := loadTUIPodcasts(podcastsDir)
	localByName := make(map[string]tuiPodcast)
	for _, lp := range localPodcasts {
		localByName[strings.ToLower(lp.name)] = lp
		localByName[strings.ToLower(filepath.Base(lp.dir))] = lp
	}

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
		relBase := filepath.Base(item.RelPath)

		dName := displayName(title)
		if len(dName) > 48 {
			dName = dName[:45] + "..."
		}

		var itemFull absItem
		absEpisodeCount := 0
		if err := absGet(baseURL, token, "/api/items/"+item.ID, &itemFull); err == nil {
			absEpisodeCount = len(itemFull.Media.Episodes)
		}

		needsAdRemoval := 0
		lp, ok := localByName[strings.ToLower(title)]
		if !ok {
			lp, ok = localByName[strings.ToLower(relBase)]
		}

		shortID := ""
		if ok {
			shortID = getOrSetPodcastShortID(lp.dir, title)
			mp3Files, _ := filepath.Glob(filepath.Join(lp.dir, "*.mp3"))
			for _, mp3 := range mp3Files {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
					needsAdRemoval++
				}
			}
		} else {
			shortID = generatePodcastShortID(title)
		}

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

func renderLocalDiskPodcastStatus(podcastsDir string, quiet bool) {
	var entries []podcastStatusEntry
	dirEntries, err := os.ReadDir(podcastsDir)
	if err == nil {
		for _, de := range dirEntries {
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") || de.Name() == ".work" {
				continue
			}
			podPath := filepath.Join(podcastsDir, de.Name())
			mp3s := findMP3Files(podPath)
			if len(mp3s) == 0 {
				continue
			}
			needsAd := 0
			for _, mp3 := range mp3s {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
					needsAd++
				}
			}
			title := de.Name()
			if cached, _ := loadPodcastCache(podPath); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
				title = strings.TrimSpace(cached.PodcastName)
			}
			shortID := getOrSetPodcastShortID(podPath, title)
			entries = append(entries, podcastStatusEntry{
				id:             shortID,
				name:           title,
				episodes:       len(mp3s),
				needsAdRemoval: needsAd,
			})
		}
	}

	if len(entries) == 0 {
		mp3s := findMP3Files(podcastsDir)
		if len(mp3s) > 0 {
			needsAd := 0
			for _, mp3 := range mp3s {
				_ = getOrCreateEpisodeStatus(mp3)
				if !isEpisodeCompleted(mp3) {
					needsAd++
				}
			}
			title := filepath.Base(podcastsDir)
			if cached, _ := loadPodcastCache(podcastsDir); cached != nil && strings.TrimSpace(cached.PodcastName) != "" {
				title = strings.TrimSpace(cached.PodcastName)
			}
			shortID := getOrSetPodcastShortID(podcastsDir, title)
			entries = append(entries, podcastStatusEntry{
				id:             shortID,
				name:           title,
				episodes:       len(mp3s),
				needsAdRemoval: needsAd,
			})
		}
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
