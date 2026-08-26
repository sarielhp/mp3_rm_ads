package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type absLoginResp struct {
	User struct {
		Token       string `json:"token"`
		AccessToken string `json:"accessToken"`
	} `json:"user"`
}

type absLibrariesResp struct {
	Libraries []absLibrary `json:"libraries"`
}

type absLibrary struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	MediaType string      `json:"mediaType"`
	Folders   []absFolder `json:"folders"`
}

type absFolder struct {
	ID       string `json:"id"`
	FullPath string `json:"fullPath"`
}

type absItemsResp struct {
	Results []absItem `json:"results"`
}

type absItem struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	RelPath   string `json:"relPath"`
	MediaType string `json:"mediaType"`
	Media     struct {
		Metadata struct {
			Title       string   `json:"title"`
			Author      string   `json:"author"`
			Description string   `json:"description"`
			Genres      []string `json:"genres,omitempty"`
			Language    string   `json:"language,omitempty"`
			ReleaseDate string   `json:"releaseDate,omitempty"`
			FeedURL     string   `json:"feedUrl,omitempty"`
			ImageURL    string   `json:"imageUrl,omitempty"`
		} `json:"metadata"`
		Episodes  []absEpisode `json:"episodes,omitempty"`
		CoverPath string       `json:"coverPath"`
	} `json:"media"`
}

type absEpisode struct {
	ID          string        `json:"id"`
	Index       int           `json:"index"`
	Season      string        `json:"season"`
	Episode     string        `json:"episode"`
	EpisodeType string        `json:"episodeType,omitempty"`
	Title       string        `json:"title"`
	Subtitle    string        `json:"subtitle"`
	Description string        `json:"description"`
	PubDate     string        `json:"pubDate"`
	Duration    float64       `json:"duration"`
	Size        int64         `json:"size"`
	PublishedAt int64         `json:"publishedAt"`
	AudioFile   *absAudioFile `json:"audioFile,omitempty"`
}

type absAudioFile struct {
	Metadata struct {
		Filename string `json:"filename"`
		Path     string `json:"path"`
		RelPath  string `json:"relPath"`
		Size     int64  `json:"size"`
	} `json:"metadata"`
	Duration      float64 `json:"duration"`
	BitRate       int     `json:"bitRate,omitempty"`
	Codec         string  `json:"codec,omitempty"`
	Channels      int     `json:"channels,omitempty"`
	ChannelLayout string  `json:"channelLayout,omitempty"`
	Format        string  `json:"format,omitempty"`
}

func absLogin(cfg Config) (string, error) {
	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}

	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, cfg.AudiobookshelfUser, cfg.AudiobookshelfPass)
	req, err := http.NewRequest("POST", baseURL+"/login", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read login response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("login returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var loginResp absLoginResp
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse login response: %w", err)
	}
	if loginResp.User.AccessToken != "" {
		return loginResp.User.AccessToken, nil
	}
	if loginResp.User.Token != "" {
		return loginResp.User.Token, nil
	}
	return "", fmt.Errorf("no token in login response")
}

func absGet(baseURL, token, endpoint string, v interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if resp.StatusCode != 200 {
		return fmt.Errorf("request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return json.Unmarshal(data, v)
}

func testAudiobookshelfServer(cfg Config, quiet bool) bool {
	if cfg.AudiobookshelfURL == "" {
		fmt.Println("ERROR: audiobookshelf_url is not configured. Set it with: abs config --abs-url <url>")
		return false
	}

	if !quiet {
		fmt.Printf("Testing Audiobookshelf server at: %s\n", cfg.AudiobookshelfURL)
	}

	if cfg.AudiobookshelfUser == "" || cfg.AudiobookshelfPass == "" {
		baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(baseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: Could not connect: %v\n", err)
			return false
		}
		resp.Body.Close()
		if !quiet {
			fmt.Println("OK: Audiobookshelf server is reachable (no credentials configured).")
		}
		return true
	}

	token, err := absLogin(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		return false
	}

	if !quiet && token != "" {
		fmt.Println("OK: Audiobookshelf server is reachable and credentials are valid.")
	}
	return true
}

func absMapPodcasts(cfg Config, quiet bool) {
	if cfg.AudiobookshelfURL == "" {
		fmt.Println("ERROR: audiobookshelf_url is not configured.")
		return
	}
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		fmt.Println("ERROR: podcasts_dir is not configured.")
		return
	}

	token, err := absLogin(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")

	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get libraries: %v\n", err)
		return
	}

	for _, lib := range libsResp.Libraries {
		if lib.MediaType != "podcast" {
			continue
		}

		var itemsResp absItemsResp
		endpoint := fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID)
		if err := absGet(baseURL, token, endpoint, &itemsResp); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to get items for library '%s': %v\n", lib.Name, err)
			continue
		}

		for _, item := range itemsResp.Results {
			item := item
			podDir := filepath.Join(podcastsDir, item.RelPath)
			podEntries, err := os.ReadDir(podDir)
			if err != nil {
				continue
			}

			fmt.Printf("\n%s %s\n", displayName(lib.Name+"/"+item.RelPath), tuiDimStyle.Render(fmt.Sprintf("(%s)", item.Media.Metadata.Title)))

			var itemFull absItem
			if err := absGet(baseURL, token, "/api/items/"+item.ID, &itemFull); err != nil {
				if !quiet {
					fmt.Printf("  ERROR: Failed to fetch item details: %v\n", err)
				}
				continue
			}

			epByAudioFile := make(map[string]absEpisode)
			for _, ep := range itemFull.Media.Episodes {
				if ep.AudioFile != nil {
					epByAudioFile[ep.AudioFile.Metadata.Filename] = ep
				}
			}

			for _, entry := range podEntries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(strings.ToLower(entry.Name()), ".mp3") {
					continue
				}

				mp3Path := filepath.Join(podDir, entry.Name())
				hasCut := false
				if _, err := os.Stat(strings.TrimSuffix(mp3Path, ".mp3") + ".cuts.json"); err == nil {
					hasCut = true
				}

				matched, ok := epByAudioFile[entry.Name()]
				if !ok {
					if !quiet {
						fmt.Printf("  ? %s\n", displayName(entry.Name()))
					}
					continue
				}

				summary := fmt.Sprintf("  %s %s\n", greenCheck, displayName(entry.Name()))
				if matched.Title != "" && matched.Title != strings.TrimSuffix(entry.Name(), ".mp3") {
					summary += fmt.Sprintf("    Title:       %s\n", displayName(matched.Title))
				}
				if matched.Description != "" {
					desc := stripHTML(matched.Description)
					if len(desc) > 120 {
						desc = desc[:120] + "..."
					}
					summary += fmt.Sprintf("    Description: %s\n", desc)
				}
				if matched.PubDate != "" {
					summary += fmt.Sprintf("    Published:   %s\n", matched.PubDate)
				}
				if matched.Duration > 0 {
					summary += fmt.Sprintf("    Duration:    %s\n", formatDurationShort(matched.Duration))
				}
				if hasCut {
					summary += fmt.Sprintf("    Ads:         %s Removed\n", greenCheck)
				}
				fmt.Print(summary)
			}
		}
	}
}

