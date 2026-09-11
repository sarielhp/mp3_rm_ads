package podcast

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"abs/pkg/backend"
)

func TestGeneratePodcastFeedXML(t *testing.T) {
	tmpDir := t.TempDir()
	podDir := filepath.Join(tmpDir, "Hardcore_History")
	_ = os.MkdirAll(podDir, 0755)

	ep1 := filepath.Join(podDir, "Episode 69 - Twilight.mp3")
	_ = os.WriteFile(ep1, []byte("fake mp3 data"), 0644)

	cover := filepath.Join(podDir, "cover.jpg")
	_ = os.WriteFile(cover, []byte("fake image data"), 0644)

	sub := Subscription{
		ID:      "hh",
		Title:   "Hardcore History",
		FeedURL: "https://dancarlin.com/feed.xml",
		Folder:  "Hardcore_History",
	}

	feedEps := []backend.FeedEpisode{
		{
			Title:           "Episode 69 - Twilight",
			GUID:            "guid-ep-69",
			Description:     "A great history episode about the eastern front.",
			PublishedAt:     1600000000000,
			DurationSeconds: 15155, // ~4h 12m 35s
		},
	}

	baseURL := "http://myserver.tailscale.net:8080/podcasts"
	err := WritePodcastFeedXML(podDir, sub, baseURL, feedEps)
	if err != nil {
		t.Fatalf("WritePodcastFeedXML failed: %v", err)
	}

	feedPath := filepath.Join(podDir, "feed.xml")
	data, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("failed to read feed.xml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<rss version=\"2.0\"") {
		t.Errorf("missing rss version tag")
	}
	if !strings.Contains(content, "<title>Hardcore History</title>") {
		t.Errorf("missing title tag")
	}
	if !strings.Contains(content, "http://myserver.tailscale.net:8080/podcasts/Hardcore_History/cover.jpg") {
		t.Errorf("missing or incorrect cover image URL: %s", content)
	}
	if !strings.Contains(content, "guid-ep-69") {
		t.Errorf("missing preserved upstream GUID: %s", content)
	}
	if !strings.Contains(content, "http://myserver.tailscale.net:8080/podcasts/Hardcore_History/Episode%2069%20-%20Twilight.mp3") {
		t.Errorf("missing or incorrect enclosure URL: %s", content)
	}

	// Verify it can be unmarshaled as valid XML
	var parsedDoc rssDocument
	if err := xml.Unmarshal(data, &parsedDoc); err != nil {
		t.Fatalf("feed.xml is not valid XML: %v", err)
	}
	if len(parsedDoc.Channel.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(parsedDoc.Channel.Items))
	}
	item := parsedDoc.Channel.Items[0]
	if item.GUID.Value != "guid-ep-69" {
		t.Errorf("expected GUID guid-ep-69, got %s", item.GUID.Value)
	}
}
