package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func showOPMLExportUsage() {
	fmt.Println("Usage: abs opml export <file> [options]")
	fmt.Println()
	fmt.Println("Export all podcast subscriptions from Audiobookshelf into an OPML XML file.")
	fmt.Println()
	fmt.Println("Details:")
	fmt.Println("  - Queries Audiobookshelf for all podcasts in your library.")
	fmt.Println("  - For each podcast, verifies if an open public RSS feed exists; if not,")
	fmt.Println("    triggers Audiobookshelf to create and open the RSS feed automatically.")
	fmt.Println("  - Generates a standard OPML 2.0 XML document with the RSS feed URLs.")
	fmt.Println("  - Writes the resulting OPML document to the specified <file> path.")
	fmt.Println("  - The exported file can be imported into any podcast player or feed reader.")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  <file>           Path to write the exported OPML file (required)")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -q, --quiet      Suppress progress outputs")
	fmt.Println("  -v, --verbose    Show detailed debug output")
	fmt.Println()
}

func showOPMLImportUsage() {
	fmt.Println("Usage: abs opml import <file> [options]")
	fmt.Println()
	fmt.Println("Import podcast subscriptions from an OPML file into Audiobookshelf.")
	fmt.Println()
	fmt.Println("Details:")
	fmt.Println("  - Parses all RSS feeds (<outline type=\"rss\" xmlUrl=\"...\">) from the OPML file.")
	fmt.Println("  - Filters out any Audiobookshelf self-hosted feeds to prevent circular dependencies.")
	fmt.Println("  - Queries Audiobookshelf for existing podcasts to prevent duplicate subscriptions.")
	fmt.Println("  - For each new RSS feed, creates a podcast entry and subscribes Audiobookshelf to it.")
	fmt.Println("  - Supports nested outline categories from Apple Podcasts, Pocket Casts, Overcast, etc.")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  <file>           Path to the OPML file to import (required)")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -q, --quiet      Suppress progress outputs")
	fmt.Println("  -v, --verbose    Show detailed debug output")
	fmt.Println()
}

func showOPMLUsage() {
	fmt.Println("Usage: abs opml <command> [args]")
	fmt.Println()
	fmt.Println("Import or export podcast subscriptions using OPML files.")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  import <file>    Import an OPML file and subscribe Audiobookshelf to new RSS feeds")
	fmt.Println("  export <file>    Generate and export all Audiobookshelf podcast RSS feeds to an OPML file")
	fmt.Println()
	fmt.Println("Run 'abs opml <command> --help' for detailed instructions on a command.")
	fmt.Println()
}

func absMapPodcasts(cfg Config, quiet bool) {
	b, err := getBackend(cfg, quiet)
	if err != nil {
		fmt.Println("ERROR: audiobookshelf is not configured.")
		return
	}
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		fmt.Println("ERROR: podcasts_dir is not configured.")
		return
	}

	podcasts, err := b.Podcasts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return
	}

	for _, item := range podcasts {
		podDir := filepath.Join(podcastsDir, item.RelPath)
		podEntries, err := os.ReadDir(podDir)
		if err != nil {
			continue
		}

		fmt.Printf("\n%s %s\n", displayName(item.RelPath), tuiDimStyle.Render(fmt.Sprintf("(%s)", item.Media.Metadata.Title)))

		epByAudioFile := make(map[string]Episode)
		for _, ep := range item.Media.Episodes {
			if ep.AudioFile != nil && ep.AudioFile.Metadata != nil {
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

func absDownloadAllData(cfg Config, quiet bool) {
	b, err := getBackend(cfg, quiet)
	if err != nil {
		fmt.Println("ERROR: audiobookshelf is not configured.")
		return
	}

	libs, err := b.PodcastLibraries()
	if err != nil || len(libs) == 0 {
		fmt.Println("No podcast libraries found.")
		return
	}

	fmt.Printf("Found %d podcast libraries\n", len(libs))
	for _, lib := range libs {
		fmt.Printf("\nLibrary: %s\n", lib.Name)
		podcasts, err := b.Podcasts()
		if err != nil {
			continue
		}
		fmt.Printf("  Found %d items\n", len(podcasts))
		for _, item := range podcasts {
			fmt.Printf("\n  Item: %s\n", item.RelPath)
			if item.Media.Metadata.Title != "" {
				fmt.Printf("    Title: %s\n", item.Media.Metadata.Title)
			}
			if item.Media.Metadata.Author != "" {
				fmt.Printf("    Author: %s\n", item.Media.Metadata.Author)
			}
			if item.Media.Metadata.Description != "" {
				desc := stripHTML(item.Media.Metadata.Description)
				fmt.Printf("    Description: %s\n", desc)
			}
			if len(item.Media.Episodes) > 0 {
				fmt.Printf("    Episodes (%d):\n", len(item.Media.Episodes))
				for i, ep := range item.Media.Episodes {
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
					if ep.AudioFile != nil && ep.AudioFile.Metadata != nil {
						fmt.Printf("         Audio File: %s\n", ep.AudioFile.Metadata.Filename)
					}
				}
			}
		}
	}
}

func sanitizePodcastTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled Podcast"
	}
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, c := range badChars {
		title = strings.ReplaceAll(title, c, "_")
	}
	title = strings.TrimSpace(title)
	if title == "" || title == ".." || title == "." || strings.Trim(title, ".") == "" || strings.HasPrefix(title, "../") || strings.HasPrefix(title, ".._") {
		return "Untitled Podcast"
	}
	return title
}
