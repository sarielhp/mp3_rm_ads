package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

func printError(msg string) {
	str := msg
	reLead := regexp.MustCompile(`^\s*\[-\]\s*`)
	str = reLead.ReplaceAllString(str, "")

	redError := "\x1b[31m\x1b[1mError\x1b[0m"
	redErrorColon := "\x1b[31m\x1b[1mError:\x1b[0m"

	var formatted string
	reErr := regexp.MustCompile(`\bError\b`)
	if reErr.MatchString(str) {
		formatted = reErr.ReplaceAllString(str, redError)
	} else {
		formatted = fmt.Sprintf("%s %s", redErrorColon, str)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, formatted)
	fmt.Fprintln(os.Stderr)
}

func parsePubDate(pubStr string) int64 {
	if pubStr == "" {
		return 0
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, pubStr); err == nil {
			return t.UnixNano() / 1e6
		}
	}
	return 0
}

func matchPodcast(podcasts []PodcastItem, searchQuery string) *PodcastItem {
	searchStr := strings.TrimSpace(searchQuery)

	if idx, err := strconv.Atoi(searchStr); err == nil {
		if idx >= 1 && idx <= len(podcasts) {
			return &podcasts[idx-1]
		}
	}

	for i := range podcasts {
		if podcasts[i].ID == searchStr || podcasts[i].Media.ID == searchStr {
			return &podcasts[i]
		}
	}

	lowerQuery := strings.ToLower(searchStr)
	for i := range podcasts {
		if strings.ToLower(podcasts[i].Media.Metadata.Title) == lowerQuery {
			return &podcasts[i]
		}
	}

	for i := range podcasts {
		if strings.Contains(strings.ToLower(podcasts[i].Media.Metadata.Title), lowerQuery) {
			return &podcasts[i]
		}
	}

	return nil
}

func getPubMS(ep FeedEpisode) int64 {
	if ep.PublishedAt > 0 {
		return ep.PublishedAt
	}
	if ep.PubDate != "" {
		return parsePubDate(ep.PubDate)
	}
	return 0
}

func terminalWidth() int {
	if colStr := os.Getenv("COLUMNS"); colStr != "" {
		if w, err := strconv.Atoi(colStr); err == nil && w > 0 {
			return w
		}
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

func resolveHostPath(path string, podcastsDir string) string {
	if path == "" {
		return ""
	}
	if fileExists(path) {
		return path
	}

	if podcastsDir != "" {
		relPath := path
		if strings.HasPrefix(relPath, "/podcasts/") {
			relPath = strings.TrimPrefix(relPath, "/podcasts/")
		} else if strings.HasPrefix(relPath, "/") {
			relPath = strings.TrimPrefix(relPath, "/")
		}

		mapped := filepath.Join(podcastsDir, relPath)
		if fileExists(mapped) {
			return mapped
		}

		baseDir := filepath.Base(filepath.Dir(path))
		baseFile := filepath.Base(path)
		mappedSub := filepath.Join(podcastsDir, baseDir, baseFile)
		if fileExists(mappedSub) {
			return mappedSub
		}
	}

	return ""
}
