package main

import (
	"fmt"
	"os"
	"strings"
)

func handleServerCommand(config Config, cli CLIOptions) {
	switch cli.ServerSubcmd {
	case "opml":
		handleServerOPML(config, cli)
	case "scan":
		handleServerScan(config, cli)
	default:
		handleServerCommandPart2(config, cli)
	}
}

func handleServerOPML(config Config, cli CLIOptions) {
	targetFile := cli.OPMLFile
	if targetFile == "" && cli.Output != "" {
		targetFile = cli.Output
	}
	if targetFile == "" && len(cli.Args) > 0 {
		targetFile = cli.Args[0]
	}

	switch cli.OPMLSubcmd {
	case "import":
		if targetFile == "" {
			showOPMLImportUsage()
			fatalError("%s\n", "Error: missing required <file> argument for 'abs opml import <file>'.")
		}
		importOPML(config, targetFile, cli.Quiet, cli.Verbose)
	case "export":
		if targetFile == "" {
			showOPMLExportUsage()
			fatalError("%s\n", "Error: missing required <file> argument for 'abs opml export <file>'.")
		}
		exportOPML(config, targetFile, cli.Quiet, cli.Verbose)
	default:
		showOPMLUsage()
		fatalError("%s\n", "Error: must specify 'import <file>' or 'export <file>'.")
	}
}

func handleServerScan(config Config, cli CLIOptions) {
	targetDir := config.PodcastsDir
	if len(cli.Args) > 0 {
		targetDir = cli.Args[0]
	}
	if targetDir == "" {
		fatalError("%s\n", "ERROR: podcasts_dir is not configured. Specify it as an argument or in config.")
	}
	config.PodcastsDir = targetDir

	if !cli.EpisodesOnly {
		absScanPodcasts(config, cli.Quiet)
	}

	if cli.PodcastsOnly {
		return
	}

	client, err := getABSClient(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	podcasts, err := client.PodcastItems()
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Failed to fetch podcasts: %v", err))
	}

	var activePodcasts []PodcastItem
	for _, p := range podcasts {
		if strings.TrimSpace(p.Media.Metadata.FeedURL) != "" {
			activePodcasts = append(activePodcasts, p)
		}
	}
	if len(activePodcasts) > 0 {
		podcasts = activePodcasts
	}

	totalNewlyDownloaded := 0
	totalPodcastsChecked := 0

	if cli.Podcast != "" {
		targetItem := matchPodcast(podcasts, cli.Podcast)
		if targetItem == nil {
			fatalError("%s\n", fmt.Sprintf("Podcast matching %s not found.", cli.Podcast))
		}
		dlCount := downloadPodcastEpisodes(client, *targetItem, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, false, cli.CountGiven, true, true, cli.DownloadAll, cli.KeepCount, cli.Verbose, cli.Quiet)
		if !cli.DryRun {
			totalNewlyDownloaded += dlCount
		}
		totalPodcastsChecked = 1
	} else {
		totalPodcastsChecked = len(podcasts)
		totalNewlyDownloaded = scanAllPodcastsForNewEpisodes(client, podcasts, cli)
	}

	if !cli.Quiet {
		fmt.Printf("\nChecked a total of %d podcast(s) for new episodes.\n", totalPodcastsChecked)
		if totalNewlyDownloaded == 0 {
			fmt.Println("No new episodes found.")
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

func scanAllPodcastsForNewEpisodes(client *ABSClient, podcasts []PodcastItem, cli CLIOptions) int {
	type scanResult struct {
		count int
	}
	resChan := make(chan scanResult, len(podcasts))
	sem := make(chan struct{}, 10)
	var completedCount int
	var countMu syncMutex

	for _, item := range podcasts {
		go func(it PodcastItem) {
			sem <- struct{}{}
			defer func() { <-sem }()

			dlCount := downloadPodcastEpisodes(client, it, cli.Count, cli.Oldest, cli.DryRun, true, false, cli.CountGiven, true, true, cli.DownloadAll, cli.KeepCount, cli.Verbose, cli.Quiet)

			countMu.Lock()
			completedCount++
			done := completedCount
			countMu.Unlock()

			if !cli.Quiet {
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

	totalNewlyDownloaded := 0
	for i := 0; i < len(podcasts); i++ {
		res := <-resChan
		if !cli.DryRun {
			totalNewlyDownloaded += res.count
		}
	}
	if !cli.Quiet {
		fmt.Print("\r\x1b[K")
	}

	if totalNewlyDownloaded > 0 && !cli.NoWait && !cli.DryRun {
		if !cli.Quiet {
			fmt.Printf("\nWaiting for Audiobookshelf to complete %d queued download(s)...\n", totalNewlyDownloaded)
		}
		waitForActiveDownloads(client, podcasts, cli.Quiet)
	}
	return totalNewlyDownloaded
}
