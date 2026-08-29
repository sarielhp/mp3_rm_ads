package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outline opmlOutlineGroup `xml:"outline"`
}

type opmlOutlineGroup struct {
	Text     string        `xml:"text,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Type   string `xml:"type,attr"`
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

type PodcastFeedInfo struct {
	Title string
	URL   string
}

type absItemDetailResp struct {
	ID    string `json:"id"`
	Media struct {
		Metadata struct {
			Title string `json:"title"`
		} `json:"metadata"`
	} `json:"media"`
	RSSFeed *struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"rssFeed"`
}

func (c *ABSClient) OpenRSSFeed(itemID, baseURL string) (string, error) {
	payload := map[string]interface{}{
		"serverAddress": strings.TrimRight(baseURL, "/"),
		"slug":          itemID,
	}
	body, err := c.Request(fmt.Sprintf("/api/feeds/item/%s/open", itemID), "POST", payload)
	if err != nil {
		return "", err
	}
	var res struct {
		Slug string `json:"slug"`
		Feed struct {
			Slug string `json:"slug"`
		} `json:"feed"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if res.Feed.Slug != "" {
		return res.Feed.Slug, nil
	}
	if res.Slug != "" {
		return res.Slug, nil
	}
	return itemID, nil
}

func getABSClient(config Config, quiet bool) (*ABSClient, error) {
	token := config.AudiobookshelfToken
	if token == "" {
		token = getABSTokenFromDB(config.AudiobookshelfDBPath)
	}
	if token == "" {
		token, _ = absLogin(config)
	}
	if token == "" {
		return nil, fmt.Errorf("Audiobookshelf API token not configured and could not be retrieved from DB or login")
	}
	client := NewABSClient(config.AudiobookshelfURL, token)
	client.Quiet = quiet
	return client, nil
}

func buildOPMLXML(feeds []PodcastFeedInfo) ([]byte, error) {
	doc := opmlDoc{
		Version: "2.0",
		Head: opmlHead{
			Title: "Audiobookshelf Podcast Feeds",
		},
		Body: opmlBody{
			Outline: opmlOutlineGroup{
				Text:     "Audiobookshelf Podcasts",
				Outlines: make([]opmlOutline, 0, len(feeds)),
			},
		},
	}

	for _, f := range feeds {
		doc.Body.Outline.Outlines = append(doc.Body.Outline.Outlines, opmlOutline{
			Type:   "rss",
			Text:   f.Title,
			XMLURL: f.URL,
		})
	}

	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.Write(data)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func fetchPodcastFeeds(client *ABSClient, baseURL string, silent, verbose bool) ([]PodcastFeedInfo, error) {
	libs, err := client.PodcastLibraries()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch podcast libraries: %w", err)
	}

	if len(libs) == 0 {
		if !silent {
			fmt.Println("No podcast libraries found on the server.")
		}
		return nil, nil
	}

	var feeds []PodcastFeedInfo
	baseURL = strings.TrimRight(baseURL, "/")

	var openFeedsMap = make(map[string]string)
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
				if f.EntityID != "" {
					slug := f.Slug
					if slug == "" && f.FeedURL != "" {
						slug = strings.TrimPrefix(f.FeedURL, "/feed/")
					}
					if slug != "" {
						openFeedsMap[f.EntityID] = slug
					}
				}
			}
		}
	}

	for _, lib := range libs {
		body, err := client.Request(fmt.Sprintf("/api/libraries/%s/items?limit=1000", lib.ID), "GET", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch library items: %w", err)
		}

		var itemsResp struct {
			Results []struct {
				ID    string `json:"id"`
				Media struct {
					Metadata struct {
						Title string `json:"title"`
					} `json:"metadata"`
				} `json:"media"`
			} `json:"results"`
		}
		_ = json.Unmarshal(body, &itemsResp)

		itemIDs := make([]string, 0, len(itemsResp.Results))
		fallbackTitles := make(map[string]string)
		for _, r := range itemsResp.Results {
			if r.ID != "" {
				itemIDs = append(itemIDs, r.ID)
				fallbackTitles[r.ID] = r.Media.Metadata.Title
			}
		}

		if len(itemIDs) == 0 {
			var rawItems []struct {
				ID    string `json:"id"`
				Media struct {
					Metadata struct {
						Title string `json:"title"`
					} `json:"metadata"`
				} `json:"media"`
			}
			if json.Unmarshal(body, &rawItems) == nil {
				for _, r := range rawItems {
					if r.ID != "" {
						itemIDs = append(itemIDs, r.ID)
						fallbackTitles[r.ID] = r.Media.Metadata.Title
					}
				}
			}
		}

		for _, itemID := range itemIDs {
			feedSlug := openFeedsMap[itemID]
			var title string

			if feedSlug == "" {
				detailBody, err := client.Request(fmt.Sprintf("/api/items/%s?include=rssfeed", itemID), "GET", nil)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to get item detail for %s: %v\n", itemID, err)
					}
					continue
				}

				var detail absItemDetailResp
				if err := json.Unmarshal(detailBody, &detail); err != nil {
					continue
				}

				title = detail.Media.Metadata.Title
				if detail.RSSFeed != nil && detail.RSSFeed.Slug != "" {
					feedSlug = detail.RSSFeed.Slug
				}
			}

			if title == "" {
				title = fallbackTitles[itemID]
			}
			if title == "" {
				title = "Untitled Podcast"
			}

			if feedSlug == "" {
				if !silent {
					fmt.Fprintf(os.Stderr, "Opening RSS feed for podcast: '%s'...\n", title)
				}
				openedSlug, err := client.OpenRSSFeed(itemID, baseURL)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to open RSS feed for %s: %v\n", title, err)
					}
				}
				if openedSlug != "" {
					feedSlug = openedSlug
				} else {
					feedSlug = itemID
				}
			}

			feedURL := fmt.Sprintf("%s/feed/%s", baseURL, feedSlug)
			feeds = append(feeds, PodcastFeedInfo{
				Title: title,
				URL:   feedURL,
			})
		}
	}

	return feeds, nil
}

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

func exportOPML(config Config, opmlFile string, quiet, verbose bool) {
	if opmlFile == "" {
		showOPMLExportUsage()
		printError("Error: missing required <file> argument for 'abs opml export <file>'.")
		os.Exit(1)
	}

	if config.AudiobookshelfURL == "" {
		printError("Error: audiobookshelf_url is not configured.")
		os.Exit(1)
	}

	client, err := getABSClient(config, quiet)
	if err != nil {
		printError(fmt.Sprintf("Error: %v", err))
		os.Exit(1)
	}

	feeds, err := fetchPodcastFeeds(client, config.AudiobookshelfURL, quiet, verbose)
	if err != nil {
		printError(fmt.Sprintf("Error generating OPML: %v", err))
		os.Exit(1)
	}

	if len(feeds) == 0 {
		printError("No podcast feeds found on Audiobookshelf to export.")
		os.Exit(1)
	}

	opmlBytes, err := buildOPMLXML(feeds)
	if err != nil {
		printError(fmt.Sprintf("Error formatting OPML XML: %v", err))
		os.Exit(1)
	}

	if dir := filepath.Dir(opmlFile); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			printError(fmt.Sprintf("Error creating directory: %v", err))
			os.Exit(1)
		}
	}
	if err := os.WriteFile(opmlFile, opmlBytes, 0644); err != nil {
		printError(fmt.Sprintf("Error writing OPML file: %v", err))
		os.Exit(1)
	}
	if !quiet {
		fmt.Printf("Successfully exported %d podcast RSS feed(s) to: %s\n", len(feeds), opmlFile)
	}
}
