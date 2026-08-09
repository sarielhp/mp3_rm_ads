package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	config := loadConfig()

	cli := parseFlags()

	if cli.CopyOpenCode {
		copyLLMFromOpenCode(&config)
		if flag.NArg() == 0 {
			return
		}
	}

	if cli.ListLLMs {
		listProfiles(config)
		return
	}

	if cli.SetDefault > 0 {
		setDefaultProfile(&config, cli.SetDefault)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return
	}

	selectedProfile := selectProfile(config, cli.UseLLM)

	batchStartTime := time.Now()

	var expandedArgs []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err == nil && info.IsDir() {
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

	totalFiles := len(expandedArgs)

	for idx, inputFile := range expandedArgs {
		fileStartTime := time.Now()

		if totalFiles > 1 && !cli.Quiet {
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Printf("Processing File [%d/%d]: '%s'\n", idx+1, totalFiles, inputFile)
			fmt.Printf("%s\n", repeatStr("=", 65))
		}

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

		if fileExists(jsonFile) && fileExists(cutsFile) && !cli.ForceTranscribe && !cli.ForceLLM {
			if !cli.Quiet {
				fmt.Printf("Skipping '%s' (both .transcript.json and .cuts.json exist). Use --force-transcribe or --force-llm to reprocess.\n", inputFile)
			}
			continue
		}

		if !cli.Quiet {
			fmt.Printf("Processing local file: '%s'\n", sourceAudioFile)
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
			fmt.Printf("Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
			fmt.Printf("Active LLM Profile: [%d] %s (%s)\n", selectedProfile.ID, selectedProfile.Name, selectedProfile.Model)
			fmt.Printf("LLM Model Pricing: %s (%s)\n", costInfo.CostStr, costInfo.Est1HStr)
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
			fmt.Printf("Step 2/3: Detecting ad/sponsor segments via LLM (%s)...\n", selectedProfile.Model)
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
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Printf("AD SEGMENTS DETECTED TO REMOVE (%d segment(s)):\n", len(adSegments))
			fmt.Printf("%s\n", repeatStr("=", 65))
			for _, ad := range adSegments {
				duration := ad.End - ad.Start
				reason := ad.Reason
				if reason == "" {
					reason = "Ad segment"
				}
				fmt.Printf("  - [%s -> %s] (%.1fs): %s\n", formatTime(ad.Start), formatTime(ad.End), duration, reason)
			}
			fmt.Printf("%s\n\n", repeatStr("=", 65))
		}

		cutsResult := saveCutsJSON(mainMP3File, totalDuration, adSegments, &selectedProfile, cli.Quiet)
		keepSegments := cutsResult.KeepSegments

		t0Step3 := time.Now()
		if !cli.Quiet {
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
		fmt.Printf("\n%s\n", repeatStr("=", 65))
		fmt.Printf("Batch Completed! Processed %d file(s) in %s.\n", totalFiles, formatClock(batchDuration.Seconds()))
		fmt.Printf("%s\n\n", repeatStr("=", 65))
	}
}

func parseFlags() CLIOptions {
	cli := CLIOptions{
		SaveTranscript: true,
	}

	flag.StringVar(&cli.Output, "o", "", "Output MP3 path or directory")
	flag.StringVar(&cli.Output, "output", "", "Output MP3 path or directory")
	flag.BoolVar(&cli.Quiet, "q", false, "Suppress progress and informational output")
	flag.BoolVar(&cli.Quiet, "quiet", false, "Suppress progress and informational output")
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
	flag.BoolVar(&cli.ListLLMs, "list-llms", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.ListLLMs, "list-profiles", false, "List all configured LLM profiles and exit")
	flag.BoolVar(&cli.CopyOpenCode, "copy_llm_from_opencode", false, "Import LLM settings from OpenCode config")

	flag.Usage = printUsage
	flag.Parse()

	// Handle --no-transcript flag (inverted logic)
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "no-transcript" {
			cli.SaveTranscript = false
		}
	})

	return cli
}

