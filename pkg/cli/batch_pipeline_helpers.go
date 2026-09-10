package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"abs/pkg/config"
	"abs/pkg/tui"
	"abs/pkg/types"
)

func validateTranscriptSanity(data *TranscriptionData, totalDuration float64, quiet bool) bool {
	if totalDuration <= 0 || data == nil {
		return true
	}
	segments := data.Segments
	fullText := data.Text
	if fullText == "" && len(segments) > 0 {
		var b strings.Builder
		for _, s := range segments {
			b.WriteString(s.Text)
			b.WriteByte(' ')
		}
		fullText = strings.TrimSpace(b.String())
	}
	wordCount := len(strings.Fields(fullText))
	minExpectedWords := int(totalDuration / 60.0 * 15.0)
	if minExpectedWords < 20 {
		minExpectedWords = 20
	}
	lastSegmentEnd := 0.0
	for _, seg := range segments {
		if seg.End > lastSegmentEnd {
			lastSegmentEnd = seg.End
		}
	}
	minRequiredCoverage := totalDuration * 0.85
	var failedReasons []string
	if wordCount < minExpectedWords {
		failedReasons = append(failedReasons, fmt.Sprintf("Word count too low (%d words found, expected at least %d words for %s audio)", wordCount, minExpectedWords, formatClock(totalDuration)))
	}
	if totalDuration >= 30.0 && lastSegmentEnd < minRequiredCoverage {
		failedReasons = append(failedReasons, fmt.Sprintf("Transcript ended prematurely at %s (expected coverage up to at least %s)", formatClock(lastSegmentEnd), formatClock(minRequiredCoverage)))
	}
	if len(failedReasons) > 0 {
		if !quiet {
			fmt.Println("\n" + repeatStr("WARNING ", 5))
			fmt.Println("TRANSCRIPT SANITY CHECK FAILED!")
			fmt.Println(repeatStr("WARNING ", 5))
			for _, reason := range failedReasons {
				fmt.Printf("  - %s\n", reason)
			}
			fmt.Println("  - The Whisper transcription appears incomplete or corrupted.")
			fmt.Println("  - Aborting ad detection and audio cutting for safety.")
			fmt.Println(repeatStr("WARNING ", 5) + "\n")
		}
		return false
	}
	return true
}

func detectScriptLanguage(text string) string {
	if text == "" {
		return ""
	}
	hebrew, arabic, cyrillic, greek, totalLetters := 0, 0, 0, 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			totalLetters++
			switch {
			case r >= 0x0590 && r <= 0x05FF:
				hebrew++
			case (r >= 0x0600 && r <= 0x06FF) || (r >= 0x0750 && r <= 0x077F):
				arabic++
			case r >= 0x0400 && r <= 0x04FF:
				cyrillic++
			case r >= 0x0370 && r <= 0x03FF:
				greek++
			}
		}
	}
	if totalLetters == 0 {
		return ""
	}
	threshold := float64(totalLetters) * 0.10
	switch {
	case float64(hebrew) >= threshold:
		return "he"
	case float64(arabic) >= threshold:
		return "ar"
	case float64(cyrillic) >= threshold:
		return "ru"
	case float64(greek) >= threshold:
		return "el"
	default:
		return ""
	}
}

func printTimingSummary(verbose bool, originalDuration, newDuration, actualCut float64, pctCut float64, numAds int, step1, step2, step3 time.Duration, total time.Duration) {
	fmt.Println("\nTIMING SUMMARY:")
	fmt.Printf("   - Original Length:     %s (%.1fs)\n", formatTime(originalDuration), originalDuration)
	fmt.Printf("   - Time Cut:            %s (%.1fs)\n", formatTime(actualCut), actualCut)
	fmt.Printf("   - New Episode Length:  %s (%.1fs)\n", formatTime(newDuration), newDuration)
	if verbose {
		fmt.Printf("   - Running Times:\n")
		fmt.Printf("       - Step 1 (Transcription): %s\n", formatClock(step1.Seconds()))
		fmt.Printf("       - Step 2 (Ad Detection):  %s\n", formatClock(step2.Seconds()))
		fmt.Printf("       - Step 3 (Audio Cut):     %s\n", formatClock(step3.Seconds()))
		fmt.Printf("       - Total File Processing:  %s\n", formatClock(total.Seconds()))
	} else {
		fmt.Printf("   - Total Running Time:     %s\n", formatClock(total.Seconds()))
	}
}

