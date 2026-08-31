package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func absMapPodcasts(cfg Config, quiet bool) {
	if cfg.AudiobookshelfURL == "" {
		fmt.Println("ERROR: audiobookshelf_url is not configured.")
		return
	}
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		fmt.Println("ERROR: podcasts_dir is not configured.")
		return
	}

	token, err := absLogin(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")

	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get libraries: %v\n", err)
		return
	}

	for _, lib := range libsResp.Libraries {
		if lib.MediaType != "podcast" {
			continue
		}

		var itemsResp absItemsResp
		endpoint := fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID)
		if err := absGet(baseURL, token, endpoint, &itemsResp); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to get items for library '%s': %v\n", lib.Name, err)
			continue
		}

		for _, item := range itemsResp.Results {
			item := item
			podDir := filepath.Join(podcastsDir, item.RelPath)
			podEntries, err := os.ReadDir(podDir)
			if err != nil {
				continue
			}

			fmt.Printf("\n%s %s\n", displayName(lib.Name+"/"+item.RelPath), tuiDimStyle.Render(fmt.Sprintf("(%s)", item.Media.Metadata.Title)))

			var itemFull absItem
			if err := absGet(baseURL, token, "/api/items/"+item.ID, &itemFull); err != nil {
				if !quiet {
					fmt.Printf("  ERROR: Failed to fetch item details: %v\n", err)
				}
				continue
			}

			epByAudioFile := make(map[string]absEpisode)
			for _, ep := range itemFull.Media.Episodes {
				if ep.AudioFile != nil {
					epByAudioFile[ep.AudioFile.Metadata.Filename] = ep
				}
			}

			for _, entry := range podEntries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
					continue
				}

				mp3Path := filepath.Join(podDir, entry.Name())
				hasCut := false
				if _, err := os.Stat(strings.TrimSuffix(mp3Path, ".mp3") + ".cuts.json"); err == nil {
					hasCut = true
				}

				matched, ok := epByAudioFile[entry.Name()]
				if !ok {
					if !quiet {
						fmt.Printf("  ? %s\n", displayName(entry.Name()))
					}
					continue
				}

				summary := fmt.Sprintf("  %s %s\n", greenCheck, displayName(entry.Name()))
				if matched.Title != "" && matched.Title != strings.TrimSuffix(entry.Name(), ".mp3") {
					summary += fmt.Sprintf("    Title:       %s\n", displayName(matched.Title))
				}
				if matched.Description != "" {
					desc := stripHTML(matched.Description)
					if len(desc) > 120 {
						desc = desc[:120] + "..."
					}
					summary += fmt.Sprintf("    Description: %s\n", desc)
				}
				if matched.PubDate != "" {
					summary += fmt.Sprintf("    Published:   %s\n", matched.PubDate)
				}
				if matched.Duration > 0 {
					summary += fmt.Sprintf("    Duration:    %s\n", formatDurationShort(matched.Duration))
				}
				if hasCut {
					summary += fmt.Sprintf("    Ads:         %s Removed\n", greenCheck)
				}
				fmt.Print(summary)
			}
		}
	}
}

func absScanPodcasts(cfg Config, quiet bool) {
	if cfg.AudiobookshelfURL == "" {
		fmt.Println("ERROR: audiobookshelf_url is not configured.")
		return
	}

	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		fmt.Println("ERROR: podcasts_dir is not configured.")
		return
	}

	token, err := absLogin(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")

	localPodcasts, err := loadTUIPodcasts(podcastsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load local podcasts: %v\n", err)
		return
	}

	existing := make(map[string]bool)
	if dirEntries, err := os.ReadDir(podcastsDir); err == nil {
		for _, e := range dirEntries {
			if e.IsDir() {
				existing[strings.ToLower(e.Name())] = true
			}
		}
	}
	for _, lp := range localPodcasts {
		existing[strings.ToLower(lp.name)] = true
		existing[strings.ToLower(filepath.Base(lp.dir))] = true
	}

	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get libraries: %v\n", err)
		return
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

	if !quiet {
		fmt.Printf("Connected to Audiobookshelf. Found %d podcasts in database.\n", len(allItems))
		fmt.Println("Scanning for new podcasts not present locally...")
	}

	newCount := 0

	for _, item := range allItems {
		title := item.Media.Metadata.Title
		relBase := filepath.Base(item.RelPath)

		isNew := true
		if existing[strings.ToLower(title)] || existing[strings.ToLower(relBase)] {
			isNew = false
		}

		if isNew {
			newCount++
			if !quiet {
				fmt.Printf("\n[+] Found new podcast: '%s' (RelPath: %s)\n", title, item.RelPath)
			}

			safeName := strings.Map(func(r rune) rune {
				if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
					return '_'
				}
				return r
			}, title)
			safeName = strings.TrimSpace(safeName)
			if safeName == "" {
				safeName = strings.Map(func(r rune) rune {
					if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
						return '_'
					}
					return r
				}, relBase)
				safeName = strings.TrimSpace(safeName)
			}
			if safeName == "" {
				safeName = "podcast_" + item.ID
			}

			podDir := filepath.Join(podcastsDir, safeName)
			if err := os.MkdirAll(podDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR: Failed to create directory '%s': %v\n", podDir, err)
				continue
			}

			var itemFull absItem
			if err := absGet(baseURL, token, "/api/items/"+item.ID, &itemFull); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR: Failed to fetch podcast details from ABS: %v\n", err)
				continue
			}

			pod := tuiPodcast{
				name:   safeName,
				dir:    podDir,
				config: loadPodcastConfig(podDir),
			}
			pod.absData = &itemFull
			if itemFull.Media.Metadata.Author != "" {
				pod.author = itemFull.Media.Metadata.Author
			}
			if itemFull.Media.Metadata.Description != "" {
				pod.description = itemFull.Media.Metadata.Description
			}
			if itemFull.Media.Metadata.FeedURL != "" {
				pod.feedURL = itemFull.Media.Metadata.FeedURL
			}

			cDir := cacheDirForPodcast(pod.dir)
			coverDest := filepath.Join(cDir, "cover.jpg")
			if err := absDownloadCover(baseURL, token, itemFull.ID, coverDest); err != nil {
				if !quiet {
					fmt.Printf("  Warning: Failed to download cover image: %v\n", err)
				}
			} else {
				if !quiet {
					fmt.Printf("  ✓ Downloaded cover to %s\n", coverDest)
				}
			}

			savePodcastToCache(&pod)
			if !quiet {
				fmt.Printf("  ✓ Saved metadata cache to details/index.json\n")
			}
		}
	}

	if !quiet {
		if newCount == 0 {
			fmt.Println("No new podcasts found. Local library is up to date.")
		} else {
			fmt.Printf("\nScan complete. Added and initialized %d new podcast(s).\n", newCount)
		}
	}
}
