package podcast

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pod/pkg/backend"
	"pod/pkg/format"
	"pod/pkg/util"
)

const commonWebpageCSS = `
:root {
  --bg: #121212;
  --card-bg: #1e1e1e;
  --text: #e0e0e0;
  --text-muted: #9e9e9e;
  --accent: #80cbc4;
  --accent-hover: #b2dfdb;
  --border: #2c2c2c;
  --btn-bg: #2d3748;
}
@media (prefers-color-scheme: light) {
  :root {
    --bg: #f5f7fa;
    --card-bg: #ffffff;
    --text: #2d3748;
    --text-muted: #718096;
    --accent: #00897b;
    --accent-hover: #004d40;
    --border: #e2e8f0;
    --btn-bg: #edf2f7;
  }
}
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background: var(--bg);
  color: var(--text);
  margin: 0;
  padding: 24px 16px;
  line-height: 1.5;
}
.container { max-width: 900px; margin: 0 auto; }
header { margin-bottom: 24px; }
.header-flex { display: flex; gap: 20px; align-items: center; }
.cover-img { width: 120px; height: 120px; border-radius: 8px; object-fit: cover; background: var(--card-bg); border: 1px solid var(--border); }
.header-info h1 { margin: 0 0 8px 0; font-size: 1.6rem; }
.header-info p { margin: 4px 0; color: var(--text-muted); font-size: 0.95rem; }
.btn {
  display: inline-block;
  background: var(--btn-bg);
  color: var(--accent);
  padding: 6px 14px;
  border-radius: 6px;
  text-decoration: none;
  font-weight: 500;
  font-size: 0.88rem;
  border: 1px solid var(--border);
  margin-right: 8px;
  margin-top: 6px;
}
.btn:hover { color: var(--accent-hover); }
.nav-link { color: var(--accent); text-decoration: none; font-size: 0.95rem; margin-bottom: 12px; display: inline-block; }
.episode-card, .show-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}
.episode-title { font-size: 1.1rem; font-weight: 600; margin: 0 0 6px 0; }
.episode-meta { color: var(--text-muted); font-size: 0.85rem; margin-bottom: 10px; }
.episode-desc { font-size: 0.9rem; color: var(--text-muted); margin: 8px 0; max-height: 120px; overflow-y: auto; }
audio { width: 100%; margin-top: 8px; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; }
.show-card { display: flex; flex-direction: column; align-items: center; text-align: center; }
.show-card .cover-img { width: 100px; height: 100px; margin-bottom: 12px; }
.show-title { font-size: 1.05rem; font-weight: 600; margin: 0 0 4px 0; }
`

func GeneratePodcastWebpageHTML(sub Subscription, podDir string, episodes []LocalEpisodeMeta, baseURL string) ([]byte, error) {
	var b strings.Builder
	title := html.EscapeString(sub.Title)
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", title)
	fmt.Fprintf(&b, "<style>%s</style>\n</head>\n<body>\n<div class=\"container\">\n", commonWebpageCSS)
	b.WriteString("<a href=\"../index.html\" class=\"nav-link\">&larr; All Podcasts</a>\n")

	renderWebpageHeader(&b, sub, podDir, len(episodes))
	b.WriteString("<main>\n")
	for _, ep := range episodes {
		renderWebpageEpisodeCard(&b, podDir, ep, baseURL, sub.Folder)
	}
	b.WriteString("</main>\n</div>\n</body>\n</html>\n")
	return []byte(b.String()), nil
}

func renderWebpageHeader(b *strings.Builder, sub Subscription, podDir string, count int) {
	b.WriteString("<header><div class=\"header-flex\">\n")
	coverSrc := findCoverRelativePath(podDir, sub.ImageURL)
	if coverSrc != "" {
		fmt.Fprintf(b, "<img src=\"%s\" alt=\"Cover\" class=\"cover-img\">\n", html.EscapeString(coverSrc))
	}
	b.WriteString("<div class=\"header-info\">\n")
	fmt.Fprintf(b, "<h1>%s</h1>\n", html.EscapeString(sub.Title))
	fmt.Fprintf(b, "<p>%d downloaded episode(s)</p>\n", count)
	b.WriteString("<a href=\"feed.xml\" class=\"btn\">RSS Feed</a>\n")
	if sub.FeedURL != "" {
		fmt.Fprintf(b, "<a href=\"%s\" target=\"_blank\" rel=\"noopener\" class=\"btn\">Original Feed</a>\n", html.EscapeString(sub.FeedURL))
	}
	b.WriteString("</div></div></header>\n")
}

