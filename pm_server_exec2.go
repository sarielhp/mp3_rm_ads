package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleServerCommandPart2(config Config, cli CLIOptions) {
	switch cli.ServerSubcmd {
	case "list":
		handleServerList(config, cli)
	case "download":
		handleServerDownload(config, cli)
	case "keep":
		handleServerKeep(config, cli)
	case "rescan":
		handleServerRescan(config, cli)
	case "timeline":
		handleServerTimeline(config, cli)
	case "get_info", "get-info":
		handleServerGetInfo(config, cli)
	case "frequency", "disable_hourly", "disable-hourly":
		handleServerFrequency(config, cli)
	case "clean-orphans":
		handleServerCleanOrphans(config, cli)
	}
}

func handleServerList(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts: %v", err))
	}
	printPodcastList(client, podcasts, cli.Verbose, cli.Quiet)
}

func handleServerDownload(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts: %v", err))
	}

	totalNewlyDownloaded := 0
	if cli.Podcast != "" {
		targetItem := matchPodcast(podcasts, cli.Podcast)
		if targetItem == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
		}
		dlCount := downloadPodcastEpisodes(client, *targetItem, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, cli.Fill, cli.CountGiven, cli.CheckNew, false, cli.DownloadAll, cli.KeepCount, cli.Verbose, cli.Quiet)
		if !cli.DryRun {
			totalNewlyDownloaded += dlCount
		}
	} else {
		for idx, item := range podcasts {
			title := item.Media.Metadata.Title
			if title == "" {
				title = "Untitled"
			}
			if !cli.Quiet {
				fmt.Printf("\rScanning podcast %d/%d: %s\x1b[K", idx+1, len(podcasts), title)
				os.Stdout.Sync()
			}
			dlCount := downloadPodcastEpisodes(client, item, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, cli.Fill, cli.CountGiven, cli.CheckNew, false, cli.DownloadAll, cli.KeepCount, cli.Verbose, cli.Quiet)
			if !cli.DryRun {
				totalNewlyDownloaded += dlCount
			}
		}
		if !cli.Quiet {
			fmt.Print("\r\x1b[K")
		}
	}

	if totalNewlyDownloaded > 0 && !cli.DryRun {
		if len(config.PostProcessors) > 0 {
			runPostProcessors(config.PostProcessors, cli.Quiet)
		} else {
			processAudioFilesBatch(cli, config, "proc")
		}
	}
}

func handleServerKeep(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts: %v", err))
	}

	if cli.Podcast != "" {
		targetItem := matchPodcast(podcasts, cli.Podcast)
		if targetItem == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
		}
		title := targetItem.Media.Metadata.Title
		if title == "" {
			title = "Untitled"
		}
		applyKeepPolicy(client, targetItem.ID, title, *cli.KeepCount, cli.DryRun, cli.Verbose, cli.Quiet)
	} else {
		for _, item := range podcasts {
			title := item.Media.Metadata.Title
			if title == "" {
				title = "Untitled"
			}
			applyKeepPolicy(client, item.ID, title, *cli.KeepCount, cli.DryRun, cli.Verbose, cli.Quiet)
		}
	}
}

func handleServerRescan(config Config, cli CLIOptions) {
	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts: %v", err))
	}

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

	totalRescanned, totalChecked, podcastCount := executeRescanLoop(client, podcasts, config.PodcastsDir, db, cli)
	if !cli.Quiet {
		fmt.Printf("\nChecked a total of %d MP3 file(s) across %d podcast(s).\n", totalChecked, podcastCount)
		if totalRescanned == 0 {
			fmt.Println("No episodes required database duration updates.")
		}
	}
}

func executeRescanLoop(client *ABSClient, podcasts []PodcastItem, podcastsDir string, db *sql.DB, cli CLIOptions) (int, int, int) {
	totalRescanned := 0
	totalChecked := 0
	if cli.Podcast != "" {
		targetItem := matchPodcast(podcasts, cli.Podcast)
		if targetItem == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
		}
		rCount, cCount := rescanPodcastEpisodes(client, *targetItem, cli.DryRun, db, podcastsDir, cli.Verbose, cli.Quiet)
		return rCount, cCount, 1
	}

	for idx, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = "Untitled"
		}
		if !cli.Quiet {
			fmt.Printf("\rScanning podcast %d/%d: %s\x1b[K", idx+1, len(podcasts), title)
			os.Stdout.Sync()
		}
		rCount, cCount := rescanPodcastEpisodes(client, item, cli.DryRun, db, podcastsDir, cli.Verbose, cli.Quiet)
		totalRescanned += rCount
		totalChecked += cCount
	}
	if !cli.Quiet {
		fmt.Print("\r\x1b[K")
	}
	return totalRescanned, totalChecked, len(podcasts)
}

func handleServerTimeline(config Config, cli CLIOptions) {
	targetDir := "."
	if len(cli.Args) > 0 {
		targetDir = cli.Args[0]
	} else if config.PodcastsDir != "" {
		targetDir = config.PodcastsDir
	}
	podcasts, err := loadTUIPodcasts(targetDir)
	if err != nil || len(podcasts) == 0 {
		podcasts = fallbackTimelinePodcasts(targetDir)
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
}

func fallbackTimelinePodcasts(targetDir string) []tuiPodcast {
	podDir, _ := filepath.Abs(targetDir)
	podName := filepath.Base(podDir)
	pod := tuiPodcast{name: podName, dir: podDir}
	mp3Files, _ := filepath.Glob(filepath.Join(podDir, "*.mp3"))
	for _, mp3 := range mp3Files {
		base := strings.TrimSuffix(mp3, ".mp3")
		hasCut := fileExists(base + ".cuts.json")
		hasTx := fileExists(base+".transcript.json") || fileExists(base+".transcript.txt")
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
		return []tuiPodcast{pod}
	}
	return nil
}
