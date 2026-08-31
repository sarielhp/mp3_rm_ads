package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FeedCacheEntry struct {
	FeedURL      string        `json:"feed_url"`
	ETag         string        `json:"etag,omitempty"`
	LastModified string        `json:"last_modified,omitempty"`
	LastChecked  time.Time     `json:"last_checked"`
	LatestGUID   string        `json:"latest_guid,omitempty"`
	Episodes     []FeedEpisode `json:"episodes,omitempty"`
}

const FeedCacheDefaultTTL = 48 * time.Hour

func (e *FeedCacheEntry) IsExpired(ttl time.Duration) bool {
	if e == nil {
		return true
	}
	if e.LastChecked.IsZero() {
		return true
	}
	return time.Since(e.LastChecked) > ttl
}

type FeedCacheManager struct {
	mu        sync.RWMutex
	cacheFile string
	entries   map[string]*FeedCacheEntry
	dirty     bool
}

var globalFeedCache = newFeedCacheManager()

func feedCachePath() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(userTmpDir(), "feed_cache.json")
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(cacheHome, "abs")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "feed_cache.json")
}

func newFeedCacheManager() *FeedCacheManager {
	mgr := &FeedCacheManager{
		cacheFile: feedCachePath(),
		entries:   make(map[string]*FeedCacheEntry),
	}
	mgr.load()
	return mgr
}

func (m *FeedCacheManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.cacheFile)
	if err != nil {
		return
	}
	var loaded map[string]*FeedCacheEntry
	if err := json.Unmarshal(data, &loaded); err == nil {
		m.entries = loaded
	}
}

func (m *FeedCacheManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty {
		return nil
	}
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(m.cacheFile), 0755)
	err = os.WriteFile(m.cacheFile, data, 0644)
	if err == nil {
		m.dirty = false
	}
	return err
}

func (m *FeedCacheManager) Get(feedURL string) *FeedCacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if entry, ok := m.entries[feedURL]; ok {
		cp := *entry
		return &cp
	}
	return nil
}

func (m *FeedCacheManager) Put(feedURL string, entry *FeedCacheEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[feedURL] = entry
	m.dirty = true
}

type rssXML struct {
	XMLName xml.Name   `xml:"rss"`
	Channel channelXML `xml:"channel"`
}

type channelXML struct {
	Title string    `xml:"title"`
	Items []itemXML `xml:"item"`
}

type itemXML struct {
	Title       string        `xml:"title"`
	Description string        `xml:"description"`
	PubDate     string        `xml:"pubDate"`
	GUID        guidXML       `xml:"guid"`
	ID          string        `xml:"id"`
	Enclosure   *enclosureXML `xml:"enclosure"`
	Duration    string        `xml:"duration"`
	Season      string        `xml:"season"`
	Episode     string        `xml:"episode"`
}

type guidXML struct {
	Value string `xml:",chardata"`
}

type enclosureXML struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

func parseFeedDate(pubDate string) (int64, string) {
	pubDate = strings.TrimSpace(pubDate)
	if pubDate == "" {
		return 0, ""
	}
	dateFormats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, pubDate); err == nil {
			return t.UnixMilli(), pubDate
		}
	}
	return 0, pubDate
}

func parseRSSXML(data []byte) ([]FeedEpisode, error) {
	var rss rssXML
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}

	var episodes []FeedEpisode
	for _, it := range rss.Channel.Items {
		if it.Enclosure == nil || strings.TrimSpace(it.Enclosure.URL) == "" {
			continue
		}

		guid := strings.TrimSpace(it.GUID.Value)
		if guid == "" {
			guid = strings.TrimSpace(it.ID)
		}
		if guid == "" && it.Enclosure != nil {
			guid = it.Enclosure.URL
		}

		pubMS, pubStr := parseFeedDate(it.PubDate)

		ep := FeedEpisode{
			Title:            strings.TrimSpace(it.Title),
			DescriptionPlain: strings.TrimSpace(it.Description),
			PubDate:          pubStr,
			PublishedAt:      pubMS,
			GUID:             guid,
			Season:           strings.TrimSpace(it.Season),
			Episode:          strings.TrimSpace(it.Episode),
		}
		if it.Enclosure != nil && it.Enclosure.URL != "" {
			encURL := strings.TrimSpace(it.Enclosure.URL)
			if strings.HasPrefix(encURL, "http://") {
				encURL = "https://" + strings.TrimPrefix(encURL, "http://")
			}
			ep.EnclosureURL = encURL
			ep.Enclosure = &FeedEnclosure{
				URL:  encURL,
				Type: strings.TrimSpace(it.Enclosure.Type),
			}
		}
		episodes = append(episodes, ep)
	}
	return episodes, nil
}

func fetchFeedDirect(feedURL string, cachedETag, cachedLastMod string) ([]FeedEpisode, string, string, bool, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, "", "", false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ABSPodcastManager/1.0)")
	if cachedETag != "" {
		req.Header.Set("If-None-Match", cachedETag)
	}
	if cachedLastMod != "" {
		req.Header.Set("If-Modified-Since", cachedLastMod)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, cachedETag, cachedLastMod, true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", false, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}

	newETag := resp.Header.Get("ETag")
	newLastMod := resp.Header.Get("Last-Modified")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", false, err
	}

	episodes, err := parseRSSXML(body)
	if err != nil {
		return nil, "", "", false, err
	}

	return episodes, newETag, newLastMod, false, nil
}