func renderWebpageEpisodeCard(b *strings.Builder, podDir string, ep LocalEpisodeMeta, baseURL, folder string) {
	b.WriteString("<div class=\"episode-card\">\n")
	fmt.Fprintf(b, "<div class=\"episode-title\">%s</div>\n", html.EscapeString(ep.Title))

	var metaParts []string
	if ep.PubDate != "" {
		metaParts = append(metaParts, ep.PubDate)
	}
	if ep.DurationSec > 0 {
		metaParts = append(metaParts, format.FormatClock(ep.DurationSec))
	}
	if ep.SizeBytes > 0 {
		metaParts = append(metaParts, formatBytes(ep.SizeBytes))
	}
	if len(metaParts) > 0 {
		fmt.Fprintf(b, "<div class=\"episode-meta\">%s</div>\n", html.EscapeString(strings.Join(metaParts, " &bull; ")))
	}

	if ep.Description != "" {
		desc := html.EscapeString(ep.Description)
		fmt.Fprintf(b, "<div class=\"episode-desc\">%s</div>\n", desc)
	}

	audioSrc := buildEpisodeAudioSrc(ep.Filename, baseURL, folder)
	fmt.Fprintf(b, "<audio controls preload=\"none\" src=\"%s\"></audio>\n", html.EscapeString(audioSrc))

	reportPath := filepath.Join(podDir, strings.TrimSuffix(ep.Filename, filepath.Ext(ep.Filename))+".report.html")
	if _, err := os.Stat(reportPath); err == nil {
		reportRel := url.PathEscape(filepath.Base(reportPath))
		fmt.Fprintf(b, "<div style=\"margin-top:6px;\"><a href=\"%s\" class=\"btn\" target=\"_blank\">Ad Removal Report</a></div>\n", reportRel)
	}

	b.WriteString("</div>\n")
}

func buildEpisodeAudioSrc(filename, baseURL, folder string) string {
	escapedFile := url.PathEscape(filename)
	if baseURL == "" {
		return escapedFile
	}
	base := strings.TrimRight(baseURL, "/")
	if folder == "" {
		return fmt.Sprintf("%s/%s", base, escapedFile)
	}
	return fmt.Sprintf("%s/%s/%s", base, url.PathEscape(folder), escapedFile)
}

func findCoverRelativePath(podDir, fallbackURL string) string {
	for _, name := range []string{"cover.jpg", "cover.png", "cover.jpeg"} {
		if fi, err := os.Stat(filepath.Join(podDir, name)); err == nil && !fi.IsDir() {
			return name
		}
	}
	return fallbackURL
}

func GenerateCatalogWebpageHTML(podcastsDir string, subs []Subscription, baseURL string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>Podcast Library</title>\n")
	fmt.Fprintf(&b, "<style>%s</style>\n</head>\n<body>\n<div class=\"container\">\n", commonWebpageCSS)
	b.WriteString("<header><h1>Podcast Library</h1>\n<p style=\"color:var(--text-muted);\">Local Ad-Free Podcasts</p>\n")
	if _, err := os.Stat(filepath.Join(podcastsDir, "antennapod.opml")); err == nil {
		b.WriteString("<a href=\"antennapod.opml\" class=\"btn\">AntennaPod OPML</a>\n")
	}
	b.WriteString("</header>\n<main class=\"grid\">\n")

	sort.Slice(subs, func(i, j int) bool {
		return strings.ToLower(subs[i].Title) < strings.ToLower(subs[j].Title)
	})

	for _, sub := range subs {
		renderCatalogShowCard(&b, podcastsDir, sub)
	}

	b.WriteString("</main>\n</div>\n</body>\n</html>\n")
	return []byte(b.String()), nil
}

func renderCatalogShowCard(b *strings.Builder, podcastsDir string, sub Subscription) {
	podDir := ResolvePodcastDirForSub(sub, podcastsDir)
	folder := sub.Folder
	if folder == "" {
		folder = filepath.Base(podDir)
	}
	escapedFolder := url.PathEscape(folder)

	b.WriteString("<div class=\"show-card\">\n")
	coverSrc := findCoverRelativePath(podDir, sub.ImageURL)
	if coverSrc != "" && !strings.HasPrefix(coverSrc, "http://") && !strings.HasPrefix(coverSrc, "https://") {
		coverSrc = fmt.Sprintf("%s/%s", escapedFolder, url.PathEscape(coverSrc))
	}
	if coverSrc != "" {
		fmt.Fprintf(b, "<img src=\"%s\" alt=\"Cover\" class=\"cover-img\">\n", html.EscapeString(coverSrc))
	}
	fmt.Fprintf(b, "<div class=\"show-title\">%s</div>\n", html.EscapeString(sub.Title))
	eps := util.FindMP3Files(podDir)
	fmt.Fprintf(b, "<p style=\"color:var(--text-muted);font-size:0.85rem;margin:4px 0 10px;\">%d episode(s)</p>\n", len(eps))
	fmt.Fprintf(b, "<div><a href=\"%s/index.html\" class=\"btn\">Episodes</a>\n", escapedFolder)
	fmt.Fprintf(b, "<a href=\"%s/feed.xml\" class=\"btn\">RSS</a></div>\n", escapedFolder)
	b.WriteString("</div>\n")
}

func WritePodcastWebpage(podDir string, sub Subscription, baseURL string, feedEpisodes []backend.FeedEpisode) error {
	if err := os.MkdirAll(podDir, 0755); err != nil {
		return err
	}
	episodes := CollectLocalEpisodes(podDir, feedEpisodes)
	data, err := GeneratePodcastWebpageHTML(sub, podDir, episodes, baseURL)
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(filepath.Join(podDir, "index.html"), data, 0644)
}

func WriteCatalogWebpage(podcastsDir string, subs []Subscription, baseURL string) error {
	if err := os.MkdirAll(podcastsDir, 0755); err != nil {
		return err
	}
	data, err := GenerateCatalogWebpageHTML(podcastsDir, subs, baseURL)
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(filepath.Join(podcastsDir, "index.html"), data, 0644)
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
