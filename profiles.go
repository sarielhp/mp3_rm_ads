package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	fatalError("Error: Profile ID [%d] not found in configuration.\n", targetID)
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

	id := 0
	fmt.Sscanf(useLLM, "%d", &id)
	if id > 0 {
		for _, p := range cfg.Profiles {
			if p.ID == id {
				return p
			}
		}
	}

	lowerQuery := strings.ToLower(useLLM)
	for _, p := range cfg.Profiles {
		if strings.Contains(strings.ToLower(p.Name), lowerQuery) || strings.Contains(strings.ToLower(p.Model), lowerQuery) {
			return p
		}
	}

	fmt.Fprintf(os.Stderr, "Error: Profile '%s' not found.\n", useLLM)
	listProfiles(cfg)
	fatalError("")
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
		fatalError("Error: Input audio file '%s' (or '%s') not found.\n", inputFile, precutFile)
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

func listWhispers(cfg Config) {
	activeID := cfg.ActiveWhisperID
	fmt.Printf("\n%s\n", repeatStr("=", 70))
	fmt.Println("AVAILABLE WHISPER SERVERS:")
	fmt.Printf("%s\n", repeatStr("=", 70))

	if len(cfg.WhisperProfiles) == 0 {
		fmt.Println("No Whisper profiles configured in configuration file.")
		fmt.Println("Currently using fallback/legacy configuration:")
		fmt.Printf("      - URL:          %s [DEFAULT]\n", cfg.WhisperURL)
		fmt.Printf("      - Speed Factor: %.1f\n", cfg.WhisperSpeedFactor)
		if cfg.WhisperDockerContainer != "" {
			fmt.Printf("      - Container:    %s\n", cfg.WhisperDockerContainer)
		}
		if cfg.WhisperLanguage != "" {
			fmt.Printf("      - Language:     %s\n", cfg.WhisperLanguage)
		}
		if cfg.WhisperPrompt != "" {
			fmt.Printf("      - Prompt:       %s\n", cfg.WhisperPrompt)
		}
		if cfg.WhisperWakeCommand != "" {
			fmt.Printf("      - Wake Cmd:     %s\n", cfg.WhisperWakeCommand)
		}
	} else {
		for _, wp := range cfg.WhisperProfiles {
			isDefault := wp.ID == activeID
			defaultBadge := ""
			if isDefault {
				defaultBadge = " [DEFAULT]"
			}
			fmt.Printf("  [%d] %s%s\n", wp.ID, wp.Name, defaultBadge)
			fmt.Printf("      - URL:          %s\n", wp.URL)
			fmt.Printf("      - Speed Factor: %.1f\n", wp.SpeedFactor)
			if wp.DockerContainer != "" {
				fmt.Printf("      - Container:    %s\n", wp.DockerContainer)
			}
			if wp.Language != "" {
				fmt.Printf("      - Language:     %s\n", wp.Language)
			}
			if wp.Prompt != "" {
				fmt.Printf("      - Prompt:       %s\n", wp.Prompt)
			}
			if wp.WakeCommand != "" {
				fmt.Printf("      - Wake Cmd:     %s\n", wp.WakeCommand)
			}
			fmt.Println()
		}
	}
	fmt.Printf("%s\n\n", repeatStr("=", 70))
}

func addWhisperProfile(cfg *Config, spec string) {
	parts := strings.Split(spec, "|")
	if len(parts) < 2 {
		fatalError("Error: Invalid Whisper server spec. Format: Name|URL|[SpeedFactor]|[DockerContainer]|[Language]|[Prompt]|[WakeCommand]\n")
	}

	name := strings.TrimSpace(parts[0])
	url := strings.TrimSpace(parts[1])
	if name == "" || url == "" {
		fatalError("Error: Name and URL cannot be empty in Whisper server spec.\n")
	}

	speedFactor := 7.0
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		if sf, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			speedFactor = sf
		}
	}

	var container, language, prompt, wakeCmd string
	if len(parts) > 3 {
		container = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		language = strings.TrimSpace(parts[4])
	}
	if len(parts) > 5 {
		prompt = strings.TrimSpace(parts[5])
	}
	if len(parts) > 6 {
		wakeCmd = strings.TrimSpace(parts[6])
	}

	nextID := 1
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID >= nextID {
			nextID = wp.ID + 1
		}
	}

	newProfile := WhisperProfile{
		ID:              nextID,
		Name:            name,
		URL:             url,
		SpeedFactor:     speedFactor,
		DockerContainer: container,
		Language:        language,
		Prompt:          prompt,
		WakeCommand:     wakeCmd,
	}

	cfg.WhisperProfiles = append(cfg.WhisperProfiles, newProfile)

	if cfg.ActiveWhisperID <= 0 {
		cfg.ActiveWhisperID = nextID
	}

	saveConfig(*cfg)
	fmt.Printf("Added Whisper server profile [%d] %s (URL: %s)\n", nextID, name, url)
}

func removeWhisperProfile(cfg *Config, targetID int) {
	foundIndex := -1
	for i, wp := range cfg.WhisperProfiles {
		if wp.ID == targetID {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		fatalError("Error: Whisper server profile [%d] not found in configuration.\n", targetID)
	}

	profileName := cfg.WhisperProfiles[foundIndex].Name
	cfg.WhisperProfiles = append(cfg.WhisperProfiles[:foundIndex], cfg.WhisperProfiles[foundIndex+1:]...)

	if cfg.ActiveWhisperID == targetID {
		if len(cfg.WhisperProfiles) > 0 {
			cfg.ActiveWhisperID = cfg.WhisperProfiles[0].ID
		} else {
			cfg.ActiveWhisperID = 0
		}
	}

	saveConfig(*cfg)
	fmt.Printf("Removed Whisper server profile [%d] %s\n", targetID, profileName)
}

func setDefaultWhisperProfile(cfg *Config, targetID int) {
	if targetID == 0 {
		cfg.ActiveWhisperID = 0
		saveConfig(*cfg)
		fmt.Println("Default Whisper server updated to fallback/legacy configuration.")
		return
	}
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID == targetID {
			cfg.ActiveWhisperID = targetID
			saveConfig(*cfg)
			fmt.Printf("Default Whisper server profile updated to [%d] %s\n", targetID, wp.Name)
			return
		}
	}
	fatalError("Error: Whisper server profile [%d] not found in configuration.\n", targetID)
}
