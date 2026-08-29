package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type rawOPMLOutline struct {
	Type     string           `xml:"type,attr"`
	Text     string           `xml:"text,attr"`
	Title    string           `xml:"title,attr"`
	XMLURL   string           `xml:"xmlUrl,attr"`
	XMLURL2  string           `xml:"xmlURL,attr"`
	HTMLURL  string           `xml:"htmlUrl,attr"`
	Children []rawOPMLOutline `xml:"outline"`
}

type rawOPMLDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    struct {
		Outlines []rawOPMLOutline `xml:"outline"`
	} `xml:"body"`
}

func parseOPMLXML(data []byte) ([]PodcastFeedInfo, error) {
	var doc rawOPMLDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse OPML XML: %w", err)
	}

	var feeds []PodcastFeedInfo
	seen := make(map[string]bool)

	var traverse func(outlines []rawOPMLOutline)
	traverse = func(outlines []rawOPMLOutline) {
		for _, o := range outlines {
			feedURL := strings.TrimSpace(o.XMLURL)
			if feedURL == "" {
				feedURL = strings.TrimSpace(o.XMLURL2)
			}
			if feedURL != "" {
				title := strings.TrimSpace(o.Title)
				if title == "" {
					title = strings.TrimSpace(o.Text)
				}
				if title == "" {
					title = "Untitled Podcast"
				}
				normURL := strings.ToLower(feedURL)
				if !seen[normURL] {
					seen[normURL] = true
					feeds = append(feeds, PodcastFeedInfo{
						Title: title,
						URL:   feedURL,
					})
				}
			}
			if len(o.Children) > 0 {
				traverse(o.Children)
			}
		}
	}

	traverse(doc.Body.Outlines)
	return feeds, nil
}

func isAudiobookshelfHostedFeed(feedURL, absBaseURL string) bool {
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return false
	}
	absBaseURL = strings.TrimRight(strings.TrimSpace(absBaseURL), "/")
	if absBaseURL != "" && strings.HasPrefix(strings.ToLower(feedURL), strings.ToLower(absBaseURL)+"/feed/") {
		return true
	}
	feedU, err1 := url.Parse(feedURL)
	absU, err2 := url.Parse(absBaseURL)
	if err1 == nil && err2 == nil && feedU.Host != "" && absU.Host != "" {
		if strings.EqualFold(feedU.Host, absU.Host) && strings.HasPrefix(feedU.Path, "/feed") {
			return true
		}
	}
	return false
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
	return strings.TrimSpace(title)
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