func printUsage() {
	fmt.Printf("\n%s\n", repeatStr("=", 75))
	fmt.Println("  mp3_rm_ads - Automatic Podcast Ad & Sponsor Segment Remover")
	fmt.Printf("%s\n", repeatStr("=", 75))
	fmt.Println("\nUSAGE:")
	fmt.Println("  mp3_rm_ads <file1.mp3> [file2.mp3 ...] [options]")
	fmt.Println("  mp3_rm_ads <transcript.json> [options]")
	fmt.Println("\nOPTIONS:")
	fmt.Println("  -o, --output PATH              Output MP3 path or directory")
	fmt.Println("  -q, --quiet                    Suppress progress and informational output")
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
	fmt.Println("  --set-default ID               Set default LLM profile ID in config file")
	fmt.Println("  --copy_llm_from_opencode       Import LLM settings from OpenCode config")
	fmt.Println("  -h, --help                     Show this detailed usage message")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  mp3_rm_ads episode.mp3")
	fmt.Println("  mp3_rm_ads episode1.mp3 episode2.mp3 episode3.mp3")
	fmt.Println("  mp3_rm_ads --recut episode.mp3")
	fmt.Println("  mp3_rm_ads -q --srt episode.mp3")
	fmt.Println("  mp3_rm_ads episode.transcript.json -srt -txt")
	fmt.Println("  mp3_rm_ads --use-llm 2 episode.mp3")
	fmt.Println("  mp3_rm_ads --copy_llm_from_opencode")
	fmt.Printf("%s\n\n", repeatStr("=", 75))
}

func listProfiles(cfg Config) {
	activeID := cfg.ActiveProfileID
	fmt.Printf("\n%s\n", repeatStr("=", 70))
	fmt.Println("AVAILABLE LLM PROFILES & PRICING:")
	fmt.Printf("%s\n", repeatStr("=", 70))

	for _, p := range cfg.Profiles {
		isDefault := p.ID == activeID
		defaultBadge := ""
		if isDefault {
			defaultBadge = " [DEFAULT]"
		}
		hasKey := ""
		if p.APIKey != "" {
			hasKey = " (Key set)"
		}
		costInfo := getProfileCost(p)

		headerStr := fmt.Sprintf("  [%d] %s", p.ID, p.Name)
		fmt.Printf("%s%s\n", headerStr, defaultBadge)
		fmt.Printf("      - Model:     %s\n", p.Model)
		fmt.Printf("      - Type:      %s%s\n", p.Type, hasKey)
		fmt.Printf("      - Pricing:   %s\n", costInfo.CostStr)
		fmt.Printf("      - Est. 1-Hr: %s\n", costInfo.Est1HStr)
		fmt.Printf("      - URL:       %s\n", p.URL)
		fmt.Println()
	}
	fmt.Printf("%s\n\n", repeatStr("=", 70))
}

