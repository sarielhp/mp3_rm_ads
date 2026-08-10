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
)

func main() {
	cli := parseFlags()

	if cli.TestWhisper {
		testWhisperServer(loadConfig().WhisperURL)
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
		printConfig(config)
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
			printUsage()
			os.Exit(1)
		}
		dir := args[0]
		removeWorkDirs(dir)
		mp3Files := findMP3Files(dir)
		if len(mp3Files) == 0 {
			if !cli.Quiet {
				fmt.Printf("No MP3 files found in directory '%s'.\n", dir)
			}
			return
		}
		expandedArgs = mp3Files
	} else if len(args) == 0 {
		if config.PodcastsDir != "" {
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
			printUsage()
			return
		}
	} else {
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err == nil && info.IsDir() {
				removeWorkDirs(arg)
				mp3Files := findMP3Files(arg)
				if len(mp3Files) == 0 {
					if !cli.Quiet {
						fmt.Printf("No MP3 files found in directory '%s'.\n", arg)
					}
				} else {
					expandedArgs = append(expandedArgs, mp3Files...)
				}
			} else {
				expandedArgs = append(expandedArgs, arg)
			}
		}
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
			if !cli.Quiet {
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
		if !cli.Quiet {
			fmt.Printf("Step 2/3 (Ad Detection) finished in %s\n", formatClock(step2Duration.Seconds()))
		}

		if len(adSegments) == 0 {
			saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
			fileTotalDuration := time.Since(fileStartTime)
			if !cli.Quiet {
				fmt.Println("No ad segments detected by LLM!")
				printTimingSummary(totalDuration, totalDuration, 0, 0, 0, step1Duration(t0Step1), step2Duration, 0, fileTotalDuration)
			}
			if sourceAudioFile != outputFile {
				copyFile(sourceAudioFile, outputFile)
			}
			fmt.Printf("Result saved to: '%s'\n", outputFile)
			continue
		}

		if !cli.Quiet {
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
			if !cli.Quiet {
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
				printFullSummary(totalDuration, newDuration, actualCut, pctCut, len(adSegments),
					step1Duration(t0Step1), step2Duration, step3Duration, fileTotalDuration)
				fmt.Printf("\nSuccess! Ad-free episode saved to: '%s'\n", outputFile)
			}
		} else {
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

func testWhisperServer(whisperURL string) {
	if whisperURL == "" {
		fmt.Println("ERROR: whisper_url is not configured in config file.")
		return
	}

	fmt.Printf("Testing whisper server at: %s\n", whisperURL)

	pcmData := make([]byte, 3200)
	header := buildWavHeader(len(pcmData))
	audioContent := append(header, pcmData...)

	boundary := fmt.Sprintf("----WhisperBoundary%d", time.Now().UnixNano())
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.SetBoundary(boundary)
	fw, _ := w.CreateFormFile("file", "test.wav")
	fw.Write(audioContent)
	w.WriteField("response_format", "verbose_json")
	w.WriteField("temperature", "0.0")
	w.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", whisperURL, &buf)
	if err != nil {
		fmt.Printf("ERROR: Failed to create request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ERROR: Connection failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		fmt.Println("SUCCESS: Whisper server responded OK (200)")
	} else {
		fmt.Printf("FAIL: Server returned status %d: %s\n", resp.StatusCode, string(body))
	}
}

func parseFlags() CLIOptions {
	for _, a := range os.Args[1:] {
		switch a {
		case "help", "usage", "-h", "--h", "-help", "--help", "?", "-?":
			printUsage()
			os.Exit(0)
		}
	}

	cli := CLIOptions{
		SaveTranscript: true,
	}

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "config" {
		cli.IsConfigCommand = true
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "dir" {
		cli.IsDirCommand = true
		args = args[1:]
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flag.StringVar(&cli.Output, "o", "", "Output MP3 path or directory")
	flag.StringVar(&cli.Output, "output", "", "Output MP3 path or directory")
	flag.BoolVar(&cli.Quiet, "q", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Quiet, "quiet", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Verbose, "v", false, "Verbose output (show detailed info)")
	flag.BoolVar(&cli.Verbose, "verbose", false, "Verbose output (show detailed info)")
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
	flag.BoolVar(&cli.TestWhisper, "test-whisper", false, "Test whisper server connection and exit")

	flag.Usage = printUsage
	flag.CommandLine.Parse(args)

	// Handle flags visited
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

func printUsage() {
	fmt.Printf("\n%s\n", repeatStr("=", 75))
	fmt.Println("  mp3_rm_ads - Automatic Podcast Ad & Sponsor Segment Remover")
	fmt.Printf("%s\n", repeatStr("=", 75))
	fmt.Println("\nUSAGE:")
	fmt.Println("  mp3_rm_ads [command] [options]")
	fmt.Println("\nCOMMANDS:")
	fmt.Println("  <file1.mp3> [file2.mp3 ...]    Process individual MP3 files (default)")
	fmt.Println("  <transcript.json> [options]    Process a transcript JSON file")
	fmt.Println("  dir <directory> [options]      Recursively process all MP3s in a directory")
	fmt.Println("  config [options]               Manage configuration")
	fmt.Println("\nIf no command is given and podcasts_dir is configured,")
	fmt.Println("it is equivalent to: mp3_rm_ads dir <podcasts_dir>")
	fmt.Println("\nOPTIONS:")
	fmt.Println("  -o, --output PATH              Output MP3 path or directory")
	fmt.Println("  -q, --quiet                    Suppress progress and informational output")
	fmt.Println("  -v, --verbose                  Verbose output (show detailed info)")
	fmt.Println("  --srt                          Export/convert transcript JSON to SubRip (.srt)")
	fmt.Println("  --txt                          Export/convert transcript JSON to text (.txt)")
	fmt.Println("  --recut                        Recut audio using .cuts.json (no Whisper/LLM)")
	fmt.Println("  --no-transcript                Disable saving default .transcript.json file")
	fmt.Println("  --use-chunks                   Split long audio into chunks for reliable transcription")
	fmt.Println("  --extract-keywords             Extract keywords via LLM to improve Whisper accuracy")
	fmt.Println("  -t, --transcribe-minutes Nm    Only transcribe first N minutes (e.g. -t 10m)")
	fmt.Println("  --force-llm                    Force re-running LLM ad detection even if .cuts.json exists")
	fmt.Println("  --force-transcribe             Force re-transcribing audio even if .transcript.json exists")
	fmt.Println("  --use-llm ID_OR_NAME           Select active LLM profile by ID or name")
	fmt.Println("  --list-llms                    List all configured LLM profiles and exit")
	fmt.Println("  --list-profiles                List all configured LLM profiles and exit")
	fmt.Println("  --set-default ID               Set default LLM profile ID in config file")
	fmt.Println("  --podcasts_dir DIR             Set default podcasts/media directory in config file")
	fmt.Println("  --copy_llm_from_opencode       Import LLM settings from OpenCode config")
	fmt.Println("  --test-whisper                  Test whisper server connection and exit")
	fmt.Println("  -h, --help                     Show this detailed usage message")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  mp3_rm_ads episode.mp3")
	fmt.Println("  mp3_rm_ads episode1.mp3 episode2.mp3 episode3.mp3")
	fmt.Println("  mp3_rm_ads dir ~/podcasts")
	fmt.Println("  mp3_rm_ads dir ~/podcasts -q")
	fmt.Println("  mp3_rm_ads config --podcasts_dir /path/to/podcasts")
	fmt.Println("  mp3_rm_ads --recut episode.mp3")
	fmt.Println("  mp3_rm_ads -q --srt episode.mp3")
	fmt.Println("  mp3_rm_ads episode.transcript.json -srt -txt")
	fmt.Println("  mp3_rm_ads --use-llm 2 episode.mp3")
	fmt.Println("  mp3_rm_ads --copy_llm_from_opencode")
	fmt.Printf("%s\n\n", repeatStr("=", 75))
}