func importOPML(config Config, opmlFile string, quiet, verbose bool) {
	if opmlFile == "" {
		showOPMLImportUsage()
		printError("Error: missing required <file> argument for 'abs opml import <file>'.")
		os.Exit(1)
	}

	data, err := os.ReadFile(opmlFile)
	if err != nil {
		printError(fmt.Sprintf("Error reading OPML file '%s': %v", opmlFile, err))
		os.Exit(1)
	}

	feeds, err := parseOPMLXML(data)
	if err != nil {
		printError(fmt.Sprintf("Error: %v", err))
		os.Exit(1)
	}

	if len(feeds) == 0 {
		printError(fmt.Sprintf("No podcast RSS feeds found in OPML file '%s'.", opmlFile))
		os.Exit(1)
	}

	if !quiet {
		fmt.Printf("Loaded %d podcast feed(s) from OPML file '%s'.\n", len(feeds), opmlFile)
		fmt.Println("Connecting to Audiobookshelf...")
	}

	client, err := getABSClient(config, quiet)
	if err != nil {
		printError(fmt.Sprintf("Error connecting to Audiobookshelf: %v", err))
		os.Exit(1)
	}

	libs, err := client.PodcastLibraries()
	if err != nil {
		printError(fmt.Sprintf("Error fetching podcast libraries: %v", err))
		os.Exit(1)
	}
	if len(libs) == 0 {
		printError("Error: no podcast library found on Audiobookshelf.")
		os.Exit(1)
	}

	targetLib := libs[0]
	if len(targetLib.Folders) == 0 {
		printError("Error: podcast library has no storage folders configured in Audiobookshelf.")
		os.Exit(1)
	}
	targetFolder := targetLib.Folders[0]

	existingItems, err := client.PodcastItems()
	if err != nil {
		printError(fmt.Sprintf("Error fetching existing podcasts from Audiobookshelf: %v", err))
		os.Exit(1)
	}

	existingFeeds := make(map[string]bool)
	existingTitles := make(map[string]bool)
	for _, it := range existingItems {
		u := strings.TrimSpace(it.Media.Metadata.FeedURL)
		if u != "" {
			existingFeeds[strings.ToLower(u)] = true
		}
		t := strings.TrimSpace(it.Media.Metadata.Title)
		if t != "" {
			existingTitles[strings.ToLower(t)] = true
			existingTitles[strings.ToLower(sanitizePodcastTitle(t))] = true
		}
	}

	if feedsBody, err := client.Request("/api/feeds", "GET", nil); err == nil {
		var fRes struct {
			Feeds []struct {
				EntityID string `json:"entityId"`
				FeedURL  string `json:"feedUrl"`
				Slug     string `json:"slug"`
			} `json:"feeds"`
		}
		if json.Unmarshal(feedsBody, &fRes) == nil {
			for _, f := range fRes.Feeds {
				if f.FeedURL != "" {
					existingFeeds[strings.ToLower(f.FeedURL)] = true
				}
				if f.Slug != "" {
					existingFeeds[strings.ToLower("/feed/"+f.Slug)] = true
				}
				if f.EntityID != "" {
					existingFeeds[strings.ToLower("/feed/"+f.EntityID)] = true
				}
			}
		}
	}

	newlySubscribed := 0
	alreadyExisted := 0
	skippedSelfFeeds := 0

	for idx, f := range feeds {
		normURL := strings.ToLower(strings.TrimSpace(f.URL))
		normTitle := strings.ToLower(strings.TrimSpace(f.Title))
		safeTitle := sanitizePodcastTitle(f.Title)
		normSafeTitle := strings.ToLower(safeTitle)

		if isAudiobookshelfHostedFeed(f.URL, config.AudiobookshelfURL) || existingFeeds[normURL] && strings.Contains(normURL, "/feed/") {
			skippedSelfFeeds++
			if verbose && !quiet {
				fmt.Printf("  [%d/%d] Skipping Audiobookshelf hosted feed (circular dependency prevention): %s (%s)\n",
					idx+1, len(feeds), f.Title, f.URL)
			}
			continue
		}

		if existingFeeds[normURL] || existingTitles[normTitle] || existingTitles[normSafeTitle] {
			alreadyExisted++
			if verbose && !quiet {
				fmt.Printf("  [%d/%d] Already subscribed: %s\n", idx+1, len(feeds), f.Title)
			}
			continue
		}

		containerPath := strings.TrimRight(targetFolder.FullPath, "/") + "/" + safeTitle

		if !quiet {
			fmt.Printf("  [%d/%d] Subscribing: %s...\n", idx+1, len(feeds), f.Title)
		}

		_, err := client.CreatePodcast(targetLib.ID, targetFolder.ID, containerPath, f.Title, f.URL)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "400") {
				alreadyExisted++
				existingTitles[normTitle] = true
				existingTitles[normSafeTitle] = true
				if verbose && !quiet {
					fmt.Printf("      Already existed on server: %s\n", f.Title)
				}
				continue
			}
			printError(fmt.Sprintf("      Failed to subscribe '%s': %v", f.Title, err))
			continue
		}

		existingFeeds[normURL] = true
		existingTitles[normTitle] = true
		existingTitles[normSafeTitle] = true
		newlySubscribed++
		if !quiet {
			fmt.Printf("      Successfully subscribed in Audiobookshelf!\n")
		}
	}

	if !quiet {
		if skippedSelfFeeds > 0 {
			fmt.Printf("\nOPML Import Summary: %d newly subscribed, %d already existed, %d Audiobookshelf self-feed(s) skipped (%d total in OPML).\n",
				newlySubscribed, alreadyExisted, skippedSelfFeeds, len(feeds))
		} else {
			fmt.Printf("\nOPML Import Summary: %d newly subscribed, %d already existed (%d total in OPML).\n",
				newlySubscribed, alreadyExisted, len(feeds))
		}
	}
}
