package podcast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubscriptionStoreCRUD(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "podcasts.json")

	store, err := NewSubscriptionStore(storeFile)
	if err != nil {
		t.Fatalf("NewSubscriptionStore failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Fatalf("expected 0 items initially, got %d", len(store.List()))
	}

	sub := Subscription{
		Title:   "Hardcore History",
		FeedURL: "https://feed.dancarlin.com/hh.xml",
	}
	if err := store.Add(sub); err != nil {
		t.Fatalf("store.Add failed: %v", err)
	}

	if len(store.List()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store.List()))
	}

	found := store.Get("hardcore history")
	if found == nil {
		t.Fatalf("expected to find subscription by title")
	}
	if found.FeedURL != sub.FeedURL {
		t.Errorf("expected feed URL %s, got %s", sub.FeedURL, found.FeedURL)
	}
	if found.ID == "" {
		t.Errorf("expected generated ID")
	}

	if err := store.Save(); err != nil {
		t.Fatalf("store.Save failed: %v", err)
	}

	// Reload from disk
	loadedStore, err := NewSubscriptionStore(storeFile)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if len(loadedStore.List()) != 1 {
		t.Fatalf("expected 1 item after reload, got %d", len(loadedStore.List()))
	}

	// Remove
	removed, err := loadedStore.Remove(found.ID)
	if err != nil || !removed {
		t.Fatalf("failed to remove: %v", err)
	}
	if len(loadedStore.List()) != 0 {
		t.Fatalf("expected 0 items after remove, got %d", len(loadedStore.List()))
	}
}

func TestSubscriptionStoreImportOPML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "podcasts.json")

	store, err := NewSubscriptionStore(storeFile)
	if err != nil {
		t.Fatalf("NewSubscriptionStore failed: %v", err)
	}

	opmlXML := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
  <head><title>Feeds</title></head>
  <body>
    <outline text="RadioLab" title="RadioLab" type="rss" xmlUrl="https://feeds.example.com/radiolab.xml"/>
    <outline text="The Daily" title="The Daily" type="rss" xmlUrl="https://feeds.example.com/daily.xml"/>
  </body>
</opml>`

	count, err := store.ImportFromOPML([]byte(opmlXML))
	if err != nil {
		t.Fatalf("ImportFromOPML failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 imported feeds, got %d", count)
	}

	data, err := os.ReadFile(storeFile)
	if err != nil || len(data) == 0 {
		t.Fatalf("expected saved file at %s", storeFile)
	}
}
