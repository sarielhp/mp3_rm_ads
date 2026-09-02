package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (c *AudiobookshelfBackend) Scan(opts ScanOptions) (ScanResult, error) {
	podcastsDir := opts.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = c.PodcastsDir
	}
	if podcastsDir == "" {
		return ScanResult{}, fmt.Errorf("podcasts_dir is not configured")
	}

	podcasts, err := c.Podcasts()
	if err != nil {
		return ScanResult{}, fmt.Errorf("failed to fetch podcasts: %w", err)
	}

	res := ScanResult{
		CheckedPodcasts: len(podcasts),
		Podcasts:        podcasts,
	}
	if opts.EpisodesOnly {
		return res, nil
	}

	existing := scanLocalPodcastsDir(podcastsDir)
	if !opts.Quiet {
		fmt.Printf("Connected to Audiobookshelf. Found %d podcasts in database.\n", len(podcasts))
		fmt.Println("Scanning for new podcasts not present locally...")
	}

	for _, item := range podcasts {
		title := item.Media.Metadata.Title
		relBase := filepath.Base(item.RelPath)
		if existing[strings.ToLower(title)] || existing[strings.ToLower(relBase)] {
			continue
		}
		res.NewPodcasts++
		_ = c.initNewPodcastDir(podcastsDir, item, opts.Quiet)
	}

	if !opts.Quiet {
		if res.NewPodcasts == 0 {
			fmt.Println("No new podcasts found. Local library is up to date.")
		} else {
			fmt.Printf("\nScan complete. Added and initialized %d new podcast(s).\n", res.NewPodcasts)
		}
	}
	return res, nil
}

func scanLocalPodcastsDir(podcastsDir string) map[string]bool {
	existing := make(map[string]bool)
	if dirEntries, err := os.ReadDir(podcastsDir); err == nil {
		for _, e := range dirEntries {
			if e.IsDir() {
				existing[strings.ToLower(e.Name())] = true
			}
		}
	}
	return existing
}

func (c *AudiobookshelfBackend) initNewPodcastDir(podcastsDir string, item Podcast, quiet bool) error {
	title := item.Media.Metadata.Title
	relBase := filepath.Base(item.RelPath)
	if !quiet {
		fmt.Printf("\n[+] Found new podcast: '%s' (RelPath: %s)\n", title, item.RelPath)
	}

	safeName := sanitizePodcastName(title)
	if safeName == "" {
		safeName = sanitizePodcastName(relBase)
	}
	if safeName == "" {
		safeName = "podcast_" + item.ID
	}

	podDir := filepath.Join(podcastsDir, safeName)
	if err := os.MkdirAll(podDir, 0755); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "  ERROR: Failed to create directory '%s': %v\n", podDir, err)
		}
		return err
	}

	detailsDir := filepath.Join(podDir, ".cache", "details")
	_ = os.MkdirAll(detailsDir, 0755)
	coverDest := filepath.Join(detailsDir, "cover.jpg")
	if err := c.DownloadCover(item.ID, coverDest); err != nil {
		if !quiet {
			fmt.Printf("  Warning: Failed to download cover image: %v\n", err)
		}
	} else if !quiet {
		fmt.Printf("  ✓ Downloaded cover to %s\n", coverDest)
	}
	return nil
}

func sanitizePodcastName(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, s)
	return strings.TrimSpace(cleaned)
}

func StripHTML(s string) string {
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
