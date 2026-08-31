package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type podcastStatusEntry struct {
	name           string
	episodes       int
	needsAdRemoval int
}

func absStatus(cfg Config, quiet bool) {
	targetHost := cfg.RemoteHost
	if targetHost == "" {
		targetHost = cfg.RemoteFFmpegHost
	}
	if targetHost != "" && !strings.EqualFold(targetHost, "local") {
		renderRemoteStatusSection(&cfg, targetHost, nil, quiet)
	}

	renderLocalLibraryStatus(cfg, quiet)
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
		fmt.Printf("  %-60s │ %-8s │ %-16s\n", "Podcast Name", "Episodes", "Needs Ad Removal")
		fmt.Printf("  %-60s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 60), strings.Repeat("─", 8), strings.Repeat("─", 16))
	}

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for _, item := range allItems {
		title := item.Media.Metadata.Title
		relBase := filepath.Base(item.RelPath)

		dName := displayName(title)
		if len(dName) > 60 {
			dName = dName[:57] + "..."
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

		if ok {
			mp3Files, _ := filepath.Glob(filepath.Join(lp.dir, "*.mp3"))
			for _, mp3 := range mp3Files {
				if !isEpisodeCompleted(mp3) {
					needsAdRemoval++
				}
			}
		}

		totalEpisodes += absEpisodeCount
		totalNeedsAdRemoval += needsAdRemoval

		if !quiet {
			fmt.Printf("  %-60s │ %-8d │ %-16d\n", dName, absEpisodeCount, needsAdRemoval)
		}
	}

	if !quiet {
		fmt.Printf("  %-60s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 60), strings.Repeat("─", 8), strings.Repeat("─", 16))
		fmt.Printf("  %-60s │ %-8d │ %-16d\n", "TOTAL", totalEpisodes, totalNeedsAdRemoval)
		fmt.Printf("%s\n\n", strings.Repeat("=", 90))
	}

	return nil
}

func renderLocalDiskPodcastStatus(podcastsDir string, quiet bool) {
	var entries []podcastStatusEntry
	dirEntries, err := os.ReadDir(podcastsDir)
	if err == nil {
		for _, de := range dirEntries {
			if !de.IsDir() || strings.HasPrefix(de.Name(), ".") {
				continue
			}
			podPath := filepath.Join(podcastsDir, de.Name())
			mp3s := findMP3Files(podPath)
			if len(mp3s) == 0 {
				continue
			}
			needsAd := 0
			for _, mp3 := range mp3s {
				if !isEpisodeCompleted(mp3) {
					needsAd++
				}
			}
			entries = append(entries, podcastStatusEntry{
				name:           de.Name(),
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
				if !isEpisodeCompleted(mp3) {
					needsAd++
				}
			}
			entries = append(entries, podcastStatusEntry{
				name:           filepath.Base(podcastsDir),
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
	fmt.Printf("  %-60s │ %-8s │ %-16s\n", "Podcast Name", "Episodes", "Needs Ad Removal")
	fmt.Printf("  %-60s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 60), strings.Repeat("─", 8), strings.Repeat("─", 16))

	totalEpisodes := 0
	totalNeedsAdRemoval := 0

	for _, e := range entries {
		dName := displayName(e.name)
		if len(dName) > 60 {
			dName = dName[:57] + "..."
		}
		totalEpisodes += e.episodes
		totalNeedsAdRemoval += e.needsAdRemoval
		fmt.Printf("  %-60s │ %-8d │ %-16d\n", dName, e.episodes, e.needsAdRemoval)
	}

	fmt.Printf("  %-60s ┼ %-8s ┼ %-16s\n", strings.Repeat("─", 60), strings.Repeat("─", 8), strings.Repeat("─", 16))
	fmt.Printf("  %-60s │ %-8d │ %-16d\n", "TOTAL", totalEpisodes, totalNeedsAdRemoval)
	fmt.Printf("%s\n\n", strings.Repeat("=", 90))
}
