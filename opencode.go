package main

import (
	"fmt"
	"os"
	"strings"
)

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
