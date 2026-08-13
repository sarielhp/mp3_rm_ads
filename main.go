package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
)

func main() {
	cli := parseFlags()

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
		tmp, err := os.CreateTemp(tmpDir, "mp3_rm_ads_silent_*.log")
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
	}()

	if cli.TestWhisper || cli.IsTestCommand {
		ensureConfigExists()
		config := loadConfig()
		if !testWhisperServer(config.WhisperURL, cli.Quiet) {
			hasError = true
			os.Exit(1)
		}
		return
	}

	ensureConfigExists()
	config := loadConfig()

	if cli.SetPodcastsDir {
		setPodcastsDir(&config, cli.PodcastsDir)
		if cli.IsConfigCommand || flag.NArg() == 0 {
			return
		}
	}

	if cli.IsConfigCommand {
		if cli.SetPodcastsDir || cli.SetDefault > 0 || cli.CopyOpenCode || cli.ListLLMs {
			printConfig(config)
		} else {
			clihelp.PrintCommandUsage(buildUsageApp(), "config")
		}
		return
	}

	if cli.CopyOpenCode {
		copyLLMFromOpenCode(&config)
		if flag.NArg() == 0 {
			return
		}
	}

	go wakeWhisperServer(config.WhisperURL, cli.Quiet)

	if cli.ListLLMs {
		listProfiles(config)
		return
	}

	if cli.SetDefault > 0 {
		setDefaultProfile(&config, cli.SetDefault)
		return
	}

	args := flag.Args()

	var expandedArgs []string

	if cli.IsDirCommand {
		if len(args) == 0 {
			clihelp.PrintCommandUsage(buildUsageApp(), "dir")
			os.Exit(1)
		}
		dir := args[0]
		if !cli.Quiet {
			fmt.Printf("Scanning: %s\n", dir)
		}
		removeWorkDirs(dir)
		mp3Files := findMP3Files(dir)
		if len(mp3Files) == 0 {
			if !cli.Quiet {
				fmt.Printf("No MP3 files found in directory '%s'.\n", dir)
			}
			return
		}
		expandedArgs = mp3Files
	} else if cli.IsFileCommand {
		if len(args) == 0 {
			clihelp.PrintCommandUsage(buildUsageApp(), "file")
			os.Exit(1)
		}
		expandedArgs = args
	} else if len(args) == 0 {
		if config.PodcastsDir != "" {
			if !cli.Quiet {
				fmt.Printf("Scanning: %s\n", config.PodcastsDir)
			}
			removeWorkDirs(config.PodcastsDir)
			mp3Files := findMP3Files(config.PodcastsDir)
			if len(mp3Files) == 0 {
				if !cli.Quiet {
					fmt.Printf("No MP3 files found in directory '%s'.\n", config.PodcastsDir)
				}
				return
			}
			expandedArgs = mp3Files
		} else {
			clihelp.PrintGlobalUsage(buildUsageApp())
			return
		}
	} else {
		clihelp.PrintGlobalUsage(buildUsageApp())
		os.Exit(1)
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
			pctCut := actualCut / totalDuration * 100
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

func wakeWhisperServer(whisperURL string, quiet bool) {
	if whisperURL == "" {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", whisperURL, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func testWhisperServer(whisperURL string, quiet bool) bool {
	return testWhisperServerEx(whisperURL, 5, 3*time.Second, quiet)
}

func testWhisperServerEx(whisperURL string, maxRetries int, retryDelay time.Duration, quiet bool) bool {
	if whisperURL == "" {
		fmt.Println("ERROR: whisper_url is not configured in config file.")
		return false
	}

	if !quiet {
		fmt.Printf("Testing whisper server at: %s\n", whisperURL)
	}

	pcmData := make([]byte, 3200)
	header := buildWavHeader(len(pcmData))
	audioContent := append(header, pcmData...)

	var lastErr error
	var lastStatus int
	var lastResponseBody string

	client := &http.Client{Timeout: 10 * time.Second}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		wakeWhisperServer(whisperURL, true)

		boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		w.SetBoundary(boundary)
		fw, _ := w.CreateFormFile("file", "test.wav")
		fw.Write(audioContent)
		w.WriteField("response_format", "verbose_json")
		w.WriteField("temperature", "0.0")
		w.Close()

		req, err := http.NewRequest("POST", whisperURL, &buf)
		if err != nil {
			if !quiet {
				fmt.Printf("ERROR: Failed to create request: %v\n", err)
			}
			return false
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				if !quiet {
					fmt.Printf("Attempt %d/%d: Connection error: %v (server may be sleeping, retrying in %v...)\n",
						attempt, maxRetries, err, retryDelay)
				}
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		lastResponseBody = string(body)

		if resp.StatusCode == http.StatusOK {
			fmt.Println("SUCCESS: Whisper server responded OK (200)")
			return true
		}

		if resp.StatusCode >= 500 && attempt < maxRetries {
			if !quiet {
				fmt.Printf("Attempt %d/%d: Server returned status %d: %s (server may be waking up, retrying in %v...)\n",
					attempt, maxRetries, resp.StatusCode, lastResponseBody, retryDelay)
			}
			time.Sleep(retryDelay)
			continue
		}

		fmt.Printf("FAIL: Server returned status %d: %s\n", resp.StatusCode, lastResponseBody)
		return false
	}

	if lastErr != nil {
		fmt.Printf("FAIL: Could not connect to Whisper server at '%s' after %d attempt(s): %v\n", whisperURL, maxRetries, lastErr)
	} else {
		fmt.Printf("FAIL: Server at '%s' returned status %d after %d attempt(s): %s\n", whisperURL, lastStatus, maxRetries, lastResponseBody)
	}
	return false
}

func parseFlags() CLIOptions {
	cli := CLIOptions{
		SaveTranscript: true,
	}

	args := os.Args[1:]

	var detectedCommand string
	if len(args) > 0 {
		switch args[0] {
		case "config":
			cli.IsConfigCommand = true
			detectedCommand = "config"
			args = args[1:]
		case "dir":
			cli.IsDirCommand = true
			detectedCommand = "dir"
			args = args[1:]
		case "file":
			cli.IsFileCommand = true
			detectedCommand = "file"
			args = args[1:]
		case "test":
			cli.IsTestCommand = true
			detectedCommand = "test"
			args = args[1:]
			if len(args) > 0 && (args[0] == "whisper" || args[0] == "whisper-server") {
				cli.TestWhisper = true
				args = args[1:]
			} else {
				cli.TestWhisper = true
			}
		case "test-whisper":
			cli.IsTestCommand = true
			cli.TestWhisper = true
			detectedCommand = "test"
			args = args[1:]
		}
	}

	for _, a := range args {
		switch a {
		case "help", "usage", "-h", "--h", "-help", "--help", "?", "-?":
			if detectedCommand != "" {
				clihelp.PrintCommandUsage(buildUsageApp(), detectedCommand)
			} else {
				clihelp.PrintGlobalUsage(buildUsageApp())
			}
			os.Exit(0)
		}
	}

	testWhisperCmd := cli.TestWhisper
	isTestCmd := cli.IsTestCommand

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flag.StringVar(&cli.Output, "o", "", "Output MP3 path or directory")
	flag.StringVar(&cli.Output, "output", "", "Output MP3 path or directory")
	flag.BoolVar(&cli.Quiet, "q", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Quiet, "quiet", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Verbose, "v", false, "Verbose output (show detailed info)")
	flag.BoolVar(&cli.Verbose, "verbose", false, "Verbose output (show detailed info)")
	flag.BoolVar(&cli.Silent, "s", false, "Suppress output unless error occurs")
	flag.BoolVar(&cli.Silent, "silent", false, "Suppress output unless error occurs")
	flag.BoolVar(&cli.ExportSRT, "srt", false, "Export/convert transcript JSON to SubRip (.srt)")
	flag.BoolVar(&cli.ExportTXT, "txt", false, "Export/convert transcript JSON to text (.txt)")
	flag.BoolVar(&cli.Recut, "recut", false, "Recut audio using .cuts.json (no Whisper/LLM)")
	flag.BoolVar(&cli.SaveTranscript, "no-transcript", true, "Disable saving default .transcript.json file")
	flag.BoolVar(&cli.UseChunks, "use-chunks", false, "Split long audio into chunks for reliable transcription")
	flag.BoolVar(&cli.ExtractKeywords, "extract-keywords", false, "Extract keywords via LLM to improve Whisper accuracy")
	flag.StringVar(&cli.TranscribeMin, "t", "", "Only transcribe first N minutes (e.g. -t 10m)")
	flag.StringVar(&cli.TranscribeMin, "transcribe-minutes", "", "Only transcribe first N minutes (e.g. -t 10m)")
	flag.BoolVar(&cli.ForceLLM, "force-llm", false, "Force re-running LLM ad detection even if .cuts.json exists")
	flag.BoolVar(&cli.ForceTranscribe, "force-transcribe", false, "Force re-transcribing audio even if .transcript.json exists")
	flag.StringVar(&cli.UseLLM, "use-llm", "", "Select active LLM profile by ID or name")
	flag.StringVar(&cli.UseLLM, "profile", "", "Select active LLM profile by ID or name")
	flag.IntVar(&cli.SetDefault, "set-default", 0, "Set default LLM profile ID in config file")
	flag.StringVar(&cli.PodcastsDir, "podcasts_dir", "", "Set default podcasts/media directory in config file")
	flag.StringVar(&cli.PodcastsDir, "podcasts-dir", "", "Set default podcasts/media directory in config file")
	flag.BoolVar(&cli.ListLLMs, "list-llms", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.ListLLMs, "list-profiles", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.CopyOpenCode, "copy_llm_from_opencode", false, "Import LLM settings from OpenCode config")
	var testWhisperFlag bool
	flag.BoolVar(&testWhisperFlag, "test-whisper", false, "Test whisper server connection and exit")

	flag.Usage = func() {
		if detectedCommand != "" {
			clihelp.PrintCommandUsage(buildUsageApp(), detectedCommand)
		} else {
			clihelp.PrintGlobalUsage(buildUsageApp())
		}
	}
	flag.CommandLine.Parse(args)

	if testWhisperCmd || testWhisperFlag || isTestCmd {
		cli.TestWhisper = true
		cli.IsTestCommand = true
	}

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "no-transcript" {
			cli.SaveTranscript = false
		}
		if f.Name == "podcasts_dir" || f.Name == "podcasts-dir" {
			cli.SetPodcastsDir = true
		}
	})

	return cli
}

