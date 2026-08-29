package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func handleServerCommand(config Config, cli CLIOptions) {
	switch cli.ServerSubcmd {
	case "opml":
		exportOPML(config, cli)
		return

	case "scan":
		targetDir := config.PodcastsDir
		if len(cli.Args) > 0 {
			targetDir = cli.Args[0]
		}
		if targetDir == "" {
			fmt.Println("ERROR: podcasts_dir is not configured. Specify it as an argument or in config.")
			os.Exit(1)
		}
		config.PodcastsDir = targetDir

		if !cli.EpisodesOnly {
			absScanPodcasts(config, cli.Quiet)
		}

		if !cli.PodcastsOnly {
			client, err := getABSClient(config, cli.Silent)
			if err != nil {
				printError(fmt.Sprintf("Error: %v", err))
				os.Exit(1)
			}
			podcasts, err := client.PodcastItems()
			if err != nil {
				printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
				os.Exit(1)
			}

			totalNewlyDownloaded := 0
			totalPodcastsChecked := 0

			if cli.Podcast != "" {
				targetItem := matchPodcast(podcasts, cli.Podcast)
				if targetItem == nil {
					printError(fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
					os.Exit(1)
				}
				dlCount := downloadPodcastEpisodes(client, *targetItem, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, false, cli.CountGiven, true, true, cli.KeepCount, cli.Verbose, cli.Silent)
				if !cli.DryRun {
					totalNewlyDownloaded += dlCount
				}
				totalPodcastsChecked = 1
			} else {
				totalPodcastsChecked = len(podcasts)
				type scanResult struct {
					count int
				}
				resChan := make(chan scanResult, len(podcasts))
				sem := make(chan struct{}, 10)
				var completedCount int
				var countMu sync.Mutex

				for _, item := range podcasts {
					go func(it PodcastItem) {
						sem <- struct{}{}
						defer func() { <-sem }()

						dlCount := downloadPodcastEpisodes(client, it, cli.Count, cli.Oldest, cli.DryRun, true, false, cli.CountGiven, true, true, cli.KeepCount, cli.Verbose, cli.Silent)

						countMu.Lock()
						completedCount++
						done := completedCount
						countMu.Unlock()

						if !cli.Silent {
							title := it.Media.Metadata.Title
							if title == "" {
								title = "Untitled"
							}
							fmt.Printf("\rScanning podcasts (%d/%d): %s\x1b[K", done, len(podcasts), title)
							os.Stdout.Sync()
						}
						resChan <- scanResult{count: dlCount}
					}(item)
				}

				for i := 0; i < len(podcasts); i++ {
					res := <-resChan
					if !cli.DryRun {
						totalNewlyDownloaded += res.count
					}
				}
				if !cli.Silent {
					fmt.Print("\r\x1b[K")
				}

				if totalNewlyDownloaded > 0 && !cli.NoWait && !cli.DryRun {
					if !cli.Silent {
						fmt.Printf("\nWaiting for Audiobookshelf to complete %d queued download(s)...\n", totalNewlyDownloaded)
					}
					waitForActiveDownloads(client, podcasts, cli.Silent)
				}
			}

			if !cli.Silent {
				fmt.Printf("\nChecked a total of %d podcast(s) for new episodes.\n", totalPodcastsChecked)
				if totalNewlyDownloaded == 0 {
					fmt.Println("No new episodes found.")
				}
			}

			if totalNewlyDownloaded > 0 && len(config.PostProcessors) > 0 {
				runPostProcessors(config.PostProcessors, cli.Silent)
			}
		}
		return

	case "list":
		client, err := getABSClient(config, cli.Silent)
		if err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(1)
		}
		podcasts, err := client.PodcastItems()
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
			os.Exit(1)
		}
		printPodcastList(client, podcasts, cli.Verbose, cli.Silent)
		return

	case "download":
		client, err := getABSClient(config, cli.Silent)
		if err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(1)
		}
		podcasts, err := client.PodcastItems()
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
			os.Exit(1)
		}

		totalNewlyDownloaded := 0
		if cli.Podcast != "" {
			targetItem := matchPodcast(podcasts, cli.Podcast)
			if targetItem == nil {
				printError(fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
				os.Exit(1)
			}
			dlCount := downloadPodcastEpisodes(client, *targetItem, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, cli.Fill, cli.CountGiven, cli.CheckNew, false, cli.KeepCount, cli.Verbose, cli.Silent)
			if !cli.DryRun {
				totalNewlyDownloaded += dlCount
			}
		} else {
			for idx, item := range podcasts {
				title := item.Media.Metadata.Title
				if title == "" {
					title = "Untitled"
				}
				if !cli.Silent {
					fmt.Printf("\rScanning podcast %d/%d: %s\x1b[K", idx+1, len(podcasts), title)
					os.Stdout.Sync()
				}
				dlCount := downloadPodcastEpisodes(client, item, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, cli.Fill, cli.CountGiven, cli.CheckNew, false, cli.KeepCount, cli.Verbose, cli.Silent)
				if !cli.DryRun {
					totalNewlyDownloaded += dlCount
				}
			}
			if !cli.Silent {
				fmt.Print("\r\x1b[K")
			}
		}

		if totalNewlyDownloaded > 0 && len(config.PostProcessors) > 0 {
			runPostProcessors(config.PostProcessors, cli.Silent)
		}
		return

	case "keep":
		client, err := getABSClient(config, cli.Silent)
		if err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(1)
		}
		podcasts, err := client.PodcastItems()
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
			os.Exit(1)
		}

		if cli.Podcast != "" {
			targetItem := matchPodcast(podcasts, cli.Podcast)
			if targetItem == nil {
				printError(fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
				os.Exit(1)
			}
			title := targetItem.Media.Metadata.Title
			if title == "" {
				title = "Untitled"
			}
			applyKeepPolicy(client, targetItem.ID, title, *cli.KeepCount, cli.DryRun, cli.Verbose, cli.Silent)
		} else {
			for _, item := range podcasts {
				title := item.Media.Metadata.Title
				if title == "" {
					title = "Untitled"
				}
				applyKeepPolicy(client, item.ID, title, *cli.KeepCount, cli.DryRun, cli.Verbose, cli.Silent)
			}
		}
		return

	case "rescan":
		client, err := getABSClient(config, cli.Silent)
		if err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(1)
		}
		podcasts, err := client.PodcastItems()
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
			os.Exit(1)
		}

		totalRescanned := 0
		totalChecked := 0
		podcastCount := 0

		dbPath := cli.SqliteDBPath
		if dbPath == "" {
			dbPath = config.AudiobookshelfDBPath
		}

		var db *sql.DB
		if !cli.DryRun && dbPath != "" && fileExists(dbPath) {
			var err error
			db, err = sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
			if err == nil {
				defer db.Close()
			}
		}

		if cli.Podcast != "" {
			targetItem := matchPodcast(podcasts, cli.Podcast)
			if targetItem == nil {
				printError(fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
				os.Exit(1)
			}
			rCount, cCount := rescanPodcastEpisodes(client, *targetItem, cli.DryRun, db, config.PodcastsDir, cli.Verbose, cli.Silent)
			totalRescanned += rCount
			totalChecked += cCount
			podcastCount = 1
		} else {
			podcastCount = len(podcasts)
			for idx, item := range podcasts {
				title := item.Media.Metadata.Title
				if title == "" {
					title = "Untitled"
				}
				if !cli.Silent {
					fmt.Printf("\rScanning podcast %d/%d: %s\x1b[K", idx+1, len(podcasts), title)
					os.Stdout.Sync()
				}
				rCount, cCount := rescanPodcastEpisodes(client, item, cli.DryRun, db, config.PodcastsDir, cli.Verbose, cli.Silent)
				totalRescanned += rCount
				totalChecked += cCount
			}
			if !cli.Silent {
				fmt.Print("\r\x1b[K")
			}
		}

		if !cli.Silent {
			fmt.Printf("\nChecked a total of %d MP3 file(s) across %d podcast(s).\n", totalChecked, podcastCount)
			if totalRescanned == 0 {
				fmt.Println("No episodes required database duration updates.")
			}
		}
		return

	case "timeline":
		targetDir := "."
		if len(cli.Args) > 0 {
			targetDir = cli.Args[0]
		} else if config.PodcastsDir != "" {
			targetDir = config.PodcastsDir
		}
		podcasts, err := loadTUIPodcasts(targetDir)
		if err != nil || len(podcasts) == 0 {
			podDir, _ := filepath.Abs(targetDir)
			podName := filepath.Base(podDir)
			pod := tuiPodcast{name: podName, dir: podDir}
			mp3Files, _ := filepath.Glob(filepath.Join(podDir, "*.mp3"))
			for _, mp3 := range mp3Files {
				base := strings.TrimSuffix(mp3, ".mp3")
				hasCut := false
				if _, err := os.Stat(base + ".cuts.json"); err == nil {
					hasCut = true
				}
				hasTx := false
				if _, err := os.Stat(base + ".transcript.json"); err == nil {
					hasTx = true
				} else if _, err := os.Stat(base + ".transcript.txt"); err == nil {
					hasTx = true
				}
				var fSize int64
				var modTime time.Time
				if fi, err := os.Stat(mp3); err == nil {
					fSize = fi.Size()
					modTime = fi.ModTime()
				}
				pod.episodes = append(pod.episodes, tuiEpisode{
					filename:      filepath.Base(mp3),
					path:          mp3,
					hasAdsRemoved: hasCut,
					hasTranscript: hasTx,
					fileSize:      fSize,
					modTime:       modTime,
				})
			}
			if len(pod.episodes) > 0 {
				podcasts = []tuiPodcast{pod}
			}
		}
		if len(podcasts) == 0 {
			if !cli.Quiet {
				fmt.Printf("No podcast episodes found in '%s'.\n", targetDir)
			}
			return
		}
		for _, pod := range podcasts {
			releases := getPodcastLastEpisodesOnlineTimeline(pod, 20)
			fmt.Print(formatEpisodesTimelineTable(releases, pod.name, 100))
		}
		return
	}
}

func waitForActiveDownloads(client *ABSClient, podcasts []PodcastItem, silent bool) {
	startTime := time.Now()
	for {
		hasActive := false
		var activeTitles []string
		for _, p := range podcasts {
			dls, err := client.ActiveDownloads(p.ID)
			if err == nil && len(dls) > 0 {
				hasActive = true
				for _, d := range dls {
					if d.EpisodeDisplayTitle != "" {
						activeTitles = append(activeTitles, d.EpisodeDisplayTitle)
					}
				}
			}
		}
		if !hasActive {
			if !silent {
				fmt.Println("\nAll downloads completed successfully!")
			}
			break
		}
		if !silent && len(activeTitles) > 0 {
			fmt.Printf("\r    Downloading (%d active): %s\x1b[K", len(activeTitles), activeTitles[0])
			os.Stdout.Sync()
		}
		time.Sleep(3 * time.Second)
		if time.Since(startTime) > 300*time.Second {
			if !silent {
				fmt.Println("\nTimeout waiting for downloads to finish. Downloads continue in background.")
			}
			break
		}
	}
}
