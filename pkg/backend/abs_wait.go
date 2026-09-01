package backend

import (
	"fmt"
	"os"
	"time"
)

func (c *AudiobookshelfBackend) WaitForActiveDownloads(podcasts []Podcast, quiet bool, timeout time.Duration) error {
	time.Sleep(2 * time.Second)
	startTime := time.Now()
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	for {
		hasActive := false
		var activeTitles []string
		for _, p := range podcasts {
			dls, err := c.ActiveDownloads(p.ID)
			if err == nil && len(dls) > 0 {
				hasActive = true
				for _, d := range dls {
					if d.EpisodeDisplayTitle != "" {
						activeTitles = append(activeTitles, d.EpisodeDisplayTitle)
					}
				}
			}
		}
		if !hasActive {
			if !quiet {
				fmt.Println("\nAll downloads completed successfully!")
			}
			break
		}
		if !quiet && len(activeTitles) > 0 {
			fmt.Printf("\r    Downloading (%d active): %s\x1b[K", len(activeTitles), activeTitles[0])
			os.Stdout.Sync()
		}
		time.Sleep(3 * time.Second)
		if time.Since(startTime) > timeout {
			if !quiet {
				fmt.Println("\nTimeout waiting for downloads to finish. Downloads continue in background.")
			}
			break
		}
	}
	return nil
}
