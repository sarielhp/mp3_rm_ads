package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sarielhp/clihelp"
)

func main() {
	defer func() {
		globalPlayer.Stop()
		if r := recover(); r != nil {
			globalPlayer.Stop()
			panic(r)
		}
	}()

	action, cli := parseFlags()

	var silentLogFile *os.File
	var silentLogPath string
	var origStdout *os.File
	var origStderr *os.File
	hasError := false

	if cli.Silent {
		origStdout = os.Stdout
		origStderr = os.Stderr

		origStdout.Sync()
		origStderr.Sync()

		tmpDir := userTmpDir()
		tmp, err := os.CreateTemp(tmpDir, "abs_silent_*.log")
		if err == nil {
			silentLogFile = tmp
			silentLogPath = tmp.Name()
			os.Stdout = silentLogFile
			os.Stderr = silentLogFile
		}
	}

	finishSilent := func(hadError bool) {
		if silentLogFile == nil {
			os.Stdout.Sync()
			os.Stderr.Sync()
			return
		}
		os.Stdout.Sync()
		os.Stderr.Sync()
		silentLogFile.Sync()

		os.Stdout = origStdout
		os.Stderr = origStderr
		silentLogFile.Close()

		if hadError {
			data, readErr := os.ReadFile(silentLogPath)
			if readErr == nil {
				origStderr.Write(data)
				origStderr.Sync()
			}
		}
		os.Remove(silentLogPath)
		silentLogFile = nil
	}

	defer func() {
		if r := recover(); r != nil {
			finishSilent(true)
			panic(r)
		} else {
			finishSilent(hasError)
		}
		if !cli.Quiet {
			fmt.Println()
		}
	}()

	ensureConfigExists()
	config := loadConfig()

	switch action {
	case "test":
		if cli.TestABSMap {
			absMapPodcasts(config, cli.Quiet)
		} else if cli.TestABSDownload {
			absDownloadAllData(config, cli.Quiet)
		} else if cli.TestKitty {
			testKittyImage(cli.Args)
		} else if cli.TestABS {
			if !testAudiobookshelfServer(config, cli.Quiet) {
				hasError = true
				os.Exit(1)
			}
		} else {
			if !testWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet) {
				hasError = true
				os.Exit(1)
			}
		}
		return

	case "config":
		if cli.PodcastsDir != "" {
			setPodcastsDir(&config, cli.PodcastsDir)
		}
		if cli.ABSURL != "" || cli.ABSUser != "" || cli.ABSPass != "" {
			setAudiobookshelf(&config, cli.ABSURL, cli.ABSUser, cli.ABSPass)
		}
		if cli.ABSToken != "" {
			config.AudiobookshelfToken = cli.ABSToken
			saveConfig(config)
			if !cli.Silent {
				fmt.Println("Saved Audiobookshelf API token to config.")
			}
		}
		if cli.SqliteDBPath != "" {
			config.AudiobookshelfDBPath = cli.SqliteDBPath
			saveConfig(config)
			if !cli.Silent {
				fmt.Printf("Saved SQLite DB path to config: %s\n", cli.SqliteDBPath)
			}
		}
		if cli.ProcessorCmd != "" {
			handleConfigProcessor(&config, cli.ProcessorCmd, cli.ProcessorValue)
		} else if cli.ConfigInfo {
			printConfigInfo(config)
		} else if cli.ListLLMs {
			listProfiles(config)
		} else if cli.ListWhispers {
			listWhispers(config)
		} else if cli.SetDefault > 0 {
			setDefaultProfile(&config, cli.SetDefault)
		} else if cli.SetDefaultWhisper > 0 {
			setDefaultWhisperProfile(&config, cli.SetDefaultWhisper)
		} else if cli.AddWhisper != "" {
			addWhisperProfile(&config, cli.AddWhisper)
		} else if cli.RemoveWhisper > 0 {
			removeWhisperProfile(&config, cli.RemoveWhisper)
		} else if cli.CopyOpenCode {
			copyLLMFromOpenCode(&config)
		} else {
			printConfig(config)
		}
		return

	case "cache":
		if err := resetCache(); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting cache: %v\n", err)
			os.Exit(1)
		}
		if !cli.Quiet {
			fmt.Println("Cache reset successfully.")
		}
		return

	case "status":
		targetDir := config.PodcastsDir
		if len(cli.Args) > 0 {
			targetDir = cli.Args[0]
		}
		if targetDir == "" {
			fmt.Println("ERROR: podcasts_dir is not configured. Specify it as an argument or in config.")
			os.Exit(1)
		}
		config.PodcastsDir = targetDir
		absStatus(config, cli.Quiet)
		return

	case "tui":
		dir := config.PodcastsDir
		if len(cli.Args) > 0 {
			dir = cli.Args[0]
		}
		if dir == "" {
			fmt.Println("ERROR: podcasts_dir is not configured. Specify it as an argument or in config.")
			os.Exit(1)
		}
		bk := &TuiBackend{
			LoadPodcasts: func(dir string) ([]tuiPodcast, error) {
				return loadTUIPodcastsABS(dir, config)
			},
			LoadQueues:  loadAllQueues,
			SaveQueue:   saveQueue,
			GetDuration: getAudioDuration,
		}
		p := tea.NewProgram(newTuiModel(bk, dir, &config), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
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

	case "server":
		switch cli.ServerSubcmd {
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
				token := config.AudiobookshelfToken
				if token == "" {
					token = getABSTokenFromDB(config.AudiobookshelfDBPath)
				}
				if token == "" {
					token, _ = absLogin(config)
				}
				if token == "" {
					printError("Error: Audiobookshelf API token not configured.")
					os.Exit(1)
				}
				client := NewABSClient(config.AudiobookshelfURL, token)
				client.Silent = cli.Silent
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
					for idx, item := range podcasts {
						title := item.Media.Metadata.Title
						if title == "" {
							title = "Untitled"
						}
						if !cli.Silent {
							fmt.Printf("\rScanning podcast %d/%d: %s\x1b[K", idx+1, len(podcasts), title)
							os.Stdout.Sync()
						}
						dlCount := downloadPodcastEpisodes(client, item, cli.Count, cli.Oldest, cli.DryRun, cli.NoWait, false, cli.CountGiven, true, true, cli.KeepCount, cli.Verbose, cli.Silent)
						if !cli.DryRun {
							totalNewlyDownloaded += dlCount
						}
					}
					if !cli.Silent {
						fmt.Print("\r\x1b[K")
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
			token := config.AudiobookshelfToken
			if token == "" {
				token = getABSTokenFromDB(config.AudiobookshelfDBPath)
			}
			if token == "" {
				token, _ = absLogin(config)
			}
			if token == "" {
				printError("Error: Audiobookshelf API token not configured and could not be retrieved from DB or login.")
				os.Exit(1)
			}
			client := NewABSClient(config.AudiobookshelfURL, token)
			client.Silent = cli.Silent
			podcasts, err := client.PodcastItems()
			if err != nil {
				printError(fmt.Sprintf("Failed to fetch podcasts: %v", err))
				os.Exit(1)
			}
			printPodcastList(client, podcasts, cli.Verbose, cli.Silent)
			return

		case "download":
			token := config.AudiobookshelfToken
			if token == "" {
				token = getABSTokenFromDB(config.AudiobookshelfDBPath)
			}
			if token == "" {
				token, _ = absLogin(config)
			}
			if token == "" {
				printError("Error: Audiobookshelf API token not configured.")
				os.Exit(1)
			}
			client := NewABSClient(config.AudiobookshelfURL, token)
			client.Silent = cli.Silent
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
			token := config.AudiobookshelfToken
			if token == "" {
				token = getABSTokenFromDB(config.AudiobookshelfDBPath)
			}
			if token == "" {
				token, _ = absLogin(config)
			}
			if token == "" {
				printError("Error: Audiobookshelf API token not configured.")
				os.Exit(1)
			}
			client := NewABSClient(config.AudiobookshelfURL, token)
			client.Silent = cli.Silent
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
			token := config.AudiobookshelfToken
			if token == "" {
				token = getABSTokenFromDB(config.AudiobookshelfDBPath)
			}
			if token == "" {
				token, _ = absLogin(config)
			}
			if token == "" {
				printError("Error: Audiobookshelf API token not configured.")
				os.Exit(1)
			}
			client := NewABSClient(config.AudiobookshelfURL, token)
			client.Silent = cli.Silent
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
		}

	case "file", "dir":
		// Fall through
	}

	go wakeWhisperServer(config.WhisperURL, config.WhisperWakeCommand, cli.Quiet)

	args := cli.Args
	var expandedArgs []string

	hasPrintedScanning := false
	printScanning := func(dir string) {
		if cli.Quiet {
			return
		}
		if !hasPrintedScanning {
			fmt.Println()
			hasPrintedScanning = true
		}
		fmt.Printf("Scanning: %s\n", dir)
	}

	if action == "dir" {
		if len(args) == 0 {
			app := buildCLIApp(&action, &cli)
			app.RenderCommand(clihelp.Options{}, "dir")
			os.Exit(1)
		}
		dir := args[0]
		printScanning(dir)
		removeWorkDirs(dir)
		rawMp3Files := findMP3Files(dir)
		podCfg := loadPodcastConfig(dir)
		mp3Files := filterMP3FilesByPodcastConfig(rawMp3Files, dir, podCfg)
		if len(rawMp3Files) == 0 {
			if !cli.Quiet {
				fmt.Printf("No MP3 files found in directory '%s'.\n", dir)
			}
			return
		}
		if len(mp3Files) == 0 && podCfg.AdRemoval == AdRemovalNone {
			if !cli.Quiet {
				fmt.Printf("Podcast config set to 'none' (No ad removal). Skipping ad removal for '%s'.\n", dir)
			}
			return
		}
		expandedArgs = mp3Files
	} else {
		if len(args) == 0 {
			app := buildCLIApp(&action, &cli)
			app.RenderCommand(clihelp.Options{}, "file")
			os.Exit(1)
		}
		expandedArgs = args
	}

	selectedProfile := selectProfile(config, cli.UseLLM)
	batchStartTime := time.Now()

	totalFiles := len(expandedArgs)

	for idx, inputFile := range expandedArgs {
		fileStartTime := time.Now()

		if strings.HasSuffix(inputFile, ".json") {
			processJSONFile(inputFile, cli)
			continue
		}

		mainMP3File, precutFile, sourceAudioFile := resolveAudioFiles(inputFile, cli)

		baseName := stripExt(mainMP3File)
		jsonFile := cli.TranscriptPath
		if jsonFile == "" {
			jsonFile = baseName + ".transcript.json"
		}
		cutsFile := baseName + ".cuts.json"

		outputFile := resolveOutputFile(mainMP3File, cli, totalFiles)

		speedFactor := config.WhisperSpeedFactor
		if speedFactor <= 0 {
			speedFactor = 7.0
		}

		shortName := displayName(filepath.Base(inputFile))

		if fileExists(jsonFile) && fileExists(cutsFile) && !cli.ForceTranscribe && !cli.ForceLLM {
			if cli.Verbose && !cli.Quiet {
				fmt.Printf("skipping: %s\n", shortName)
			}
			continue
		}

		if !cli.Quiet {
			if totalFiles > 1 {
				printSeparator()
				dir := filepath.Dir(inputFile)
				base := filepath.Base(inputFile)
				fmt.Printf("Processing (%d/%d):\n  %s\n  %s\n", idx+1, totalFiles, dir, bold(base))
			} else {
				fmt.Printf("Processing: %s\n", bold(shortName))
			}
		}
		totalDuration := getAudioDuration(sourceAudioFile)

		if cli.TranscribeMin != "" {
			totalDuration = handleTranscribeMin(&sourceAudioFile, totalDuration, cli)
		}

		if cli.Recut {
			handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName, totalDuration, selectedProfile, cli, fileStartTime)
			continue
		}

		costInfo := getProfileCost(selectedProfile)

		if !cli.Quiet {
			fmt.Printf("  Duration: %s\n", bold(formatTime(totalDuration)))
			fmt.Printf("  Profile:  %s\n", bold(fmt.Sprintf("[%d] %s (%s)", selectedProfile.ID, selectedProfile.Name, selectedProfile.Model)))
			fmt.Printf("  Pricing:  %s\n", costInfo.CostStr)
		}

		t0Step1 := time.Now()
		isNewlyTranscribed := false

		whisperLanguage := config.WhisperLanguage
		whisperPrompt := config.WhisperPrompt
		id3Tags := map[string]string{}

		transcriptionData, err := loadOrTranscribe(sourceAudioFile, jsonFile, config, cli, selectedProfile, totalDuration, speedFactor, whisperLanguage, whisperPrompt, id3Tags, &isNewlyTranscribed, &t0Step1)
		if err != nil {
			hasError = true
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		detectedLang := transcriptionData.Language
		if detectedLang == "" && len(transcriptionData.Segments) > 0 {
			detectedLang = transcriptionData.Segments[0].Language
		}
		if !cli.Quiet && detectedLang != "" {
			langLabel := "(auto-detected)"
			if whisperLanguage != "" {
				langLabel = "(config override)"
			}
			fmt.Printf("   Detected language: %s %s\n", strings.ToUpper(detectedLang), langLabel)
		}

		if whisperLanguage == "" && isNewlyTranscribed {
			fullText := transcriptionData.Text
			if fullText == "" {
				for _, seg := range transcriptionData.Segments {
					fullText += seg.Text + " "
				}
			}
			scriptLang := detectScriptLanguage(fullText)
			if scriptLang != "" && scriptLang != detectedLang {
				transcriptionData.Language = scriptLang
				if !cli.Quiet {
					fmt.Printf("   Corrected language from %s to %s (detected from script)\n", strings.ToUpper(detectedLang), strings.ToUpper(scriptLang))
				}
			}
		}

		if !validateTranscriptSanity(transcriptionData, totalDuration, cli.Quiet) {
			hasError = true
			continue
		}

		if isNewlyTranscribed && cli.SaveTranscript {
			saveJSONTranscript(mainMP3File, transcriptionData, jsonFile, cli.Quiet, id3Tags)
		}

		if cli.ExportSRT {
			convertJSONToSRT(jsonFile, transcriptionData, cli.TranscriptPath, cli.Quiet)
		}

		if cli.ExportTXT {
			convertJSONToTXT(jsonFile, transcriptionData, totalDuration, cli.TranscriptPath, cli.Quiet)
		}

		if cli.ExportSRT || cli.ExportTXT {
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Printf("Export completed in %s\n", formatClock(fileTotalDuration.Seconds()))
			}
			continue
		}

		if cli.TranscribeMin != "" {
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Printf("Preview transcription completed in %s\n", formatClock(fileTotalDuration.Seconds()))
				fmt.Println("   Transcript saved - original file was not modified.")
			}
			if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
				os.Remove(sourceAudioFile)
			}
			continue
		}

		formattedTranscript := formatTranscript(transcriptionData, totalDuration)

		t0Step2 := time.Now()
		if !cli.Quiet {
			fmt.Println()
			fmt.Println(boldYellow("Step 2/3: Detecting ad/sponsor segments via LLM (" + selectedProfile.Model + ")..."))
		}
		adSegments := detectAdsLLM(formattedTranscript, selectedProfile)
		if len(adSegments) > 0 {
			adSegments = mergeIntervals(adSegments)
		}
		step2Duration := time.Since(t0Step2)
		if !cli.Quiet && cli.Verbose {
			fmt.Printf("Step 2/3 (Ad Detection) finished in %s\n", formatClock(step2Duration.Seconds()))
		}

		if len(adSegments) == 0 {
			saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Println("No ad segments detected by LLM!")
				printTimingSummary(cli.Verbose, totalDuration, totalDuration, 0, 0, 0, step1Duration(t0Step1), step2Duration, 0, fileTotalDuration)
			}
			if sourceAudioFile != outputFile {
				copyFile(sourceAudioFile, outputFile)
			}
			fmt.Printf("Result saved to: '%s'\n", outputFile)
			continue
		}

		if cli.Verbose && !cli.Quiet {
			fmt.Println()
			fmt.Println("AD SEGMENTS DETECTED TO REMOVE:")
			for _, ad := range adSegments {
				duration := ad.End - ad.Start
				reason := ad.Reason
				if reason == "" {
					reason = "Ad segment"
				}
				fmt.Printf("  - [%s -> %s] (%.1fs): %s\n", formatTime(ad.Start), formatTime(ad.End), duration, reason)
			}
			fmt.Println()
		}

		cutsResult := saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
		keepSegments := cutsResult.KeepSegments

		t0Step3 := time.Now()
		if !cli.Quiet {
			fmt.Println()
			fmt.Printf("Step 3/3: Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
		}

		workDir := workDirFor(outputFile)
		os.MkdirAll(workDir, 0755)
		tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
		verifyTempFile(tempOutputFile)

		if cutAudioFFmpeg(sourceAudioFile, keepSegments, tempOutputFile) {
			step3Duration := time.Since(t0Step3)
			if !cli.Quiet && cli.Verbose {
				fmt.Printf("Step 3/3 (Audio Cutting) finished in %s\n", formatClock(step3Duration.Seconds()))
			}

			if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
				checkPrecutSymlink(precutFile)
				safeMove(mainMP3File, precutFile)
				if !cli.Quiet {
					fmt.Printf("Original file preserved at: '%s'\n", precutFile)
				}
			}

			safeMove(tempOutputFile, outputFile)
			os.RemoveAll(workDir)

			newDuration := getAudioDuration(outputFile)
			actualCut := totalDuration - newDuration
			pctCut := 0.0
			if totalDuration > 0 {
				pctCut = actualCut / totalDuration * 100
			}
			fileTotalDuration := time.Since(fileStartTime)

			if !cli.Quiet {
				printFullSummary(cli.Verbose, totalDuration, newDuration, actualCut, pctCut, len(adSegments),
					step1Duration(t0Step1), step2Duration, step3Duration, fileTotalDuration)
				fmt.Printf("\nSuccess! Ad-free episode saved to: '%s'\n", outputFile)
			}
		} else {
			hasError = true
			os.Remove(tempOutputFile)
			os.RemoveAll(workDir)
			fmt.Fprintf(os.Stderr, "Failed to output ad-free audio for '%s'.\n", inputFile)
		}

		if strings.HasSuffix(sourceAudioFile, ".truncated.wav") {
			os.Remove(sourceAudioFile)
		}
	}

	if totalFiles > 1 && !cli.Quiet {
		batchDuration := time.Since(batchStartTime)
		fmt.Printf("\nBatch Completed! Processed %d file(s) in %s.\n", totalFiles, formatClock(batchDuration.Seconds()))
	}

	os.Stdout.Sync()
	os.Stderr.Sync()
}

func removeWorkDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(dir, entry.Name())
			if entry.Name() == ".work" {
				os.RemoveAll(path)
			} else {
				removeWorkDirs(path)
			}
		}
	}
}
