package tui

import "pod/pkg/podcast"

// testLibrary returns the Library the TUI tests drive. Open hands back the
// process-wide feed cache and download queue for the default paths, so a test
// that redirects the queue file through testLibrary().Queue() is redirecting
// the very queue the model under test will use.
func testLibrary() *podcast.Library {
	return podcast.Open(podcast.Config{}, nil, nil)
}