func setDefaultProfile(cfg *Config, targetID int) {
	for _, p := range cfg.Profiles {
		if p.ID == targetID {
			cfg.ActiveProfileID = targetID
			saveConfig(*cfg)
			fmt.Printf("Default LLM profile updated to [%d] %s\n", targetID, p.Name)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Error: Profile ID [%d] not found in configuration.\n", targetID)
	os.Exit(1)
}

func selectProfile(cfg Config, useLLM string) LLMProfile {
	if useLLM == "" {
		for _, p := range cfg.Profiles {
			if p.ID == cfg.ActiveProfileID {
				return p
			}
		}
		if len(cfg.Profiles) > 0 {
			return cfg.Profiles[0]
		}
		return LLMProfile{}
	}

	// Try numeric ID first
	id := 0
	fmt.Sscanf(useLLM, "%d", &id)
	if id > 0 {
		for _, p := range cfg.Profiles {
			if p.ID == id {
				return p
			}
		}
	}

	// Try name/model match
	lowerQuery := strings.ToLower(useLLM)
	for _, p := range cfg.Profiles {
		if strings.Contains(strings.ToLower(p.Name), lowerQuery) || strings.Contains(strings.ToLower(p.Model), lowerQuery) {
			return p
		}
	}

	fmt.Fprintf(os.Stderr, "Error: Profile '%s' not found.\n", useLLM)
	listProfiles(cfg)
	os.Exit(1)
	return LLMProfile{}
}

func resolveAudioFiles(inputFile string, cli CLIOptions) (mainMP3File, precutFile, sourceAudioFile string) {
	if strings.HasSuffix(inputFile, ".precut") {
		precutFile = inputFile
		mainMP3File = strings.TrimSuffix(inputFile, ".precut")
	} else {
		mainMP3File = inputFile
		precutFile = inputFile + ".precut"
	}

	if fileExists(precutFile) {
		checkPrecutSymlink(precutFile)
		sourceAudioFile = precutFile
		if !cli.Quiet {
			fmt.Printf("Found existing pre-cut audio source: '%s'\n", precutFile)
		}
	} else if fileExists(mainMP3File) {
		sourceAudioFile = mainMP3File
	} else {
		fmt.Fprintf(os.Stderr, "Error: Input audio file '%s' (or '%s') not found.\n", inputFile, precutFile)
		os.Exit(1)
	}
	return
}

func resolveOutputFile(mainMP3File string, cli CLIOptions, totalFiles int) string {
	if totalFiles > 1 && cli.Output != "" {
		if info, err := os.Stat(cli.Output); err == nil && info.IsDir() {
			return filepath.Join(cli.Output, filepath.Base(mainMP3File))
		}
	}
	if cli.Output != "" {
		return cli.Output
	}
	return mainMP3File
}

func handleTranscribeMin(sourceAudioFile *string, totalDuration float64, cli CLIOptions) float64 {
	raw := cli.TranscribeMin
	minutes := 0.0
	fmt.Sscanf(raw, "%f", &minutes)
	truncateSec := minutes * 60.0
	if truncateSec < totalDuration {
		workDir := workDirFor(*sourceAudioFile)
		os.MkdirAll(workDir, 0755)
		truncatedPath := filepath.Join(workDir, filepath.Base(*sourceAudioFile)+".truncated.wav")
		verifyTempFile(truncatedPath)
		if !cli.Quiet {
			fmt.Printf("   Reading only first %s...\n", formatTime(truncateSec))
		}
		if truncateAudio(*sourceAudioFile, truncatedPath, truncateSec) {
			*sourceAudioFile = truncatedPath
			return getAudioDuration(*sourceAudioFile)
		}
	}
	return totalDuration
}

func handleRecut(mainMP3File, sourceAudioFile, precutFile, outputFile, baseName string, totalDuration float64, selectedProfile LLMProfile, cli CLIOptions, fileStartTime time.Time) {
	cutsFile := baseName + ".cuts.json"
	if !fileExists(cutsFile) {
		fmt.Fprintf(os.Stderr, "Error: Cut metadata JSON file '%s' not found for recutting.\n", cutsFile)
		return
	}

	jsonFile := cli.TranscriptPath
	if jsonFile == "" {
		jsonFile = baseName + ".transcript.json"
	}
	if fileExists(jsonFile) && !cli.ForceTranscribe && !cli.ForceLLM {
		if !cli.Quiet {
			fmt.Printf("Skipping '%s' (.transcript.json exists). Use --force-transcribe or --force-llm to reprocess.\n", mainMP3File)
		}
		return
	}

	if !cli.Quiet {
		fmt.Printf("Recutting audio using existing cut metadata: '%s'\n", cutsFile)
	}

	data, err := readFile(cutsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading cuts file: %v\n", err)
		return
	}
	var cutsData CutsData
	if err := jsonUnmarshal(data, &cutsData); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing cuts file: %v\n", err)
		return
	}

	var existingAds []AdSegment
	for _, c := range cutsData.CutIntervals {
		st := c.StartSec
		en := c.EndSec
		existingAds = append(existingAds, AdSegment{Start: st, End: en, Reason: c.Reason})
	}

	if len(existingAds) > 0 {
		existingAds = mergeIntervals(existingAds)
	}

	cutsResult := saveCutsJSON(mainMP3File, totalDuration, existingAds, &selectedProfile, cli.Quiet)
	keepSegments := cutsResult.KeepSegments

	if len(keepSegments) == 0 {
		if !cli.Quiet {
			fmt.Println("No keep segments found in cut metadata.")
		}
		return
	}

	if !cli.Quiet {
		mergedIntervals := cutsData.MergedCutIntervals
		if len(mergedIntervals) == 0 {
			fmt.Println("No cut intervals specified in metadata!")
		} else {
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Printf("CUT INTERVALS TO REMOVE (%d segment(s)):\n", len(mergedIntervals))
			fmt.Printf("%s\n", repeatStr("=", 65))
			for _, m := range mergedIntervals {
				duration := m.End - m.Start
				fmt.Printf("  - [%s -> %s] (%.1fs)\n", formatTime(m.Start), formatTime(m.End), duration)
			}
			fmt.Printf("%s\n\n", repeatStr("=", 65))
		}
	}

	t0Recut := time.Now()
	if !cli.Quiet {
		fmt.Printf("Cutting ads with ffmpeg (%d non-ad clips)...\n", len(keepSegments))
	}

	workDir := workDirFor(outputFile)
	os.MkdirAll(workDir, 0755)
	tempOutputFile := filepath.Join(workDir, filepath.Base(outputFile)+".tmp"+filepath.Ext(outputFile))
	verifyTempFile(tempOutputFile)

	if cutAudioFFmpeg(sourceAudioFile, keepSegments, tempOutputFile) {
		recutDuration := time.Since(t0Recut)
		if !cli.Quiet {
			fmt.Printf("Audio Recutting finished in %s\n", formatClock(recutDuration.Seconds()))
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
			fmt.Printf("\n%s\n", repeatStr("=", 65))
			fmt.Println("DURATION & TIME SAVED SUMMARY (RECUT):")
			fmt.Printf("%s\n", repeatStr("=", 65))
			fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
			fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs)\n", formatTime(actualCut), actualCut)
			fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
			fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
			fmt.Printf("  - Total Recut Time:        %s\n", formatClock(fileTotalDuration.Seconds()))
			fmt.Printf("%s\n\n", repeatStr("=", 65))
			fmt.Printf("Success! Recut ad-free episode saved to: '%s'\n", outputFile)
		}
	} else {
		os.Remove(tempOutputFile)
		os.RemoveAll(workDir)
	}
}

