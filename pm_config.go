package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func migratePodcastsManagerConfig(cfg *Config) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	pmConfigDir := filepath.Join(home, ".config", "podcasts_manager")
	pmConfigPath := filepath.Join(pmConfigDir, "config.json")
	if _, err := os.Stat(pmConfigPath); os.IsNotExist(err) {
		pmConfigDir = filepath.Join(home, ".config", "podcast_manager")
		pmConfigPath = filepath.Join(pmConfigDir, "config.json")
	}

	if _, err := os.Stat(pmConfigPath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(pmConfigPath)
	if err != nil {
		return
	}

	var pmCfg struct {
		Host           string   `json:"host"`
		Token          string   `json:"token"`
		SQLiteDBPath   string   `json:"sqlite_db_path"`
		PodcastsDir    string   `json:"podcasts_dir"`
		PostProcessors []string `json:"post_processors"`
	}

	if err := json.Unmarshal(data, &pmCfg); err != nil {
		return
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
		saveConfig(*cfg)
		fmt.Printf("Migrated settings from podcast_manager config '%s' to '%s'\n", pmConfigPath, configPath())
	}
}

func getABSTokenFromDB(dbPath string) string {
	if dbPath == "" || !fileExists(dbPath) {
		return ""
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return ""
	}
	defer db.Close()

	var token string
	query := "SELECT token FROM users WHERE token IS NOT NULL AND token != '' ORDER BY createdAt ASC LIMIT 1;"
	err = db.QueryRow(query).Scan(&token)
	if err != nil {
		return ""
	}
	return token
}

func resolveProcessorPath(prog string) (string, error) {
	path, err := exec.LookPath(prog)
	if err == nil {
		return filepath.Abs(path)
	}
	if fileExists(prog) {
		return filepath.Abs(prog)
	}
	return "", fmt.Errorf("program '%s' not found or not executable", prog)
}

func handleConfigProcessor(cfg *Config, cmd string, value string) {
	switch cmd {
	case "set":
		if value == "" {
			printError("Error: missing program for 'config processor set <program>'")
			os.Exit(1)
		}
		fullPath, err := resolveProcessorPath(value)
		if err != nil {
			printError(fmt.Sprintf("Error: failed to resolve post-processor program: %v", err))
			os.Exit(1)
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
			printError("Error: missing number for 'config processor del <number>'")
			os.Exit(1)
		}
		idx, err := strconv.Atoi(value)
		if err != nil || idx < 1 || idx > len(cfg.PostProcessors) {
			printError(fmt.Sprintf("Error: invalid post-processor number '%s'. Must be between 1 and %d.", value, len(cfg.PostProcessors)))
			os.Exit(1)
		}
		removed := cfg.PostProcessors[idx-1]
		cfg.PostProcessors = append(cfg.PostProcessors[:idx-1], cfg.PostProcessors[idx:]...)
		saveConfig(*cfg)
		fmt.Printf("Deleted post-processor #%d: %s\n", idx, removed)

	default:
		printError(fmt.Sprintf("Error: unknown processor command '%s'", cmd))
		os.Exit(1)
	}
}

func printConfigInfo(cfg Config) {
	fmt.Println("=== Audiobookshelf & LLM Config Info ===")
	fmt.Printf("Config File:  %s\n", configPath())

	hostStr := cfg.AudiobookshelfURL
	if hostStr == "" {
		hostStr = "Not Set"
	}
	fmt.Printf("ABS URL:      %s\n", hostStr)

	tokenStr := "Not Set"
	if cfg.AudiobookshelfToken != "" {
		if len(cfg.AudiobookshelfToken) > 10 {
			tokenStr = fmt.Sprintf("%s... (length: %d chars)", cfg.AudiobookshelfToken[:10], len(cfg.AudiobookshelfToken))
		} else {
			tokenStr = fmt.Sprintf("%s... (length: %d chars)", cfg.AudiobookshelfToken, len(cfg.AudiobookshelfToken))
		}
	}
	fmt.Printf("ABS Token:    %s\n", tokenStr)

	dbStr := cfg.AudiobookshelfDBPath
	if dbStr == "" {
		dbStr = "Not Set"
	}
	fmt.Printf("ABS SQLite:   %s\n", dbStr)

	dirStr := cfg.PodcastsDir
	if dirStr == "" {
		dirStr = "Not Set"
	}
	fmt.Printf("Podcasts Dir: %s\n", dirStr)

	if len(cfg.PostProcessors) == 0 {
		fmt.Println("Post-Processors: None")
	} else {
		fmt.Println("Post-Processors:")
		for i, p := range cfg.PostProcessors {
			fmt.Printf("  %d. %s\n", i+1, p)
		}
	}
}

func runPostProcessors(processors []string, silent bool) {
	if !silent {
		fmt.Printf("\n=== Executing %d Post-Processor(s) ===\n", len(processors))
	}
	for _, proc := range processors {
		if !silent {
			fmt.Printf("Running post-processor: %s...\n", proc)
		}
		parts := strings.Fields(proc)
		if len(parts) == 0 {
			continue
		}
		var cmd *exec.Cmd
		if len(parts) > 1 {
			cmd = exec.Command(parts[0], parts[1:]...)
		} else {
			cmd = exec.Command(parts[0])
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			printError(fmt.Sprintf("Post-processor '%s' failed: %v", proc, err))
		}
	}
}