func absDownloadAllData(cfg Config, quiet bool) {
	if cfg.AudiobookshelfURL == "" {
		fmt.Println("ERROR: audiobookshelf_url is not configured.")
		return
	}

	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		fmt.Println("ERROR: podcasts_dir is not configured.")
		return
	}

	token, err := absLogin(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	baseURL := strings.TrimRight(cfg.AudiobookshelfURL, "/")

	// Get all libraries
	var libsResp absLibrariesResp
	if err := absGet(baseURL, token, "/api/libraries", &libsResp); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get libraries: %v\n", err)
		return
	}

	// Find podcast libraries
	var podcastLibs []absLibrary
	for _, lib := range libsResp.Libraries {
		if lib.MediaType == "podcast" {
			podcastLibs = append(podcastLibs, lib)
		}
	}

	if len(podcastLibs) == 0 {
		fmt.Println("No podcast libraries found.")
		return
	}

	fmt.Printf("Found %d podcast libraries\n", len(podcastLibs))

	// For each podcast library, get all items
	for _, lib := range podcastLibs {
		fmt.Printf("\nLibrary: %s\n", lib.Name)

		var itemsResp absItemsResp
		endpoint := fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID)
		if err := absGet(baseURL, token, endpoint, &itemsResp); err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: Failed to get items for library '%s': %v\n", lib.Name, err)
			continue
		}

		fmt.Printf("  Found %d items\n", len(itemsResp.Results))

		// Process each item
		for _, item := range itemsResp.Results {
			item := item
			fmt.Printf("\n  Item: %s\n", item.RelPath)

			// Get full item details
			var itemFull absItem
			if err := absGet(baseURL, token, "/api/items/"+item.ID, &itemFull); err != nil {
				fmt.Fprintf(os.Stderr, "    ERROR: Failed to fetch item details: %v\n", err)
				continue
			}

			// Print item metadata
			if itemFull.Media.Metadata.Title != "" {
				fmt.Printf("    Title: %s\n", itemFull.Media.Metadata.Title)
			}
			if itemFull.Media.Metadata.Author != "" {
				fmt.Printf("    Author: %s\n", itemFull.Media.Metadata.Author)
			}
			if itemFull.Media.Metadata.Description != "" {
				desc := stripHTML(itemFull.Media.Metadata.Description)
				fmt.Printf("    Description: %s\n", desc)
			}

			// Print episodes
			if len(itemFull.Media.Episodes) > 0 {
				fmt.Printf("    Episodes (%d):\n", len(itemFull.Media.Episodes))
				for i, ep := range itemFull.Media.Episodes {
					fmt.Printf("      %d. %s\n", i+1, ep.Title)
					if ep.Description != "" {
						desc := stripHTML(ep.Description)
						fmt.Printf("         Description: %s\n", desc)
					}
					if ep.PubDate != "" {
						fmt.Printf("         Published: %s\n", ep.PubDate)
					}
					if ep.Duration > 0 {
						fmt.Printf("         Duration: %s\n", formatDurationShort(ep.Duration))
					}
					if ep.AudioFile != nil {
						fmt.Printf("         Audio File: %s\n", ep.AudioFile.Metadata.Filename)
					}
				}
			}
		}
	}
}

func checkmark(ok bool) string {
	if ok {
		return greenCheck
	}
	return yellowQ
}

const greenCheck = "\u2713"
const yellowQ = "?"

func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			if c == '&' {
				entityEnd := strings.IndexByte(s[i:], ';')
				if entityEnd >= 0 {
					entity := s[i : i+entityEnd+1]
					switch entity {
					case "&amp;":
						result.WriteByte('&')
					case "&lt;":
						result.WriteByte('<')
					case "&gt;":
						result.WriteByte('>')
					case "&quot;":
						result.WriteByte('"')
					case "&apos;":
						result.WriteByte('\'')
					case "&nbsp;":
						result.WriteByte(' ')
					default:
						result.WriteString(entity)
					}
					i += entityEnd
					continue
				}
			}
			result.WriteByte(c)
		}
	}
	return strings.TrimSpace(result.String())
}
