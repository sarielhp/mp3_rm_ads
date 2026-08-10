package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		if cli.Verbose {
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
	fmt.Println()
	fmt.Println("DURATION & TIME SAVED SUMMARY:")
	fmt.Printf("  - Original Episode Length: %s (%.1fs)\n", formatTime(totalDuration), totalDuration)
	fmt.Printf("  - Total Ad Time Cut:       %s (%.1fs across %d segment(s))\n", formatTime(actualCut), actualCut, numAds)
	fmt.Printf("  - New Episode Length:      %s (%.1fs)\n", formatTime(newDuration), newDuration)
	fmt.Printf("  - Reduction:               %.1f%% of episode trimmed\n", pctCut)
	fmt.Printf("  - Running Times:\n")
	fmt.Printf("      - Step 1 (Transcription): %s\n", formatClock(step1.Seconds()))
	fmt.Printf("      - Step 2 (Ad Detection):  %s\n", formatClock(step2.Seconds()))
	fmt.Printf("      - Step 3 (Audio Cut):     %s\n", formatClock(step3.Seconds()))
	fmt.Printf("      - Total File Processing:  %s\n", formatClock(total.Seconds()))
}
