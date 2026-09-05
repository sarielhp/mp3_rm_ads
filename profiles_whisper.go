package main

import (
	"fmt"
	"strconv"
	"strings"
)

func whisperEngineBadge(engine WhisperEngine) string {
	switch engine {
	case WhisperEngineLocal:
		return "[LOCAL]"
	case WhisperEngineDocker:
		return "[DOCKER]"
	case WhisperEngineRemote:
		return "[REMOTE]"
	default:
		return "[" + strings.ToUpper(string(engine)) + "]"
	}
}

func printWhisperProfile(wp WhisperProfile, isDefault bool) {
	engine := wp.Engine
	if engine == "" {
		engine = inferWhisperEngine(wp)
	}
	badge := whisperEngineBadge(engine)
	defaultBadge := ""
	if isDefault {
		defaultBadge = " [DEFAULT]"
	}
	fmt.Printf("  [%d] %s %s%s\n", wp.ID, wp.Name, badge, defaultBadge)
	fmt.Printf("      - Engine:       %s\n", engine)
	if engine == WhisperEngineLocal {
		if wp.Model != "" {
			fmt.Printf("      - Model:        %s\n", wp.Model)
		}
		if wp.CliBinary != "" {
			fmt.Printf("      - CLI Binary:   %s\n", wp.CliBinary)
		}
		if wp.Processors > 0 {
			fmt.Printf("      - Processors:   %d\n", wp.Processors)
		}
		if wp.Threads > 0 {
			fmt.Printf("      - Threads:      %d\n", wp.Threads)
		}
		if wp.Greedy {
			fmt.Println("      - Greedy:       true")
		}
	} else {
		if wp.URL != "" {
			fmt.Printf("      - URL:          %s\n", wp.URL)
		}
		if wp.DockerContainer != "" {
			fmt.Printf("      - Container:    %s\n", wp.DockerContainer)
		}
		if wp.WakeCommand != "" {
			fmt.Printf("      - Wake Cmd:     %s\n", wp.WakeCommand)
		}
	}
	if wp.SpeedFactor > 0 {
		fmt.Printf("      - Speed Factor: %.1f\n", wp.SpeedFactor)
	}
	if wp.Language != "" {
		fmt.Printf("      - Language:     %s\n", wp.Language)
	}
	if wp.Prompt != "" {
		fmt.Printf("      - Prompt:       %s\n", wp.Prompt)
	}
	fmt.Println()
}

func listWhispers(cfg Config) {
	activeID := cfg.ActiveWhisperID
	fmt.Printf("\n%s\n", repeatStr("=", 70))
	fmt.Println("AVAILABLE WHISPER SERVERS:")
	fmt.Printf("%s\n", repeatStr("=", 70))

	if len(cfg.WhisperProfiles) == 0 {
		fmt.Println("No Whisper profiles configured in configuration file.")
		fmt.Println("Currently using fallback/legacy configuration:")
		fb := fallbackWhisperProfile(cfg)
		printWhisperProfile(fb, true)
	} else {
		for _, wp := range cfg.WhisperProfiles {
			printWhisperProfile(wp, wp.ID == activeID)
		}
	}
	fmt.Printf("%s\n\n", repeatStr("=", 70))
}

func parseEngineFirstSpec(parts []string, nextID int, name string, engine WhisperEngine) WhisperProfile {
	wp := WhisperProfile{ID: nextID, Name: name, Engine: engine, SpeedFactor: 7.0}
	if engine == WhisperEngineLocal {
		wp.SpeedFactor = 70.0
		wp.Model = "tiny.en"
		wp.CliBinary = "whisper-cli"
		wp.Processors = 4
		wp.Threads = 4
		wp.Greedy = true
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			wp.Model = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
			if sf, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
				wp.SpeedFactor = sf
			}
		}
		if len(parts) > 4 && strings.TrimSpace(parts[4]) != "" {
			if p, err := strconv.Atoi(strings.TrimSpace(parts[4])); err == nil {
				wp.Processors = p
			}
		}
		if len(parts) > 5 && strings.TrimSpace(parts[5]) != "" {
			if t, err := strconv.Atoi(strings.TrimSpace(parts[5])); err == nil {
				wp.Threads = t
			}
		}
		if len(parts) > 6 && strings.TrimSpace(parts[6]) != "" {
			wp.Greedy = strings.TrimSpace(parts[6]) != "false"
		}
		return wp
	}
	if len(parts) > 2 {
		wp.URL = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
		if sf, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
			wp.SpeedFactor = sf
		}
	}
	if engine == WhisperEngineDocker {
		if len(parts) > 4 {
			wp.DockerContainer = strings.TrimSpace(parts[4])
		}
		if len(parts) > 5 {
			wp.Language = strings.TrimSpace(parts[5])
		}
		if len(parts) > 6 {
			wp.Prompt = strings.TrimSpace(parts[6])
		}
	} else {
		if len(parts) > 4 {
			wp.WakeCommand = strings.TrimSpace(parts[4])
		}
		if len(parts) > 5 {
			wp.Language = strings.TrimSpace(parts[5])
		}
		if len(parts) > 6 {
			wp.Prompt = strings.TrimSpace(parts[6])
		}
	}
	return wp
}