func printFullSummary(verbose bool, totalDuration, newDuration, actualCut float64, pctCut float64, numAds int, step1, step2, step3 time.Duration, total time.Duration) {
	fmt.Println()
	fmt.Println("DURATION & TIME SAVED SUMMARY:")
	fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
	fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs across %d segment(s))\n", formatTime(actualCut), actualCut, numAds)
	fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
	fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
	if verbose {
		fmt.Printf("  - Running Times:\n")
		fmt.Printf("      - Step 1 (Transcription): %s\n", formatClock(step1.Seconds()))
		fmt.Printf("      - Step 2 (Ad Detection):  %s\n", formatClock(step2.Seconds()))
		fmt.Printf("      - Step 3 (Audio Cut):     %s\n", formatClock(step3.Seconds()))
		fmt.Printf("      - Total File Processing:  %s\n", formatClock(total.Seconds()))
	} else {
		fmt.Printf("  - Total Running Time:      %s\n", formatClock(total.Seconds()))
	}
}

func checkPrecutSymlink(precutFile string) {
	info, err := os.Lstat(precutFile)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		fatalError("ERROR: Pre-cut backup file %q is a symlink. Refusing to overwrite.\n", precutFile)
	}
}

func prepareWhisperFallbackConfig(cfg Config) Config {
	fallback := cfg
	for _, wp := range cfg.WhisperProfiles {
		engine := wp.Engine
		if engine == "" {
			engine = config.InferWhisperEngine(wp)
		}
		if engine == WhisperEngineGemini || engine == types.WhisperEngineRemote {
			continue
		}
		if engine == WhisperEngineLocal || engine == WhisperEngineDocker {
			fallback.ActiveWhisperID = wp.ID
			return fallback
		}
	}
	for _, wp := range cfg.WhisperProfiles {
		engine := wp.Engine
		if engine == "" {
			engine = config.InferWhisperEngine(wp)
		}
		if engine != WhisperEngineGemini {
			fallback.ActiveWhisperID = wp.ID
			return fallback
		}
	}
	fallback.WhisperEngine = WhisperEngineLocal
	return fallback
}

func runGeminiPipelineStep(sourceAudioFile, jsonFile, mainMP3File, precutFile, outputFile string, totalDuration float64, config Config, cli CLIOptions, selectedProfile LLMProfile, fileStartTime time.Time) bool {
	ctx := context.Background()
	t0Step1 := time.Now()

	chunkDur := defaultGeminiChunkSec
	if config.ChunkDurationSec > 0 {
		chunkDur = float64(config.ChunkDurationSec)
	}
	td, ads, err := ProcessWithGeminiConfig(ctx, sourceAudioFile, config, chunkDur)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing with Gemini Flash: %v\n", err)
		return false
	}

	if cli.SaveTranscript {
		_ = saveJSONTranscript(mainMP3File, td, jsonFile, cli.Quiet, map[string]string{})
	}

	if handleExportOrPreviewReturns(td, totalDuration, fileStartTime, sourceAudioFile, jsonFile, cli) {
		return true
	}

	if len(ads) > 0 {
		ads = mergeIntervals(ads)
	}

	t0Step2 := time.Now()
	_ = updateTranscriptAdDetectionStatus(jsonFile, true, "completed", "gemini-flash", "", len(ads))
	updateStatusAdDetection(mainMP3File, true, "completed", "gemini-flash", "")
	if len(ads) == 0 {
		handleNoAdsDetected(mainMP3File, sourceAudioFile, outputFile, totalDuration, selectedProfile, cli, fileStartTime, t0Step1, t0Step2)
		return true
	}

	cutsResult := saveCutsJSON(mainMP3File, totalDuration, ads, &selectedProfile, cli.Quiet)
	t0Step3 := time.Now()
	return executeLocalAudioCutting(sourceAudioFile, mainMP3File, precutFile, outputFile, cutsResult.KeepSegments, ads, totalDuration, config, cli, selectedProfile, fileStartTime, t0Step1, t0Step2, t0Step3)
}

func Execute(args []string) int {
	action, cli, err := parseFlagsArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if action == "" {
		return 0
	}
	ensureConfigExists()
	config := loadConfig()

	if handled, hErr := handleParityCommands(action, config, cli); handled {
		if hErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", hErr)
			return 1
		}
		return 0
	}

	switch action {
	case "config":
		if err := handleMainConfig(&config, cli); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	case "tui":
		if err := tui.RunTUI(&config, cli.PodcastsDir); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			return 1
		}
	case "server", "sync":
		if err := handleServerCommand(config, cli); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	case "offload":
		handleRemoteCommand(config, cli)
	case "rm_ads":
		if err := handleMainProc(config, cli, action); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}
	return 0
}