func loadOrTranscribe(sourceAudioFile, jsonFile string, config Config, cli CLIOptions, selectedProfile LLMProfile, totalDuration, speedFactor float64, whisperLanguage, whisperPrompt string, id3TagsOut map[string]string, isNewlyTranscribed *bool, t0Step1 *time.Time) (*TranscriptionData, error) {
	if fileExists(jsonFile) {
		if !cli.Quiet {
			fmt.Printf("Found existing transcript JSON file: '%s'. Reusing transcript...\n", jsonFile)
		}
		data, err := readFile(jsonFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read transcript file: %w", err)
		}
		var td TranscriptionData
		if err := jsonUnmarshal(data, &td); err != nil {
			return nil, fmt.Errorf("failed to parse transcript JSON: %w", err)
		}
		step1Duration := time.Since(*t0Step1)
		if !cli.Quiet {
			fmt.Printf("Step 1/3 (Transcript Loaded) finished in %s\n", formatClock(step1Duration.Seconds()))
		}
		return &td, nil
	}

	if !cli.Quiet {
		fmt.Println("Step 1/3: Transcribing audio via AMD GPU Whisper server...")
	}

	if whisperPrompt == "" {
		id3Tags := extractID3Tags(sourceAudioFile)
		for k, v := range id3Tags {
			id3TagsOut[k] = v
		}
		var tagTexts []string
		for _, key := range []string{"title", "artist", "album", "genre", "comment", "description", "synopsis", "purl", "encodedby", "copyright"} {
			if val, ok := id3TagsOut[key]; ok && val != "" {
				tagTexts = append(tagTexts, val)
			}
		}
		tagText := strings.Join(tagTexts, "\n")
		if tagText != "" {
			if !cli.Quiet {
				keys := make([]string, 0, len(id3Tags))
				for k := range id3Tags {
					keys = append(keys, k)
				}
				fmt.Printf("   Extracted ID3 metadata: %s\n", strings.Join(keys, ", "))
				fmt.Println("   Extracting keywords from metadata to improve transcription accuracy...")
			}
			extracted := extractKeywordsLLM(tagText, selectedProfile, cli.Quiet)
			if extracted != "" {
				whisperPrompt = extracted
				if !cli.Quiet {
					fmt.Printf("   Using keywords: %s\n", whisperPrompt)
				}
			}
		} else if !cli.Quiet {
			fmt.Println("   No ID3 metadata found in file for keyword extraction.")
		}
	}

	chunkDuration := config.ChunkDurationSec
	useChunks := cli.UseChunks || (chunkDuration > 0 && totalDuration > float64(chunkDuration)*1.5)

	dockerContainer := config.WhisperDockerContainer
	if dockerContainer == "" {
		dockerContainer = detectWhisperDockerContainer(config.WhisperURL)
		if !cli.Quiet && dockerContainer != "" {
			fmt.Printf("   Auto-detected whisper Docker container: '%s'\n", dockerContainer)
		}
	}

	whisperLangArg := whisperLanguage
	whisperPromptArg := whisperPrompt
	if whisperPromptArg == "" {
		whisperPromptArg = ""
	}

	var transcriptionData *TranscriptionData
	var err error

	if useChunks {
		parallelChunks := config.ParallelChunks
		if parallelChunks < 1 {
			parallelChunks = 1
		}
		if !cli.Quiet {
			numChunks := int(totalDuration / float64(chunkDuration))
			if numChunks < 1 {
				numChunks = 1
			}
			fmt.Printf("   Audio is %s long - splitting into %d chunks of %s for reliability...\n",
				formatTime(totalDuration), numChunks, formatTime(float64(chunkDuration)))
			if parallelChunks > 1 {
				fmt.Printf("   Parallel chunks: %d\n", parallelChunks)
			}
		}
		transcriptionData, err = transcribeChunks(
			sourceAudioFile, config.WhisperURL, cli.Quiet,
			totalDuration, speedFactor, chunkDuration, parallelChunks,
			dockerContainer, whisperPromptArg, whisperLangArg,
		)
	} else {
		transcriptionData, err = transcribeWhisper(
			sourceAudioFile, config.WhisperURL, cli.Quiet,
			totalDuration, speedFactor, dockerContainer,
			whisperPromptArg, whisperLangArg, nil,
		)
		if err != nil && strings.Contains(err.Error(), "failed to") && totalDuration > 300 {
			if !cli.Quiet {
				fmt.Println("\nFull-file transcription failed - retrying in chunks...")
			}
			chunkDur := config.ChunkDurationSec
			if chunkDur <= 0 {
				chunkDur = 900
			}
			transcriptionData, err = transcribeChunks(
				sourceAudioFile, config.WhisperURL, cli.Quiet,
				totalDuration, speedFactor, chunkDur, 1,
				dockerContainer, whisperPromptArg, whisperLangArg,
			)
		}
	}

	if err != nil {
		return nil, err
	}

	step1Duration := time.Since(*t0Step1)
	if !cli.Quiet {
		fmt.Printf("Step 1/3 (Transcription) finished in %s\n", formatClock(step1Duration.Seconds()))
	}
	*isNewlyTranscribed = true
	return transcriptionData, nil
}

