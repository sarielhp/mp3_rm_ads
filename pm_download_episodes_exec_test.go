package main

import "testing"

func TestExecuteEpisodeDownloadsReturnsTheSelectedCount(t *testing.T) {
	episodes := []FeedEpisode{
		{Title: "Ep 1", PublishedAt: 1000},
		{Title: "Ep 2", PublishedAt: 2000},
		{Title: "Ep 3", PublishedAt: 3000},
	}

	// dryRun keeps the ABS client untouched, so a nil client is safe here.
	got := executeEpisodeDownloads(nil, PodcastItem{}, episodes, []string{"3 new episodes"},
		"Test Podcast", "item-1",
		true /*noWait*/, true /*dryRun*/, false /*oldest*/, false /*forceNewOnly*/, false, /*verbose*/
		nil /*keep*/, true /*quiet*/)

	if got != len(episodes) {
		t.Errorf("expected %d, got %d: callers gate waitForActiveDownloads and post-download "+
			"processing on this count, so a constant 0 silently disables both",
			len(episodes), got)
	}
}

func TestExecuteEpisodeDownloadsReturnsZeroWhenNothingSelected(t *testing.T) {
	got := executeEpisodeDownloads(nil, PodcastItem{}, nil, nil, "Test Podcast", "item-1",
		true, true, false, false, false, nil, true)
	if got != 0 {
		t.Errorf("expected 0 for an empty selection, got %d", got)
	}
}
