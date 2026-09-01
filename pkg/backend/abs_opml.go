package backend

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline,omitempty"`
}

type opmlRoot struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title       string `xml:"title,omitempty"`
		DateCreated string `xml:"dateCreated,omitempty"`
	} `xml:"head"`
	Body struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

func BuildOPMLXML(feeds []OPMLFeed) ([]byte, error) {
	var itemOutlines []opmlOutline
	for _, f := range feeds {
		title := f.Title
		if title == "" {
			title = "Podcast Feed"
		}
		itemOutlines = append(itemOutlines, opmlOutline{
			Text:   title,
			Title:  title,
			Type:   "rss",
			XMLURL: f.URL,
		})
	}

	groupOutline := opmlOutline{
		Text:     "Audiobookshelf Podcasts",
		Outlines: itemOutlines,
	}

	doc := opmlRoot{
		Version: "2.0",
		Head: struct {
			Title       string `xml:"title,omitempty"`
			DateCreated string `xml:"dateCreated,omitempty"`
		}{
			Title:       "Audiobookshelf Podcast Feeds",
			DateCreated: time.Now().UTC().Format(time.RFC1123Z),
		},
		Body: struct {
			Outlines []opmlOutline `xml:"outline"`
		}{
			Outlines: []opmlOutline{groupOutline},
		},
	}

	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func ParseOPMLXML(data []byte) ([]OPMLFeed, error) {
	var root opmlRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var feeds []OPMLFeed
	var collect func(outlines []opmlOutline)
	collect = func(outlines []opmlOutline) {
		for _, o := range outlines {
			feedURL := o.XMLURL
			if feedURL == "" && o.Type == "rss" {
				feedURL = o.HTMLURL
			}
			if feedURL != "" {
				title := o.Title
				if title == "" {
					title = o.Text
				}
				feeds = append(feeds, OPMLFeed{
					Title: title,
					URL:   feedURL,
				})
			}
			if len(o.Outlines) > 0 {
				collect(o.Outlines)
			}
		}
	}

	collect(root.Body.Outlines)
	return feeds, nil
}

func (c *AudiobookshelfBackend) FetchPodcastFeeds(silent, verbose bool) ([]OPMLFeed, error) {
	podcasts, err := c.Podcasts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve podcasts: %w", err)
	}

	var feeds []OPMLFeed
	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		if title == "" {
			title = item.ID
		}

		slug, err := c.OpenRSSFeed(item.ID, c.Host)
		if err != nil || slug == "" {
			if !silent {
				fmt.Printf("  [SKIP] Could not open RSS feed for: %s\n", title)
			}
			continue
		}

		feedURL := slug
		if !strings.HasPrefix(feedURL, "http://") && !strings.HasPrefix(feedURL, "https://") {
			feedURL = fmt.Sprintf("%s/feed/%s", strings.TrimRight(c.Host, "/"), strings.TrimPrefix(slug, "/"))
		}

		feeds = append(feeds, OPMLFeed{
			Title: title,
			URL:   feedURL,
		})

		if !silent && verbose {
			fmt.Printf("  [OK] %s -> %s\n", title, feedURL)
		}
	}

	return feeds, nil
}

func (c *AudiobookshelfBackend) ExportOPML(opts OPMLExportOptions) ([]byte, error) {
	feeds, err := c.FetchPodcastFeeds(opts.Quiet, opts.Verbose)
	if err != nil {
		return nil, err
	}
	return BuildOPMLXML(feeds)
}
