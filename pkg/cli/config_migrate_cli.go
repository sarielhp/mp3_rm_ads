package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sariel/abs/pkg/config"
	"github.com/sariel/abs/pkg/util"
)

func migratePodcastsManagerConfig(cfg *Config) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	pmConfigDir := filepath.Join(home, ".config", "podcasts_manager")
	pmConfigPath := filepath.Join(pmConfigDir, "config.json")
	if _, err := os.Stat(pmConfigPath); os.IsNotExist(err) {
		pmConfigDir = filepath.Join(home, ".config", "podcast_manager")
		pmConfigPath = filepath.Join(pmConfigDir, "config.json")
	}

	if _, err := os.Stat(pmConfigPath); os.IsNotExist(err) {
		return false
	}

	data, err := os.ReadFile(pmConfigPath)
	if err != nil {
		return false
	}

	var pmCfg struct {
		Host           string   `json:"host"`
		Token          string   `json:"token"`
		SQLiteDBPath   string   `json:"sqlite_db_path"`
		PodcastsDir    string   `json:"podcasts_dir"`
		PostProcessors []string `json:"post_processors"`
	}

	if err := json.Unmarshal(data, &pmCfg); err != nil {
		return false
	}

	modified := false
	if cfg.AudiobookshelfURL == "" && pmCfg.Host != "" {
		cfg.AudiobookshelfURL = pmCfg.Host
		modified = true
	}
	if cfg.AudiobookshelfToken == "" && pmCfg.Token != "" {
		cfg.AudiobookshelfToken = pmCfg.Token
		modified = true
	}
	if cfg.AudiobookshelfDBPath == "" && pmCfg.SQLiteDBPath != "" {
		cfg.AudiobookshelfDBPath = pmCfg.SQLiteDBPath
		modified = true
	}
	if cfg.PodcastsDir == "" && pmCfg.PodcastsDir != "" {
		cfg.PodcastsDir = pmCfg.PodcastsDir
		modified = true
	}
	if len(cfg.PostProcessors) == 0 && len(pmCfg.PostProcessors) > 0 {
		cfg.PostProcessors = pmCfg.PostProcessors
		modified = true
	}

	if modified {
		fmt.Printf("Migrated settings from podcast_manager config '%s'\n", pmConfigPath)
	}
	return modified
}

func handleConfigMigrate(cfg *Config, source string) {
	migrated := false
	source = strings.ToLower(strings.TrimSpace(source))

	checkLegacy := source == "" || source == "all" || source == "legacy" || source == "mp3_rm_ads"
	checkPM := source == "" || source == "all" || source == "pm" || source == "podcasts_manager" || source == "podcast_manager"

	if checkLegacy {
		legacy := config.LegacyConfigPath()
		if legacy != "" && util.FileExists(legacy) {
			legacyData, err := os.ReadFile(legacy)
			if err == nil && len(legacyData) > 0 {
				var legCfg Config
				if err := json.Unmarshal(legacyData, &legCfg); err == nil {
					if cfg.PodcastsDir == "" && legCfg.PodcastsDir != "" {
						cfg.PodcastsDir = legCfg.PodcastsDir
						migrated = true
					}
					if cfg.WhisperURL == "" && legCfg.WhisperURL != "" {
						cfg.WhisperURL = legCfg.WhisperURL
						migrated = true
					}
					if cfg.AudiobookshelfURL == "" && legCfg.AudiobookshelfURL != "" {
						cfg.AudiobookshelfURL = legCfg.AudiobookshelfURL
						migrated = true
					}
					if cfg.AudiobookshelfToken == "" && legCfg.AudiobookshelfToken != "" {
						cfg.AudiobookshelfToken = legCfg.AudiobookshelfToken
						migrated = true
					}
					if len(cfg.Profiles) == 0 && len(legCfg.Profiles) > 0 {
						cfg.Profiles = legCfg.Profiles
						migrated = true
					}
					if len(cfg.WhisperProfiles) == 0 && len(legCfg.WhisperProfiles) > 0 {
						cfg.WhisperProfiles = legCfg.WhisperProfiles
						migrated = true
					}
					fmt.Printf("Migrated settings from legacy config '%s'\n", legacy)
				}
			}
		}
	}

	if checkPM {
		if migratePodcastsManagerConfig(cfg) {
			migrated = true
		}
	}

	if migrated {
		saveConfig(*cfg)
		fmt.Printf("Configuration saved to '%s'\n", config.ConfigPath())
	} else {
		fmt.Println("No legacy configuration found to migrate or settings already up-to-date.")
	}
}

func resolveProcessorPath(prog string) (string, error) {
	path, err := exec.LookPath(prog)
	if err == nil {
		return filepath.Abs(path)
	}
	if util.FileExists(prog) {
		return filepath.Abs(prog)
	}
	return "", fmt.Errorf("program '%s' not found or not executable", prog)
}

func handleConfigProcessor(cfg *Config, cmd string, value string) {
	switch cmd {
	case "set":
		if value == "" {
			fatalError("%s\n", "Error: missing program for 'config processor set <program>'")
		}
		fullPath, err := resolveProcessorPath(value)
		if err != nil {
			fatalError("%s\n", fmt.Sprintf("Error: failed to resolve post-processor program: %v", err))
		}
		exists := false
		for _, p := range cfg.PostProcessors {
			if p == fullPath {
				exists = true
				break
			}
		}
		if !exists {
			cfg.PostProcessors = append(cfg.PostProcessors, fullPath)
			saveConfig(*cfg)
		}
		fmt.Printf("Added post-processor: %s\n", fullPath)

	case "list":
		if len(cfg.PostProcessors) == 0 {
			fmt.Println("No post-processors configured.")
		} else {
			fmt.Println("=== Configured Post-Processors ===")
			for i, p := range cfg.PostProcessors {
				fmt.Printf("  %d. %s\n", i+1, p)
			}
		}

	case "del":
		if value == "" {
			fatalError("%s\n", "Error: missing number for 'config processor del <number>'")
		}
		idx, err := strconv.Atoi(value)
		if err != nil || idx < 1 || idx > len(cfg.PostProcessors) {
			fatalError("%s\n", fmt.Sprintf("Error: invalid post-processor number '%s'. Must be between 1 and %d.", value, len(cfg.PostProcessors)))
		}
		removed := cfg.PostProcessors[idx-1]
		cfg.PostProcessors = append(cfg.PostProcessors[:idx-1], cfg.PostProcessors[idx:]...)
		saveConfig(*cfg)
		fmt.Printf("Deleted post-processor #%d: %s\n", idx, removed)

	default:
		fatalError("%s\n", fmt.Sprintf("Error: unknown processor command '%s'", cmd))
	}
}