func formatTranscript(data *TranscriptionData, totalDuration float64) string {
	segments := data.Segments
	if len(segments) == 0 && data.Text != "" {
		return fmt.Sprintf("[0.0s -> %.1fs] %s", totalDuration, data.Text)
	}

	var lines []string
	for _, seg := range segments {
		lines = append(lines, fmt.Sprintf("[%.1fs -> %.1fs] %s", seg.Start, seg.End, seg.Text))
	}
	return strings.Join(lines, "\n")
}

func processJSONFile(inputFile string, cli CLIOptions) {
	if !fileExists(inputFile) {
		fmt.Fprintf(os.Stderr, "Error: Transcript JSON file '%s' not found.\n", inputFile)
		return
	}

	if !cli.Quiet {
		fmt.Printf("Processing transcript JSON file: '%s'\n", inputFile)
	}

	if !cli.ExportSRT && !cli.ExportTXT {
		cli.ExportSRT = true
		cli.ExportTXT = true
	}

	if cli.ExportSRT {
		convertJSONToSRT(inputFile, nil, cli.TranscriptPath, cli.Quiet)
	}
	if cli.ExportTXT {
		convertJSONToTXT(inputFile, nil, 0, cli.TranscriptPath, cli.Quiet)
	}
}

func saveJSONTranscript(mainFile string, data *TranscriptionData, jsonFile string, quiet bool, id3Tags map[string]string) {
	outputData := make(map[string]interface{})
	raw, _ := json.Marshal(data)
	json.Unmarshal(raw, &outputData)

	for k, v := range id3Tags {
		outputData["id3_"+k] = v
	}

	content, _ := json.MarshalIndent(outputData, "", "  ")
	writeFile(jsonFile, append(content, '\n'))
	if !quiet {
		fmt.Printf("Saved raw Whisper JSON data (.json) to: '%s'\n", jsonFile)
	}
}