func parseURLFirstSpec(parts []string, nextID int, name, url string) WhisperProfile {
	sf := 7.0
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			sf = v
		}
	}
	var container, lang, prompt, wakeCmd, model string
	var engine WhisperEngine
	if len(parts) > 3 {
		container = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		lang = strings.TrimSpace(parts[4])
	}
	if len(parts) > 5 {
		prompt = strings.TrimSpace(parts[5])
	}
	if len(parts) > 6 {
		wakeCmd = strings.TrimSpace(parts[6])
	}
	if len(parts) > 7 {
		engine = WhisperEngine(strings.ToLower(strings.TrimSpace(parts[7])))
	}
	if len(parts) > 8 {
		model = strings.TrimSpace(parts[8])
	}
	wp := WhisperProfile{
		ID:              nextID,
		Name:            name,
		URL:             url,
		SpeedFactor:     sf,
		DockerContainer: container,
		Language:        lang,
		Prompt:          prompt,
		WakeCommand:     wakeCmd,
		Engine:          engine,
		Model:           model,
	}
	if wp.Engine == "" {
		wp.Engine = inferWhisperEngine(wp)
	}
	return wp
}

func parseWhisperProfileSpec(spec string, nextID int) WhisperProfile {
	parts := strings.Split(spec, "|")
	if len(parts) < 2 {
		fatalError("Error: Invalid Whisper server spec. Format: Name|Engine|... or Name|URL|...\n")
	}
	name := strings.TrimSpace(parts[0])
	sec := strings.TrimSpace(parts[1])
	if name == "" || sec == "" {
		fatalError("Error: Name and engine/URL cannot be empty in Whisper server spec.\n")
	}
	lowerSec := strings.ToLower(sec)
	if lowerSec == "local" || lowerSec == "docker" || lowerSec == "remote" {
		return parseEngineFirstSpec(parts, nextID, name, WhisperEngine(lowerSec))
	}
	return parseURLFirstSpec(parts, nextID, name, sec)
}

func addWhisperProfile(cfg *Config, spec string) {
	nextID := 1
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID >= nextID {
			nextID = wp.ID + 1
		}
	}
	newProfile := parseWhisperProfileSpec(spec, nextID)
	cfg.WhisperProfiles = append(cfg.WhisperProfiles, newProfile)
	if cfg.ActiveWhisperID <= 0 {
		cfg.ActiveWhisperID = nextID
	}
	resolveActiveWhisperProfile(cfg)
	saveConfig(*cfg)
	badge := whisperEngineBadge(newProfile.Engine)
	fmt.Printf("Added Whisper server profile [%d] %s %s\n", nextID, newProfile.Name, badge)
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
	resolveActiveWhisperProfile(cfg)
	saveConfig(*cfg)
	fmt.Printf("Removed Whisper server profile [%d] %s\n", targetID, profileName)
}

func setDefaultWhisperProfile(cfg *Config, targetID int) {
	if targetID == 0 {
		cfg.ActiveWhisperID = 0
		resolveActiveWhisperProfile(cfg)
		saveConfig(*cfg)
		fmt.Println("Default Whisper server updated to fallback/legacy configuration.")
		return
	}
	for _, wp := range cfg.WhisperProfiles {
		if wp.ID == targetID {
			cfg.ActiveWhisperID = targetID
			resolveActiveWhisperProfile(cfg)
			saveConfig(*cfg)
			engine := wp.Engine
			if engine == "" {
				engine = inferWhisperEngine(wp)
			}
			badge := whisperEngineBadge(engine)
			fmt.Printf("Default Whisper server profile updated to [%d] %s %s\n", targetID, wp.Name, badge)
			return
		}
	}
	fatalError("Error: Whisper server profile [%d] not found in configuration.\n", targetID)
}
