package podcast

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"abs/pkg/backend"
	"abs/pkg/format"
	"abs/pkg/util"
)

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	ITunes  string     `xml:"xmlns:itunes,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string          `xml:"title"`
	Link        string          `xml:"link"`
	Description string          `xml:"description"`
	Language    string          `xml:"language,omitempty"`
	Author      string          `xml:"itunes:author,omitempty"`
	Image       *rssFeedImage   `xml:"image,omitempty"`
	ITunesImage *rssItunesImage `xml:"itunes:image,omitempty"`
	Items       []rssItemXML    `xml:"item"`
}

type rssFeedImage struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type rssItunesImage struct {
	Href string `xml:"href,attr"`
}

type rssItemXML struct {
	Title       string          `xml:"title"`
	Description rssCData        `xml:"description"`
	PubDate     string          `xml:"pubDate"`
	GUID        rssGUID         `xml:"guid"`
	Enclosure   rssEnclosure    `xml:"enclosure"`
	Duration    string          `xml:"itunes:duration,omitempty"`
	Image       *rssItunesImage `xml:"itunes:image,omitempty"`
}

type rssCData struct {
	Value string `xml:",cdata"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type LocalEpisodeMeta struct {
	Path        string
	Filename    string
	Title       string
	GUID        string
	PubDate     string
	PublishedAt int64
	DurationSec float64
	SizeBytes   int64
	Description string
}

func CollectLocalEpisodes(podDir string, feedEpisodes []backend.FeedEpisode) []LocalEpisodeMeta {
	mp3Files := util.FindMP3Files(podDir)
	if len(mp3Files) == 0 {
		return nil
	}

	feedMap := buildFeedEpisodeLookup(feedEpisodes)
	var list []LocalEpisodeMeta
	for _, path := range mp3Files {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			continue
		}
		item := buildEpisodeMeta(path, podDir, fi, feedMap)
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].PublishedAt != list[j].PublishedAt {
			return list[i].PublishedAt > list[j].PublishedAt
		}
		return list[i].Filename < list[j].Filename
	})
	return list
}

func buildFeedEpisodeLookup(eps []backend.FeedEpisode) map[string]backend.FeedEpisode {
	m := make(map[string]backend.FeedEpisode)
	for _, ep := range eps {
		titleKey := strings.ToLower(strings.TrimSpace(ep.Title))
		if titleKey != "" {
			m[titleKey] = ep
			m[strings.ToLower(SanitizeTitle(ep.Title))] = ep
		}
		if ep.GUID != "" {
			m[ep.GUID] = ep
		}
	}
	return m
}

func buildEpisodeMeta(path, podDir string, fi os.FileInfo, feedMap map[string]backend.FeedEpisode) LocalEpisodeMeta {
	relPath := filepath.Base(path)
	if podDir != "" {
		if r, err := filepath.Rel(podDir, path); err == nil && r != "" && !strings.HasPrefix(r, "..") {
			relPath = filepath.ToSlash(r)
		}
	}
	fn := relPath
	title := EpisodeTitleFromPath(path)
	cleanKey := strings.ToLower(SanitizeTitle(title))
	matched, hasMatch := feedMap[cleanKey]
	if !hasMatch {
		matched, hasMatch = feedMap[strings.ToLower(strings.TrimSpace(title))]
	}

	guid := ""
	pubDateStr := ""
	pubMs := int64(0)
	desc := ""
	durSec := float64(0)

	if hasMatch {
		guid = matched.GUID
		pubMs = GetPubMS(matched)
		desc = matched.Description
		durSec = matched.DurationSeconds
	}
	if guid == "" {
		h := sha256.Sum256([]byte(relPath))
		guid = "abs:ep:" + hex.EncodeToString(h[:8])
	}
	if pubMs <= 0 {
		pubMs = fi.ModTime().UnixMilli()
	}
	pubDateStr = time.UnixMilli(pubMs).UTC().Format(time.RFC1123Z)
	if desc == "" {
		desc = title
	}

	return LocalEpisodeMeta{
		Path:        path,
		Filename:    fn,
		Title:       title,
		GUID:        guid,
		PubDate:     pubDateStr,
		PublishedAt: pubMs,
		DurationSec: durSec,
		SizeBytes:   fi.Size(),
		Description: desc,
	}
}