func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {
	if data == nil {
		if !fileExists(inputFile) {
			fmt.Fprintf(os.Stderr, "Error: Cannot convert to SRT, JSON file not found: '%s'\n", inputFile)
			return ""
		}
		raw, err := readFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading JSON file: %v\n", err)
			return ""
		}
		var td TranscriptionData
		if err := jsonUnmarshal(raw, &td); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
			return ""
		}
		data = &td
	}

	base := stripExt(inputFile)
	srtFile := customPath
	if srtFile == "" || !strings.HasSuffix(srtFile, ".srt") {
		srtFile = base + ".srt"
	}

	var lines []string
	for idx, seg := range data.Segments {
		st := formatSRTTime(seg.Start)
		en := formatSRTTime(seg.End)
		text := strings.TrimSpace(seg.Text)
		lines = append(lines, fmt.Sprintf("%d", idx+1))
		lines = append(lines, fmt.Sprintf("%s --> %s", st, en))
		lines = append(lines, text)
		lines = append(lines, "")
	}

	writeFile(srtFile, []byte(strings.Join(lines, "\n")+"\n"))
	if !quiet {
		fmt.Printf("Converted and saved SubRip Subtitle file (.srt) to: '%s'\n", srtFile)
	}
	return srtFile
}

func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {
	if data == nil {
		if !fileExists(inputFile) {
			fmt.Fprintf(os.Stderr, "Error: Cannot convert to TXT, JSON file not found: '%s'\n", inputFile)
			return ""
		}
		raw, err := readFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading JSON file: %v\n", err)
			return ""
		}
		var td TranscriptionData
		if err := jsonUnmarshal(raw, &td); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
			return ""
		}
		data = &td
	}

	base := stripExt(inputFile)
	txtFile := customPath
	if txtFile == "" || !strings.HasSuffix(txtFile, ".txt") {
		txtFile = base + ".transcript.txt"
	}

	lang := data.Language
	if lang == "" {
		lang = "auto"
	}

	var lines []string
	lines = append(lines, repeatStr("=", 80))
	lines = append(lines, fmt.Sprintf("PODCAST TRANSCRIPTION: %s", filepathBase(base)))
	lines = append(lines, fmt.Sprintf("Original Duration: %s (%.1fs) | Language: %s", formatTime(totalDuration), totalDuration, strings.ToUpper(lang)))
	lines = append(lines, repeatStr("=", 80))
	lines = append(lines, "")

	if len(data.Segments) == 0 && data.Text != "" {
		lines = append(lines, fmt.Sprintf("[00:00.0 -> %s] %s", formatTime(totalDuration), data.Text))
	} else {
		for _, seg := range data.Segments {
			st := seg.Start
			en := seg.End
			text := strings.TrimSpace(seg.Text)
			lines = append(lines, fmt.Sprintf("[%s -> %s] %s", formatTime(st), formatTime(en), text))

			if len(seg.Words) > 0 {
				var wordStrs []string
				for _, w := range seg.Words {
					wordStrs = append(wordStrs, fmt.Sprintf("%s(%.2f-%.2f)", w.Word, w.Start, w.End))
				}
				lines = append(lines, "   Words: "+strings.Join(wordStrs, " "))
			}
		}
	}

	writeFile(txtFile, []byte(strings.Join(lines, "\n")+"\n"))
	if !quiet {
		fmt.Printf("Converted and saved timestamped text transcript (.txt) to: '%s'\n", txtFile)
	}
	return txtFile
}

func checkPrecutSymlink(precutFile string) {
	info, err := os.Lstat(precutFile)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Pre-cut backup file '%s' is a symlink. Refusing to overwrite.\n", precutFile)
		os.Exit(1)
	}
}

func safeMove(src, dst string) {
	os.Remove(dst)
	os.Rename(src, dst)
}

func copyFile(src, dst string) {
	data, err := readFile(src)
	if err != nil {
		return
	}
	writeFile(dst, data)
}

