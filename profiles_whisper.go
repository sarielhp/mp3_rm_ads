package main

import (
	"fmt"
	"strconv"
	"strings"
)

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