func GeneratePodcastFeedXML(sub Subscription, podDir string, episodes []LocalEpisodeMeta, baseURL string) ([]byte, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	folder := sub.Folder
	if folder == "" {
		folder = filepath.Base(podDir)
	}

	channelURL := fmt.Sprintf("%s/%s/", baseURL, url.PathEscape(folder))
	channel := rssChannel{
		Title:       sub.Title,
		Link:        channelURL,
		Description: fmt.Sprintf("Ad-free podcast feed for %s", sub.Title),
		Language:    "en",
		Author:      "abs",
	}

	coverPath := findLocalCover(podDir)
	coverURL := ""
	if coverPath != "" && baseURL != "" {
		coverURL = fmt.Sprintf("%s/%s/%s", baseURL, url.PathEscape(folder), url.PathEscape(filepath.Base(coverPath)))
	} else if sub.ImageURL != "" {
		coverURL = sub.ImageURL
	}

	if coverURL != "" {
		channel.Image = &rssFeedImage{
			URL:   coverURL,
			Title: sub.Title,
			Link:  channelURL,
		}
		channel.ITunesImage = &rssItunesImage{Href: coverURL}
	}

	for _, ep := range episodes {
		encURL := ""
		if baseURL != "" {
			encURL = fmt.Sprintf("%s/%s/%s", baseURL, url.PathEscape(folder), escapeRelPath(ep.Filename))
		}
		itemXML := rssItemXML{
			Title:       ep.Title,
			Description: rssCData{Value: ep.Description},
			PubDate:     ep.PubDate,
			GUID:        rssGUID{IsPermaLink: false, Value: ep.GUID},
			Enclosure: rssEnclosure{
				URL:    encURL,
				Length: ep.SizeBytes,
				Type:   "audio/mpeg",
			},
		}
		if ep.DurationSec > 0 {
			itemXML.Duration = format.FormatClock(ep.DurationSec)
		}
		if coverURL != "" {
			itemXML.Image = &rssItunesImage{Href: coverURL}
		}
		channel.Items = append(channel.Items, itemXML)
	}

	doc := rssDocument{
		Version: "2.0",
		ITunes:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel: channel,
	}

	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal feed xml: %w", err)
	}
	header := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	return append(header, append(data, '\n')...), nil
}

func findLocalCover(podDir string) string {
	candidates := []string{"cover.jpg", "cover.png", "cover.jpeg", "cover.webp", "folder.jpg", "folder.png"}
	for _, c := range candidates {
		p := filepath.Join(podDir, c)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return p
		}
	}
	detailsPath := filepath.Join(podDir, ".cache", "details", "cover.jpg")
	if fi, err := os.Stat(detailsPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
		target := filepath.Join(podDir, "cover.jpg")
		if err := util.CopyFileErr(detailsPath, target); err == nil {
			return target
		}
		return detailsPath
	}
	cDir := CacheDirForPodcast(podDir)
	for _, c := range candidates {
		p := filepath.Join(cDir, c)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			target := filepath.Join(podDir, filepath.Base(p))
			if err := util.CopyFileErr(p, target); err == nil {
				return target
			}
			return p
		}
	}
	return ""
}

func EnsurePodcastCover(podDir string, sub *Subscription) string {
	if podDir == "" {
		return ""
	}
	existing := findLocalCover(podDir)
	if existing != "" {
		return existing
	}

	imageURL := resolveCoverImageURL(sub)
	if imageURL == "" {
		return ""
	}

	ext := ".jpg"
	low := strings.ToLower(imageURL)
	if strings.Contains(low, ".png") {
		ext = ".png"
	}
	destPath := filepath.Join(podDir, "cover"+ext)
	if err := DownloadCoverImage(imageURL, destPath); err == nil {
		return destPath
	}
	return ""
}

func resolveCoverImageURL(sub *Subscription) string {
	if sub == nil {
		return ""
	}
	if sub.ImageURL != "" {
		return sub.ImageURL
	}
	if sub.FeedURL == "" {
		return ""
	}
	if entry := DefaultFeedCache().Get(sub.FeedURL); entry != nil && entry.ImageURL != "" {
		sub.ImageURL = entry.ImageURL
		return entry.ImageURL
	}
	if doc, err := FetchFeedDoc(sub.FeedURL); err == nil && doc != nil && doc.ImageURL != "" {
		sub.ImageURL = doc.ImageURL
		if entry := DefaultFeedCache().Get(sub.FeedURL); entry != nil {
			entry.ImageURL = doc.ImageURL
			DefaultFeedCache().Put(sub.FeedURL, entry)
		}
		return doc.ImageURL
	}
	return ""
}

func WritePodcastFeedXML(podDir string, sub Subscription, baseURL string, feedEpisodes []backend.FeedEpisode) error {
	if err := os.MkdirAll(podDir, 0755); err != nil {
		return err
	}
	EnsurePodcastCover(podDir, &sub)

	if len(feedEpisodes) == 0 && sub.FeedURL != "" {
		if entry := DefaultFeedCache().Get(sub.FeedURL); entry != nil {
			feedEpisodes = entry.FeedEpisodes()
		}
	}

	episodes := CollectLocalEpisodes(podDir, feedEpisodes)
	data, err := GeneratePodcastFeedXML(sub, podDir, episodes, baseURL)
	if err != nil {
		return err
	}

	feedFile := filepath.Join(podDir, "feed.xml")
	return util.WriteFileAtomic(feedFile, data, 0644)
}

func escapeRelPath(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}