func findMP3Files(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subFiles := findMP3Files(filepath.Join(dir, entry.Name()))
			files = append(files, subFiles...)
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

func step1Duration(t0 time.Time) time.Duration {
	return time.Since(t0)
}

func printTimingSummary(originalDuration, newDuration, actualCut float64, pctCut float64, numAds int, step1, step2, step3 time.Duration, total time.Duration) {
	fmt.Println("\nTIMING SUMMARY:")
	fmt.Printf("   - Original Length:     %s (%.1fs)\n", formatTime(originalDuration), originalDuration)
	fmt.Printf("   - Time Cut:            %s (%.1fs)\n", formatTime(actualCut), actualCut)
	fmt.Printf("   - New Episode Length:  %s (%.1fs)\n", formatTime(newDuration), newDuration)
	fmt.Printf("   - Running Times:\n")
	fmt.Printf("       - Step 1 (Transcription): %s\n", formatClock(step1.Seconds()))
	fmt.Printf("       - Step 2 (Ad Detection):  %s\n", formatClock(step2.Seconds()))
	fmt.Printf("       - Step 3 (Audio Cut):     %s\n", formatClock(step3.Seconds()))
	fmt.Printf("       - Total File Processing:  %s\n", formatClock(total.Seconds()))
}

func printFullSummary(totalDuration, newDuration, actualCut float64, pctCut float64, numAds int, step1, step2, step3 time.Duration, total time.Duration) {
	fmt.Printf("\n%s\n", repeatStr("=", 65))
	fmt.Println("DURATION & TIME SAVED SUMMARY:")
	fmt.Printf("%s\n", repeatStr("=", 65))
	fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
	fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs across %d segment(s))\n", formatTime(actualCut), actualCut, numAds)
	fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
	fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
	fmt.Printf("  - Running Times:\n")
	fmt.Printf("      - Step 1 (Transcription): %s\n", formatClock(step1.Seconds()))
	fmt.Printf("      - Step 2 (Ad Detection):  %s\n", formatClock(step2.Seconds()))
	fmt.Printf("      - Step 3 (Audio Cut):     %s\n", formatClock(step3.Seconds()))
	fmt.Printf("      - Total File Processing:  %s\n", formatClock(total.Seconds()))
	fmt.Printf("%s\n\n", repeatStr("=", 65))
}

func copyLLMFromOpenCode(cfg *Config) {
	ocPath := opencodeConfigPath()
	if ocPath == "" || !fileExists(ocPath) {
		fmt.Fprintf(os.Stderr, "OpenCode configuration file not found.\n")
		return
	}

	data, err := readFile(ocPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading OpenCode config: %v\n", err)
		return
	}

	var ocConfig struct {
		Model      string `json:"model"`
		SmallModel string `json:"small_model"`
	}
	if err := jsonUnmarshal(data, &ocConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing OpenCode config: %v\n", err)
		return
	}

	apiKey := envOr("OPENROUTER_API_KEY", "")
	if apiKey == "" {
		apiKey = envOr("OPENAI_API_KEY", "")
	}

	cleanModel := strings.TrimPrefix(ocConfig.Model, "openrouter/")
	cleanSmallModel := strings.TrimPrefix(ocConfig.SmallModel, "openrouter/")

	fmt.Println("Copying LLM settings from OpenCode...")
	fmt.Printf("   - Imported Primary Model: '%s'\n", cleanModel)
	fmt.Printf("   - Imported Small Model:   '%s'\n", cleanSmallModel)
	if apiKey != "" {
		fmt.Println("   - API Key detected: Yes")
	} else {
		fmt.Println("   - API Key detected: No key in ENV (Set OPENROUTER_API_KEY)")
	}

	updated := false
	nextID := 0
	for _, p := range cfg.Profiles {
		if p.ID > nextID {
			nextID = p.ID
		}
	}
	nextID++

	for _, mod := range []string{cleanModel, cleanSmallModel} {
		if mod == "" {
			continue
		}

		found := false
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Model == mod || strings.Contains(cfg.Profiles[i].Name, mod) {
				if apiKey != "" {
					cfg.Profiles[i].APIKey = apiKey
				}
				cfg.Profiles[i].URL = "https://openrouter.ai/api/v1/chat/completions"
				cfg.Profiles[i].Type = "openrouter"
				fmt.Printf("   Updated existing profile [%d] for model '%s'\n", cfg.Profiles[i].ID, mod)
				found = true
				updated = true
				break
			}
		}

		if !found {
			newProfile := LLMProfile{
				ID:     nextID,
				Name:   fmt.Sprintf("OpenRouter - %s", mod),
				Type:   "openrouter",
				URL:    "https://openrouter.ai/api/v1/chat/completions",
				Model:  mod,
				APIKey: apiKey,
			}
			cfg.Profiles = append(cfg.Profiles, newProfile)
			fmt.Printf("   Added new profile [%d] for model '%s'\n", nextID, mod)
			nextID++
			updated = true
		}
	}

	if updated {
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Model == cleanModel {
				cfg.ActiveProfileID = cfg.Profiles[i].ID
				break
			}
		}
		saveConfig(*cfg)
		fmt.Println("Successfully imported OpenCode configuration!")
	}
}