func buildUsageApp() *clihelp.App {
	return &clihelp.App{
		Name:        "mp3_rm_ads",
		Description: "Automatic Podcast Ad & Sponsor Segment Remover",
		GlobalNote:  "If no command is given and podcasts_dir is configured, it is equivalent to: mp3_rm_ads dir <podcasts_dir>",
		Commands: []clihelp.Command{
			{
				Name:        "file",
				Description: "Process individual MP3 or transcript JSON files",
				UsageLine:   "mp3_rm_ads file <file1.mp3> [file2.mp3 ...] [options]",
				Options:     globalOptions(),
				Examples:    fileExamples(),
			},
			{
				Name:        "dir",
				Description: "Recursively process all MP3s in a directory",
				UsageLine:   "mp3_rm_ads dir <directory> [options]",
				Options:     globalOptions(),
				Examples: []clihelp.Example{
					{Line: "mp3_rm_ads dir ~/podcasts"},
					{Line: "mp3_rm_ads dir ~/podcasts -q"},
				},
			},
			{
				Name:        "config",
				Description: "Manage configuration",
				UsageLine:   "mp3_rm_ads config [options]",
				Options: []clihelp.Option{
					{Flags: "--podcasts_dir DIR", Description: "Set default podcasts/media directory in config file"},
					{Flags: "--list-llms", Description: "List all configured LLM profiles and exit"},
					{Flags: "--set-default ID", Description: "Set default LLM profile ID in config file"},
					{Flags: "--copy_llm_from_opencode", Description: "Import LLM settings from OpenCode config"},
				},
				Examples: []clihelp.Example{
					{Line: "mp3_rm_ads config --podcasts_dir /path/to/podcasts"},
					{Line: "mp3_rm_ads config --list-llms"},
					{Line: "mp3_rm_ads config --set-default 2"},
					{Line: "mp3_rm_ads config --copy_llm_from_opencode"},
				},
			},
			{
				Name:        "test",
				Description: "Test external services like Whisper server",
				UsageLine:   "mp3_rm_ads test whisper [options]",
				Options: []clihelp.Option{
					{Flags: "--test-whisper", Description: "Test whisper server connection with retries"},
				},
				Examples: []clihelp.Example{
					{Line: "mp3_rm_ads test whisper"},
					{Line: "mp3_rm_ads test"},
				},
			},
		},
	}
}

