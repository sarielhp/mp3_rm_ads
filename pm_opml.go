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

func (c *ABSClient) OpenRSSFeed(itemID string) (string, error) {
	body, err := c.Request(fmt.Sprintf("/api/feeds/item/%s/open", itemID), "POST", map[string]interface{}{})
	if err != nil {
		return "", err
	}
	var res struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	return res.Slug, nil
}

func getABSClient(config Config, silent bool) (*ABSClient, error) {
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
	client.Silent = silent
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

			title := detail.Media.Metadata.Title
			if title == "" {
				title = fallbackTitles[itemID]
			}
			if title == "" {
				title = "Untitled Podcast"
			}

			feedSlug := ""
			if detail.RSSFeed != nil && detail.RSSFeed.Slug != "" {
				feedSlug = detail.RSSFeed.Slug
			} else {
				if !silent {
					fmt.Fprintf(os.Stderr, "Opening RSS feed for podcast: '%s'...\n", title)
				}
				openedSlug, err := client.OpenRSSFeed(itemID)
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

func exportOPML(config Config, cli CLIOptions) {
	if config.AudiobookshelfURL == "" {
		printError("Error: audiobookshelf_url is not configured.")
		os.Exit(1)
	}

	client, err := getABSClient(config, cli.Silent)
	if err != nil {
		printError(fmt.Sprintf("Error: %v", err))
		os.Exit(1)
	}

	feeds, err := fetchPodcastFeeds(client, config.AudiobookshelfURL, cli.Silent, cli.Verbose)
	if err != nil {
		printError(fmt.Sprintf("Error generating OPML: %v", err))
		os.Exit(1)
	}

	if len(feeds) == 0 {
		return
	}

	opmlBytes, err := buildOPMLXML(feeds)
	if err != nil {
		printError(fmt.Sprintf("Error formatting OPML XML: %v", err))
		os.Exit(1)
	}

	if cli.Output != "" {
		if dir := filepath.Dir(cli.Output); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				printError(fmt.Sprintf("Error creating directory: %v", err))
				os.Exit(1)
			}
		}
		if err := os.WriteFile(cli.Output, opmlBytes, 0644); err != nil {
			printError(fmt.Sprintf("Error writing OPML file: %v", err))
			os.Exit(1)
		}
		if !cli.Silent {
			fmt.Printf("Successfully generated OPML file at: %s\n", cli.Output)
			fmt.Printf("Found and processed %d podcast feeds.\n", len(feeds))
		}
	} else {
		fmt.Print(string(opmlBytes))
	}
}
