package backend

import (
	"fmt"
	"net/url"
	"strings"
)

func (c *AudiobookshelfBackend) ImportOPML(data []byte, opts OPMLImportOptions) (OPMLImportResult, error) {
	feeds, err := ParseOPMLXML(data)
	if err != nil {
		return OPMLImportResult{}, err
	}

	libs, err := c.PodcastLibraries()
	if err != nil {
		return OPMLImportResult{}, fmt.Errorf("error fetching podcast libraries: %w", err)
	}
	if len(libs) == 0 {
		return OPMLImportResult{}, fmt.Errorf("no podcast library found on server")
	}

	targetLib := libs[0]
	if len(targetLib.Folders) == 0 {
		return OPMLImportResult{}, fmt.Errorf("podcast library has no storage folders configured")
	}
	targetFolder := targetLib.Folders[0]

	existingItems, err := c.Podcasts()
	if err != nil {
		return OPMLImportResult{}, fmt.Errorf("error fetching existing podcasts: %w", err)
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

	res := OPMLImportResult{TotalFeeds: len(feeds)}

	for idx, f := range feeds {
		normURL := strings.ToLower(strings.TrimSpace(f.URL))
		normTitle := strings.ToLower(strings.TrimSpace(f.Title))
		safeTitle := sanitizePodcastTitle(f.Title)
		normSafeTitle := strings.ToLower(safeTitle)

		if isAudiobookshelfHostedFeed(f.URL, c.Host) || (existingFeeds[normURL] && strings.Contains(normURL, "/feed/")) {
			res.SkippedSelfFeeds++
			continue
		}

		if existingFeeds[normURL] || existingTitles[normTitle] || existingTitles[normSafeTitle] {
			res.AlreadyExisted++
			continue
		}

		containerPath := strings.TrimRight(targetFolder.FullPath, "/") + "/" + safeTitle
		if !opts.Quiet {
			fmt.Printf("  [%d/%d] Subscribing: %s...\n", idx+1, len(feeds), f.Title)
		}

		_, err := c.CreatePodcast(targetLib.ID, targetFolder.ID, containerPath, f.Title, f.URL)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "400") {
				res.AlreadyExisted++
				existingTitles[normTitle] = true
				existingTitles[normSafeTitle] = true
				continue
			}
			continue
		}

		existingFeeds[normURL] = true
		existingTitles[normTitle] = true
		existingTitles[normSafeTitle] = true
		res.Subscribed++
	}

	return res, nil
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