func globalOptions() []clihelp.Option {
	return []clihelp.Option{
		{Flags: "-o, --output PATH", Description: "Output MP3 path or directory"},
		{Flags: "-q, --quiet", Description: "Suppress progress and informational output"},
		{Flags: "-v, --verbose", Description: "Verbose output (show detailed info)"},
		{Flags: "-s, --silent", Description: "Suppress output unless error occurs"},
		{Flags: "--srt", Description: "Export/convert transcript JSON to SubRip (.srt)"},
		{Flags: "--txt", Description: "Export/convert transcript JSON to text (.txt)"},
		{Flags: "--recut", Description: "Recut audio using .cuts.json (no Whisper/LLM)"},
		{Flags: "--no-transcript", Description: "Disable saving default .transcript.json file"},
		{Flags: "--use-chunks", Description: "Split long audio into chunks for reliable transcription"},
		{Flags: "--extract-keywords", Description: "Extract keywords via LLM to improve Whisper accuracy"},
		{Flags: "-t, --transcribe-minutes Nm", Description: "Only transcribe first N minutes (e.g. -t 10m)"},
		{Flags: "--force-llm", Description: "Force re-running LLM ad detection even if .cuts.json exists"},
		{Flags: "--force-transcribe", Description: "Force re-transcribing audio even if .transcript.json exists"},
		{Flags: "--use-llm ID_OR_NAME", Description: "Select active LLM profile by ID or name"},
		{Flags: "--list-llms", Description: "List all configured LLM profiles and exit"},
		{Flags: "--set-default ID", Description: "Set default LLM profile ID in config file"},
		{Flags: "--podcasts_dir DIR", Description: "Set default podcasts/media directory in config file"},
		{Flags: "--copy_llm_from_opencode", Description: "Import LLM settings from OpenCode config"},
		{Flags: "--test-whisper", Description: "Test whisper server connection and exit"},
		{Flags: "-h, --help", Description: "Show this detailed usage message"},
	}
}

func fileExamples() []clihelp.Example {
	return []clihelp.Example{
		{Line: "mp3_rm_ads file episode.mp3"},
		{Line: "mp3_rm_ads file episode1.mp3 episode2.mp3 episode3.mp3"},
		{Line: "mp3_rm_ads file --recut episode.mp3"},
		{Line: "mp3_rm_ads file -q --srt episode.mp3"},
		{Line: "mp3_rm_ads file episode.transcript.json -srt -txt"},
		{Line: "mp3_rm_ads file --use-llm 2 episode.mp3"},
		{Line: "mp3_rm_ads file --copy_llm_from_opencode"},
	}
}
